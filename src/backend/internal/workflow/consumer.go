package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

const (
	EventsStream = "wf:events"
	groupWorkers = "workers"
	groupEngine  = "engine"
	consumerName = "c1" // per-process; scale-out would use hostname
)

func StepStream(handler string) string { return "wf:steps:" + handler }

// Worker runs all consumers for --role=worker|all: one loop per registered
// handler stream + one engine loop for wf:events.
type Worker struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	rdb    *redis.Client
	reg    *Registry
	engine *Engine
	log    *slog.Logger

	consumers int // per stream; 0 means DefaultConsumers
}

func NewWorker(pool *pgxpool.Pool, rdb *redis.Client, reg *Registry, engine *Engine, log *slog.Logger) *Worker {
	return &Worker{pool: pool, q: store.New(pool), rdb: rdb, reg: reg, engine: engine, log: log}
}

// DefaultConsumers is how many goroutines read each stream. One was the
// throughput cap found by the 2026-11-24 load test (P1).
const DefaultConsumers = 8

// SetConcurrency sets the number of consumers per stream (min 1). Call before Run.
// Safe because OnResult locks the instance row (results of one instance are
// serialized) and step execution is guarded by unique(instance, step, attempt).
func (w *Worker) SetConcurrency(n int) {
	if n < 1 {
		n = 1
	}
	w.consumers = n
}

// Run blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	n := w.consumers
	if n < 1 {
		n = DefaultConsumers
	}
	for _, name := range w.reg.Names() {
		stream := StepStream(name)
		w.ensureGroup(ctx, stream, groupWorkers)
		for i := 0; i < n; i++ {
			go w.consumeLoop(ctx, stream, groupWorkers, fmt.Sprintf("%s-%d", consumerName, i), w.processStepMessage)
		}
	}
	w.ensureGroup(ctx, EventsStream, groupEngine)
	for i := 0; i < n; i++ {
		go w.consumeLoop(ctx, EventsStream, groupEngine, fmt.Sprintf("%s-%d", consumerName, i), w.processResultMessage)
	}
	<-ctx.Done()
}

func (w *Worker) ensureGroup(ctx context.Context, stream, group string) {
	err := w.rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil && !isBusyGroup(err) {
		w.log.Error("create consumer group", "stream", stream, "err", err)
	}
}

func (w *Worker) consumeLoop(ctx context.Context, stream, group, consumer string, process func(context.Context, string) error) {
	for {
		if ctx.Err() != nil {
			return
		}
		res, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: group, Consumer: consumer,
			Streams: []string{stream, ">"}, Count: 10, Block: 2 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || ctx.Err() != nil {
				continue
			}
			w.log.Warn("xreadgroup error; backing off", "stream", stream, "err", err)
			time.Sleep(time.Second) // E17: Redis down -> keep retrying
			continue
		}
		for _, s := range res {
			for _, m := range s.Messages {
				w.handleMessage(ctx, stream, group, m, process)
			}
		}
	}
}

func (w *Worker) handleMessage(ctx context.Context, stream, group string, m redis.XMessage, process func(context.Context, string) error) {
	payload, _ := m.Values["payload"].(string)
	if err := process(ctx, payload); err != nil {
		// Leave un-ACKed: XAUTOCLAIM will redeliver (at-least-once).
		w.log.Error("message processing failed; left pending", "stream", stream, "id", m.ID, "err", err)
		return
	}
	if err := w.rdb.XAck(ctx, stream, group, m.ID).Err(); err != nil {
		w.log.Warn("xack failed (redelivery is safe)", "stream", stream, "id", m.ID, "err", err)
	}
}

// processStepMessage executes one auto step with the idempotency contract:
// unique(instance, step, attempt) in step_executions is the effect guard (E04/E06).
func (w *Worker) processStepMessage(ctx context.Context, payload string) error {
	var ev StartEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		w.log.Error("bad step event; dropping", "payload", payload)
		return nil // poison message: drop, don't loop forever
	}

	// Replay? Re-emit the recorded result and stop (E04: crash after commit,
	// before publish/ack — the engine must still hear about it).
	prev, err := w.q.GetStepExecution(ctx, store.GetStepExecutionParams{
		InstanceID: ev.InstanceID, StepName: ev.Step, Attempt: int16(ev.Attempt),
	})
	if err == nil {
		return w.emitRecordedResult(ctx, ev, prev)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	inst, err := w.q.GetInstance(ctx, ev.InstanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	def, err := w.q.GetDefinitionByID(ctx, inst.DefinitionID)
	if err != nil {
		return err
	}
	steps, err := ParseSteps(def.Steps)
	if err != nil {
		return err
	}
	idx := StepIndex(steps, ev.Step)
	if idx < 0 {
		return fmt.Errorf("unknown step %q", ev.Step)
	}
	stepDef := steps[idx]
	handler, ok := w.reg.Get(stepDef.Handler)
	if !ok {
		return fmt.Errorf("no handler %q registered", stepDef.Handler)
	}

	var instCtx map[string]any
	_ = json.Unmarshal(inst.Context, &instCtx)

	timeout := time.Duration(stepDef.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	hctx, cancel := context.WithTimeout(ctx, timeout)
	out, hErr := handler.Execute(hctx, StepInput{
		InstanceID: ev.InstanceID, TransactionID: inst.TransactionID,
		Step: stepDef, Attempt: ev.Attempt, Compensating: ev.Compensate, Context: instCtx,
	})
	cancel()

	return w.recordAndEmit(ctx, ev, out, hErr)
}

// recordAndEmit persists the step execution and the result event atomically.
func (w *Worker) recordAndEmit(ctx context.Context, ev StartEvent, out StepOutput, hErr error) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := w.q.WithTx(tx)

	result := ResultEvent{InstanceID: ev.InstanceID, Step: ev.Step, Attempt: ev.Attempt, Compensate: ev.Compensate}
	execStatus := store.StepStatusCompleted
	var outJSON, errText pgtype.Text

	if hErr != nil {
		execStatus = store.StepStatusFailed
		result.OK = false
		result.Retryable = IsRetryable(hErr)
		result.Error = hErr.Error()
		errText = pgtype.Text{String: truncate(hErr.Error(), 1000), Valid: true}
	} else {
		result.OK = true
		b, _ := json.Marshal(out)
		result.Output = b
		outJSON = pgtype.Text{String: string(b), Valid: true}
	}

	if _, err := q.InsertStepExecution(ctx, store.InsertStepExecutionParams{
		InstanceID: ev.InstanceID, StepName: ev.Step, Attempt: int16(ev.Attempt),
		Status: execStatus,
		Input:  nil,
		Output: jsonbOrNil(outJSON), Error: errText,
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil // concurrent replay already recorded; its path emits the result
		}
		return err
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if _, err := q.InsertOutbox(ctx, store.InsertOutboxParams{
		Aggregate: "workflow", EventType: EventStepResult, Payload: payload,
		AvailableAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (w *Worker) emitRecordedResult(ctx context.Context, ev StartEvent, prev store.GetStepExecutionRow) error {
	result := ResultEvent{
		InstanceID: ev.InstanceID, Step: ev.Step, Attempt: ev.Attempt, Compensate: ev.Compensate,
		OK: prev.Status == store.StepStatusCompleted,
	}
	if result.OK && len(prev.Output) > 0 {
		result.Output = prev.Output
	} else if !result.OK && prev.Error.Valid {
		result.Error = prev.Error.String
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = w.q.InsertOutbox(ctx, store.InsertOutboxParams{
		Aggregate: "workflow", EventType: EventStepResult, Payload: payload,
		AvailableAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	w.log.Info("step replay detected; re-emitted recorded result", "instance", ev.InstanceID, "step", ev.Step, "attempt", ev.Attempt)
	return err
}

func (w *Worker) processResultMessage(ctx context.Context, payload string) error {
	var ev ResultEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		w.log.Error("bad result event; dropping", "payload", payload)
		return nil
	}
	return w.engine.OnResult(ctx, ev)
}

func isBusyGroup(err error) bool {
	return err != nil && len(err.Error()) >= 9 && err.Error()[:9] == "BUSYGROUP"
}

func jsonbOrNil(t pgtype.Text) []byte {
	if !t.Valid {
		return nil
	}
	return []byte(t.String)
}

// PumpLocal drains due outbox events and processes them in-process, bypassing
// Redis. TEST/DEV HARNESS ONLY — the production path is outbox.Relay + streams.
//
// Start events whose handler is NOT in this worker's registry are SKIPPED
// (left unpublished) so a pump with the right registry can claim them.
// Packages that pump the shared test database must not run concurrently
// (`go test -p 1`) — two differently-wired engines (F6 pause hook) stealing
// each other's events would break both.
// Returns the number of events processed by THIS worker.
func (w *Worker) PumpLocal(ctx context.Context, batch int32) (int, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := w.q.WithTx(tx)
	rows, err := q.ClaimOutboxBatch(ctx, batch)
	if err != nil {
		return 0, err
	}

	type work struct {
		id        int64
		eventType string
		payload   string
	}
	var mine []work
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		if r.EventType == EventStepStart {
			var ev StartEvent
			if json.Unmarshal(r.Payload, &ev) == nil && ev.Handler != "" {
				if _, ok := w.reg.Get(ev.Handler); !ok {
					continue // foreign handler: leave for its own pump
				}
			}
		}
		mine = append(mine, work{r.ID, r.EventType, string(r.Payload)})
		ids = append(ids, r.ID)
	}
	if len(ids) > 0 {
		if err := q.MarkOutboxPublished(ctx, ids); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	for i, r := range mine {
		var perr error
		switch r.eventType {
		case EventStepStart:
			perr = w.processStepMessage(ctx, r.payload)
		case EventStepResult:
			perr = w.processResultMessage(ctx, r.payload)
		}
		if perr != nil {
			// The batch was marked published before processing. Release the
			// events after the failing one so a later pump still sees them
			// instead of stranding their workflows.
			if rest := mine[i+1:]; len(rest) > 0 {
				back := make([]int64, 0, len(rest))
				for _, x := range rest {
					back = append(back, x.id)
				}
				_, _ = w.pool.Exec(context.WithoutCancel(ctx),
					`UPDATE outbox SET published_at = NULL WHERE id = ANY($1::bigint[])`, back)
			}
			return 0, perr
		}
	}
	return len(mine), nil
}

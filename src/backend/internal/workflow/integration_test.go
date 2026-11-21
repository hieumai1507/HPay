package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

// --- test handlers ---------------------------------------------------------

type echoHandler struct {
	calls atomic.Int64
	mu    sync.Mutex
	by    map[uuid.UUID]int // calls per instance: other tests leave unpublished events in the shared database
}

func (h *echoHandler) callsFor(id uuid.UUID) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.by[id]
}

func (h *echoHandler) Name() string { return "echo" }
func (h *echoHandler) Execute(_ context.Context, in StepInput) (StepOutput, error) {
	h.calls.Add(1)
	h.mu.Lock()
	if h.by == nil {
		h.by = map[uuid.UUID]int{}
	}
	h.by[in.InstanceID]++
	h.mu.Unlock()
	return StepOutput{Context: map[string]any{"echoed_" + in.Step.Name: true}}, nil
}

type flakyHandler struct{ fails atomic.Int64 }

func (h *flakyHandler) Name() string { return "flaky" }
func (h *flakyHandler) Execute(_ context.Context, _ StepInput) (StepOutput, error) {
	if h.fails.Add(1) <= 2 {
		return StepOutput{}, RetryableError{Err: fmt.Errorf("transient %d", h.fails.Load())}
	}
	return StepOutput{Context: map[string]any{"flaky_ok": true}}, nil
}

type boomHandler struct{}

func (boomHandler) Name() string { return "boom" }
func (boomHandler) Execute(_ context.Context, _ StepInput) (StepOutput, error) {
	return StepOutput{}, fmt.Errorf("terminal explosion")
}

// --- harness ---------------------------------------------------------------

type harness struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	engine *Engine
	worker *Worker
	reg    *Registry
}

func newHarness(t *testing.T, handlers ...StepHandler) *harness {
	pool := testutil.PoolFromEnv(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	reg := NewRegistry()
	for _, h := range handlers {
		reg.Register(h)
	}
	engine := NewEngine(pool, log)
	worker := NewWorker(pool, nil, reg, engine, log) // rdb nil: tests drive outbox directly
	return &harness{pool: pool, q: store.New(pool), engine: engine, worker: worker, reg: reg}
}

func (h *harness) insertDefinition(t *testing.T, name string, steps string) {
	t.Helper()
	_, err := h.q.InsertWorkflowDefinition(context.Background(), store.InsertWorkflowDefinitionParams{
		Name: name, Version: 1, Steps: []byte(steps), IsActive: true,
	})
	if err != nil {
		t.Fatalf("insert definition: %v", err)
	}
}

// pumpOutbox delegates to Worker.PumpLocal (parallel-safe across packages).
func (h *harness) pumpOutbox(t *testing.T) int {
	t.Helper()
	n, err := h.worker.PumpLocal(context.Background(), 100)
	if err != nil {
		t.Fatalf("pump: %v", err)
	}
	return n
}

func (h *harness) drain(t *testing.T, maxPumps int) {
	t.Helper()
	for i := 0; i < maxPumps; i++ {
		if h.pumpOutbox(t) == 0 {
			return
		}
	}
}

func (h *harness) instanceStatus(t *testing.T, id uuid.UUID) store.WfStatus {
	t.Helper()
	inst, err := h.q.GetInstance(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return inst.Status
}

func uniqueName(prefix string) string { return prefix + "_" + uuid.NewString()[:8] }

// --- tests -----------------------------------------------------------------

func TestWorkflowHappyPath(t *testing.T) {
	echo := &echoHandler{}
	h := newHarness(t, echo)
	name := uniqueName("noop")
	h.insertDefinition(t, name, `[
	  {"name":"S1","type":"auto","handler":"echo","retry":2,"timeout_ms":2000,"on_failure":"fail"},
	  {"name":"S2","type":"auto","handler":"echo","retry":2,"timeout_ms":2000,"on_failure":"fail"}]`)

	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, map[string]any{"seed": 1})
	if err != nil {
		t.Fatal(err)
	}
	h.drain(t, 10)

	if got := h.instanceStatus(t, instID); got != store.WfStatusCompleted {
		t.Fatalf("want completed, got %s", got)
	}
	if n := echo.callsFor(instID); n != 2 {
		t.Fatalf("want 2 handler calls for this instance, got %d", n)
	}
}

func TestWorkflowRetryThenSuccessWithBackoff(t *testing.T) {
	flaky := &flakyHandler{}
	h := newHarness(t, flaky)
	name := uniqueName("flaky")
	h.insertDefinition(t, name, `[
	  {"name":"F1","type":"auto","handler":"flaky","retry":3,"timeout_ms":2000,"on_failure":"fail"}]`)

	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// attempt 1 fails -> retry scheduled with FUTURE available_at: pump sees nothing new
	h.pumpOutbox(t) // start -> exec fail
	h.pumpOutbox(t) // result -> engine schedules retry (future)

	var futureCount int
	err = h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE published_at IS NULL AND available_at > now()`).Scan(&futureCount)
	if err != nil {
		t.Fatal(err)
	}
	if futureCount == 0 {
		t.Fatal("expected a backoff-delayed retry event in outbox")
	}
	// fast-forward: make delayed events available now
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE outbox SET available_at = now() WHERE published_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	h.drain(t, 20)
	// still one more retry needed
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE outbox SET available_at = now() WHERE published_at IS NULL`); err != nil {
		t.Fatal(err)
	}
	h.drain(t, 20)

	if got := h.instanceStatus(t, instID); got != store.WfStatusCompleted {
		t.Fatalf("want completed after retries, got %s", got)
	}
	if flaky.fails.Load() != 3 {
		t.Fatalf("want 3 executions (2 fail + 1 ok), got %d", flaky.fails.Load())
	}
}

func TestWorkflowTerminalFailure(t *testing.T) {
	h := newHarness(t, boomHandler{})
	name := uniqueName("boom")
	h.insertDefinition(t, name, `[
	  {"name":"B1","type":"auto","handler":"boom","retry":1,"timeout_ms":2000,"on_failure":"fail"}]`)

	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	h.drain(t, 10)
	if got := h.instanceStatus(t, instID); got != store.WfStatusFailed {
		t.Fatalf("want failed, got %s", got)
	}
}

// TestEdge_E04_RedeliveryNoDoubleSideEffect: the same start-message is processed
// twice (crash before XACK) — the side effect runs exactly once, the result is still re-emitted.
func TestEdge_E04_RedeliveryNoDoubleSideEffect(t *testing.T) {
	echo := &echoHandler{}
	h := newHarness(t, echo)
	name := uniqueName("redeliver")
	h.insertDefinition(t, name, `[
	  {"name":"S1","type":"auto","handler":"echo","retry":1,"timeout_ms":2000,"on_failure":"fail"}]`)

	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(StartEvent{InstanceID: instID, Step: "S1", Handler: "echo", Attempt: 1})

	if err := h.worker.processStepMessage(context.Background(), string(payload)); err != nil {
		t.Fatal(err)
	}
	if err := h.worker.processStepMessage(context.Background(), string(payload)); err != nil { // redelivery
		t.Fatal(err)
	}
	if echo.calls.Load() != 1 {
		t.Fatalf("side effect ran %d times, want 1", echo.calls.Load())
	}
	// two result events are emitted (original + replay) — the idempotent engine handles the rest
	var results int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE event_type = $1 AND payload->>'instance_id' = $2 AND published_at IS NULL`,
		EventStepResult, instID.String()).Scan(&results); err != nil {
		t.Fatal(err)
	}
	if results != 2 {
		t.Fatalf("want 2 result events (original + replay), got %d", results)
	}
}

// TestEdge_E05_OutboxSurvivesBrokerOutage: Redis "down" (relay not running) —
// the event stays in the outbox after commit; once the relay runs again it can be published.
func TestEdge_E05_OutboxSurvivesBrokerOutage(t *testing.T) {
	h := newHarness(t, &echoHandler{})
	name := uniqueName("survive")
	h.insertDefinition(t, name, `[
	  {"name":"S1","type":"auto","handler":"echo","retry":1,"timeout_ms":2000,"on_failure":"fail"}]`)

	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE published_at IS NULL AND payload->>'instance_id' = $1`,
		instID.String()).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("event must sit in outbox while broker is down; got %d", pending)
	}

	// Redis is back: publish for real through the relay if TEST_REDIS_ADDR is set
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Log("TEST_REDIS_ADDR not set — outbox persistence verified, stream publish skipped")
		return
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	// mini-relay (test version — same logic as outbox.Relay, avoids an import cycle)
	_ = log
	tx, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	q := store.New(tx)
	rows, err := q.ClaimOutboxBatch(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		// parallel-safety: only publish rows of THIS test instance;
		// rows of other test packages are left untouched for their own pump.
		var ev StartEvent
		if json.Unmarshal(row.Payload, &ev) != nil || ev.InstanceID != instID {
			continue
		}
		if err := rdb.XAdd(context.Background(), &redis.XAddArgs{
			Stream: StepStream("echo"),
			Values: map[string]any{"type": row.EventType, "payload": string(row.Payload)},
		}).Err(); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, row.ID)
	}
	if err := q.MarkOutboxPublished(context.Background(), ids); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		t.Fatal("relay after outage published nothing")
	}
	l, err := rdb.XLen(context.Background(), StepStream("echo")).Result()
	if err != nil || l == 0 {
		t.Fatalf("message must be on stream after relay: len=%d err=%v", l, err)
	}
}

// TestEdge_E06_DuplicateResultEventIdempotent: the same result event enters the engine twice —
// the instance advances only one step thanks to the guard (status, current_step).
func TestEdge_E06_DuplicateResultEventIdempotent(t *testing.T) {
	h := newHarness(t, &echoHandler{})
	name := uniqueName("dup")
	h.insertDefinition(t, name, `[
	  {"name":"S1","type":"auto","handler":"echo","retry":1,"timeout_ms":2000,"on_failure":"fail"},
	  {"name":"S2","type":"auto","handler":"echo","retry":1,"timeout_ms":2000,"on_failure":"fail"}]`)

	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ev := ResultEvent{InstanceID: instID, Step: "S1", Attempt: 1, OK: true}
	if err := h.engine.OnResult(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if err := h.engine.OnResult(context.Background(), ev); err != nil { // duplicate
		t.Fatal(err)
	}
	// exactly one step.start event is produced for S2
	var starts int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE event_type = $1 AND payload->>'instance_id' = $2 AND payload->>'step' = 'S2'`,
		EventStepStart, instID.String()).Scan(&starts); err != nil {
		t.Fatal(err)
	}
	if starts != 1 {
		t.Fatalf("duplicate result must not double-advance: S2 starts = %d, want 1", starts)
	}
}

// TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim: 2 sweepers in parallel on the same
// stuck instance — the lease UPDATE guarantees only one claimer and only one re-emit.
func TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim(t *testing.T) {
	h := newHarness(t, &echoHandler{})
	name := uniqueName("stuck")
	h.insertDefinition(t, name, `[
	  {"name":"S1","type":"auto","handler":"echo","retry":1,"timeout_ms":2000,"on_failure":"fail"}]`)

	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// make the instance "stuck": move updated_at back past the threshold, delete the unpublished start event (simulates loss)
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE workflow_instances SET updated_at = now() - interval '10 minutes' WHERE id = $1`, instID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE outbox SET published_at = now() WHERE payload->>'instance_id' = $1`, instID.String()); err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rec := NewRecovery(h.pool, nil, h.reg, h.engine, h.worker, log)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = rec.SweepStuck(context.Background()) }()
	}
	wg.Wait()

	var reEmits int
	if err := h.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM outbox WHERE event_type = $1 AND payload->>'instance_id' = $2 AND published_at IS NULL`,
		EventStepStart, instID.String()).Scan(&reEmits); err != nil {
		t.Fatal(err)
	}
	if reEmits != 1 {
		t.Fatalf("lease must prevent double re-emit: got %d, want 1", reEmits)
	}
	if got := h.instanceStatus(t, instID); got != store.WfStatusRunning {
		t.Fatalf("instance must be back to running, got %s", got)
	}
}

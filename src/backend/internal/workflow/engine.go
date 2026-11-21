package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

var ErrDefinitionNotFound = errors.New("workflow definition not found")

// PauseHook runs inside the SAME DB transaction that parks an instance on a
// human_approval step. The approval module implements it (F6): it creates the
// approval_requests row and flips the transaction to pending_approval, so
// there is never a paused instance without a request to act on.
type PauseHook interface {
	OnPause(ctx context.Context, q *store.Queries, instanceID, transactionID uuid.UUID, step StepDef) error
}

// Engine owns instance lifecycle. All transitions happen inside DB
// transactions with the instance row locked (guarded state machine).
type Engine struct {
	pool  *pgxpool.Pool
	q     *store.Queries
	log   *slog.Logger
	pause PauseHook
}

func NewEngine(pool *pgxpool.Pool, log *slog.Logger) *Engine {
	return &Engine{pool: pool, q: store.New(pool), log: log}
}

// SetPauseHook wires the approval module in at boot (nil = human steps fail).
func (e *Engine) SetPauseHook(h PauseHook) { e.pause = h }

// Start creates an instance for the active definition and enqueues the first
// step — all in ONE transaction with the caller's business rows when tx given.
func (e *Engine) Start(ctx context.Context, tx pgx.Tx, defName string, txnID uuid.UUID, initCtx map[string]any) (uuid.UUID, error) {
	q := e.q
	if tx != nil {
		q = e.q.WithTx(tx)
	}
	def, err := q.GetActiveDefinitionByName(ctx, defName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrDefinitionNotFound
		}
		return uuid.Nil, err
	}
	steps, err := ParseSteps(def.Steps)
	if err != nil {
		return uuid.Nil, err
	}
	ctxJSON, err := json.Marshal(orEmpty(initCtx))
	if err != nil {
		return uuid.Nil, err
	}
	first := steps[0]
	instID, err := q.InsertWorkflowInstance(ctx, store.InsertWorkflowInstanceParams{
		DefinitionID:  def.ID,
		TransactionID: txnID,
		CurrentStep:   pgtype.Text{String: first.Name, Valid: true},
		Context:       ctxJSON,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return instID, e.dispatchStep(ctx, q, instID, first, 1)
}

// OnResult advances the state machine for a step result. Idempotent: stale or
// duplicate events no-op via the (status, current_step, attempt) guard (E06).
func (e *Engine) OnResult(ctx context.Context, ev ResultEvent) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := e.q.WithTx(tx)

	inst, err := q.GetInstanceForUpdate(ctx, ev.InstanceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // unknown instance: drop
		}
		return err
	}
	if ev.Compensate || store.WfStatus(inst.Status) == store.WfStatusCompensating {
		if store.WfStatus(inst.Status) != store.WfStatusCompensating || !inst.CurrentStep.Valid || inst.CurrentStep.String != ev.Step {
			e.log.Debug("stale compensation event dropped", "instance", ev.InstanceID, "step", ev.Step)
			return tx.Commit(ctx)
		}
		if err := e.onCompensateResult(ctx, q, inst, ev); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !inst.CurrentStep.Valid || inst.CurrentStep.String != ev.Step {
		e.log.Debug("stale workflow event dropped", "instance", ev.InstanceID, "step", ev.Step)
		return tx.Commit(ctx) // duplicate/stale — already handled (E06)
	}

	def, err := q.GetDefinitionByID(ctx, inst.DefinitionID)
	if err != nil {
		return err
	}
	steps, err := ParseSteps(def.Steps)
	if err != nil {
		return err
	}
	idx := StepIndex(steps, ev.Step)
	if idx < 0 {
		return fmt.Errorf("step %q not in definition %s", ev.Step, def.Name)
	}
	cur := steps[idx]

	// Status guard (E06): auto steps only resolve while running; a human step
	// resolves from waiting_approval — its result event is produced by the
	// approval module's finalize transaction, never by a worker (F6).
	switch store.WfStatus(inst.Status) {
	case store.WfStatusRunning:
	case store.WfStatusWaitingApproval:
		if cur.Type != StepHumanApproval {
			e.log.Debug("stale workflow event dropped", "instance", ev.InstanceID, "step", ev.Step)
			return tx.Commit(ctx)
		}
	default:
		e.log.Debug("stale workflow event dropped", "instance", ev.InstanceID, "step", ev.Step, "status", inst.Status)
		return tx.Commit(ctx)
	}

	switch {
	case ev.OK:
		newCtx, err := mergeContext(inst.Context, ev.Output)
		if err != nil {
			return err
		}
		if err := e.advance(ctx, q, inst, steps, idx, newCtx); err != nil {
			return err
		}
	case ev.Retryable && ev.Attempt < maxRetry(cur):
		backoff := backoffFor(ev.Attempt)
		if err := e.dispatchStepAfter(ctx, q, inst.ID, cur, ev.Attempt+1, backoff); err != nil {
			return err
		}
		e.log.Info("workflow step retry scheduled", "instance", inst.ID, "step", cur.Name, "attempt", ev.Attempt+1, "backoff", backoff.String())
	default:
		if err := e.failInstance(ctx, q, inst, cur, ev.Error); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// advance moves past steps[idx]: next auto step, pause on human_approval, or complete.
func (e *Engine) advance(ctx context.Context, q *store.Queries, inst store.GetInstanceForUpdateRow, steps []StepDef, idx int, newCtx []byte) error {
	for next := idx + 1; next < len(steps); next++ {
		s := steps[next]
		switch s.Type {
		case StepHumanApproval:
			if err := q.AdvanceInstance(ctx, store.AdvanceInstanceParams{
				ID: inst.ID, CurrentStep: pgtype.Text{String: s.Name, Valid: true},
				Status: store.WfStatusWaitingApproval, Context: newCtx,
			}); err != nil {
				return err
			}
			if e.pause == nil {
				e.log.Error("human_approval step with no pause hook; failing instance", "instance", inst.ID, "step", s.Name)
				return e.failInstance(ctx, q, inst, s, "no approval module wired for human_approval step")
			}
			if err := e.pause.OnPause(ctx, q, inst.ID, inst.TransactionID, s); err != nil {
				// Fail clean instead of leaving a paused instance nobody can act
				// on (e.g. no matching policy — maker-side fail-fast missed it).
				e.log.Error("approval pause hook failed; failing instance", "instance", inst.ID, "step", s.Name, "err", err)
				return e.failInstance(ctx, q, inst, s, "approval setup failed: "+err.Error())
			}
			return nil // resumed later by the approval module (F6)
		default:
			if err := q.AdvanceInstance(ctx, store.AdvanceInstanceParams{
				ID: inst.ID, CurrentStep: pgtype.Text{String: s.Name, Valid: true},
				Status: store.WfStatusRunning, Context: newCtx,
			}); err != nil {
				return err
			}
			return e.dispatchStep(ctx, q, inst.ID, s, 1)
		}
	}
	// no more steps: done
	return q.AdvanceInstance(ctx, store.AdvanceInstanceParams{
		ID: inst.ID, CurrentStep: pgtype.Text{}, Status: store.WfStatusCompleted, Context: newCtx,
	})
}

func (e *Engine) failInstance(ctx context.Context, q *store.Queries, inst store.GetInstanceForUpdateRow, step StepDef, reason string) error {
	if step.OnFailure == FailPolicyCompensate {
		queue, err := e.buildCompensationQueue(ctx, q, inst, step.Name)
		if err != nil {
			return err
		}
		if len(queue) > 0 {
			return e.startCompensation(ctx, q, inst, queue, reason)
		}
		// nothing to reverse: fall through to plain failure
	}
	if err := q.AdvanceInstance(ctx, store.AdvanceInstanceParams{
		ID: inst.ID, CurrentStep: pgtype.Text{String: step.Name, Valid: true},
		Status: store.WfStatusFailed, Context: inst.Context,
	}); err != nil {
		return err
	}
	if inst.TransactionID != uuid.Nil {
		if err := q.FailTransactionGuarded(ctx, store.FailTransactionGuardedParams{
			ID: inst.TransactionID, FailureReason: pgtype.Text{String: truncate(reason, 500), Valid: true},
		}); err != nil {
			return err
		}
	}
	e.log.Warn("workflow instance failed", "instance", inst.ID, "step", step.Name, "policy", step.OnFailure, "reason", reason)
	return nil
}

// buildCompensationQueue lists completed compensatable steps BEFORE failedStep,
// newest first (reverse order of execution).
func (e *Engine) buildCompensationQueue(ctx context.Context, q *store.Queries, inst store.GetInstanceForUpdateRow, failedStep string) ([]string, error) {
	def, err := q.GetDefinitionByID(ctx, inst.DefinitionID)
	if err != nil {
		return nil, err
	}
	steps, err := ParseSteps(def.Steps)
	if err != nil {
		return nil, err
	}
	failedIdx := StepIndex(steps, failedStep)
	execs, err := q.ListStepExecutions(ctx, inst.ID)
	if err != nil {
		return nil, err
	}
	completed := map[string]bool{}
	for _, ex := range execs {
		if ex.Status == store.StepStatusCompleted && ex.Attempt < 101 {
			completed[ex.StepName] = true
		}
	}
	var queue []string
	for i := failedIdx - 1; i >= 0; i-- {
		st := steps[i]
		if completed[st.Name] && isCompensatable(st) {
			queue = append(queue, st.Name)
		}
	}
	return queue, nil
}

func isCompensatable(s StepDef) bool {
	if len(s.Config) == 0 {
		return false
	}
	var cfg struct {
		Compensatable bool `json:"compensatable"`
	}
	return json.Unmarshal(s.Config, &cfg) == nil && cfg.Compensatable
}

// startCompensation stores the queue in context and dispatches the first
// reverse execution (attempt >= 101 avoids the forward unique key space).
func (e *Engine) startCompensation(ctx context.Context, q *store.Queries, inst store.GetInstanceForUpdateRow, queue []string, reason string) error {
	newCtx, err := setContextKey(inst.Context, "_compensate", queue[1:])
	if err != nil {
		return err
	}
	def, _ := q.GetDefinitionByID(ctx, inst.DefinitionID)
	steps, _ := ParseSteps(def.Steps)
	first := steps[StepIndex(steps, queue[0])]

	if err := q.AdvanceInstance(ctx, store.AdvanceInstanceParams{
		ID: inst.ID, CurrentStep: pgtype.Text{String: first.Name, Valid: true},
		Status: store.WfStatusCompensating, Context: newCtx,
	}); err != nil {
		return err
	}
	if inst.TransactionID != uuid.Nil {
		if err := q.FailTransactionGuarded(ctx, store.FailTransactionGuardedParams{
			ID: inst.TransactionID, FailureReason: pgtype.Text{String: truncate(reason, 500), Valid: true},
		}); err != nil {
			return err
		}
	}
	e.log.Warn("compensation started", "instance", inst.ID, "queue", queue, "reason", reason)
	return e.dispatchCompensate(ctx, q, inst.ID, first)
}

func (e *Engine) onCompensateResult(ctx context.Context, q *store.Queries, inst store.GetInstanceForUpdateRow, ev ResultEvent) error {
	if !ev.OK {
		// Reverse posting failed: surface loudly for ops (A3 monitor), stop.
		e.log.Error("COMPENSATION FAILED — manual intervention required",
			"instance", inst.ID, "step", ev.Step, "error", ev.Error)
		return q.AdvanceInstance(ctx, store.AdvanceInstanceParams{
			ID: inst.ID, CurrentStep: pgtype.Text{String: ev.Step, Valid: true},
			Status: store.WfStatusFailed, Context: inst.Context,
		})
	}
	queue := getContextSlice(inst.Context, "_compensate")
	if len(queue) == 0 {
		return q.AdvanceInstance(ctx, store.AdvanceInstanceParams{
			ID: inst.ID, CurrentStep: pgtype.Text{}, Status: store.WfStatusCompensated, Context: inst.Context,
		})
	}
	next := queue[0]
	newCtx, err := setContextKey(inst.Context, "_compensate", queue[1:])
	if err != nil {
		return err
	}
	def, _ := q.GetDefinitionByID(ctx, inst.DefinitionID)
	steps, _ := ParseSteps(def.Steps)
	nextStep := steps[StepIndex(steps, next)]
	if err := q.AdvanceInstance(ctx, store.AdvanceInstanceParams{
		ID: inst.ID, CurrentStep: pgtype.Text{String: next, Valid: true},
		Status: store.WfStatusCompensating, Context: newCtx,
	}); err != nil {
		return err
	}
	return e.dispatchCompensate(ctx, q, inst.ID, nextStep)
}

func (e *Engine) dispatchCompensate(ctx context.Context, q *store.Queries, instID uuid.UUID, step StepDef) error {
	payload, err := json.Marshal(StartEvent{
		InstanceID: instID, Step: step.Name, Handler: step.Handler,
		Attempt: 101, Compensate: true,
	})
	if err != nil {
		return err
	}
	_, err = q.InsertOutbox(ctx, store.InsertOutboxParams{
		Aggregate: "workflow", EventType: EventStepStart, Payload: payload,
		AvailableAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	return err
}

func setContextKey(current []byte, key string, val any) ([]byte, error) {
	base := map[string]any{}
	if len(current) > 0 {
		if err := json.Unmarshal(current, &base); err != nil {
			return nil, err
		}
	}
	base[key] = val
	return json.Marshal(base)
}

func getContextSlice(current []byte, key string) []string {
	base := map[string]any{}
	_ = json.Unmarshal(current, &base)
	raw, _ := base[key].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if sv, ok := v.(string); ok {
			out = append(out, sv)
		}
	}
	return out
}

// ReEmitCurrentStep re-dispatches the current step of a recovered instance (E20).
func (e *Engine) ReEmitCurrentStep(ctx context.Context, instID uuid.UUID) error {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := e.q.WithTx(tx)

	inst, err := q.GetInstanceForUpdate(ctx, instID)
	if err != nil {
		return err
	}
	if store.WfStatus(inst.Status) != store.WfStatusRecovering || !inst.CurrentStep.Valid {
		return tx.Commit(ctx)
	}
	def, err := q.GetDefinitionByID(ctx, inst.DefinitionID)
	if err != nil {
		return err
	}
	steps, err := ParseSteps(def.Steps)
	if err != nil {
		return err
	}
	idx := StepIndex(steps, inst.CurrentStep.String)
	if idx < 0 {
		return fmt.Errorf("recover: step %q missing", inst.CurrentStep.String)
	}
	// Re-dispatch with a fresh attempt number beyond any recorded attempt so
	// the idempotency key (instance, step, attempt) never collides.
	attempt := nextFreeAttempt(ctx, q, instID, inst.CurrentStep.String, maxRetry(steps[idx]))
	if err := e.dispatchStep(ctx, q, instID, steps[idx], attempt); err != nil {
		return err
	}
	if err := q.ResumeRecoveredInstance(ctx, instID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (e *Engine) dispatchStep(ctx context.Context, q *store.Queries, instID uuid.UUID, step StepDef, attempt int) error {
	return e.dispatchStepAfter(ctx, q, instID, step, attempt, 0)
}

func (e *Engine) dispatchStepAfter(ctx context.Context, q *store.Queries, instID uuid.UUID, step StepDef, attempt int, delay time.Duration) error {
	payload, err := json.Marshal(StartEvent{InstanceID: instID, Step: step.Name, Handler: step.Handler, Attempt: attempt})
	if err != nil {
		return err
	}
	_, err = q.InsertOutbox(ctx, store.InsertOutboxParams{
		Aggregate: "workflow", EventType: EventStepStart, Payload: payload,
		AvailableAt: pgtype.Timestamptz{Time: time.Now().Add(delay), Valid: true},
	})
	return err
}

func maxRetry(s StepDef) int {
	if s.Retry <= 0 {
		return 1
	}
	return s.Retry
}

func backoffFor(attempt int) time.Duration {
	d := time.Duration(1<<uint(attempt)) * time.Second // 2,4,8...
	if d > 60*time.Second {
		return 60 * time.Second
	}
	return d
}

func nextFreeAttempt(ctx context.Context, q *store.Queries, instID uuid.UUID, step string, from int) int {
	attempt := from
	for {
		attempt++
		_, err := q.GetStepExecution(ctx, store.GetStepExecutionParams{
			InstanceID: instID, StepName: step, Attempt: int16(attempt),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return attempt
		}
		if err != nil || attempt > from+100 {
			return attempt
		}
	}
}

func mergeContext(current []byte, output json.RawMessage) ([]byte, error) {
	base := map[string]any{}
	if len(current) > 0 {
		if err := json.Unmarshal(current, &base); err != nil {
			return nil, err
		}
	}
	if len(output) > 0 {
		var out StepOutput
		if err := json.Unmarshal(output, &out); err != nil {
			return nil, err
		}
		for k, v := range out.Context {
			base[k] = v
		}
	}
	return json.Marshal(base)
}

func orEmpty(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

---
name: gen-workflow-step
description: Create a new workflow step handler for the HPay workflow engine. Trigger when the user asks to "add a step handler", "add a new step to a workflow", or when a new workflow definition needs a handler that doesn't exist yet.
---

# gen-workflow-step — Step handler + registration + resilience tests

## Engine contract (from docs 00 §4 — do not deviate)

```go
type StepHandler interface {
    Name() string
    Execute(ctx context.Context, in StepInput) (StepOutput, error)
}
```

- Delivery is **at-least-once** (Redis Streams consumer group, XACK after success, XAUTOCLAIM reclaims after `timeout_ms`). Therefore every handler MUST be idempotent.
- Handlers NEVER publish events directly. They return `StepOutput`; the worker wrapper writes `step_executions` and enqueues the outbox event in the SAME DB transaction as the handler's side effects.
- Money-touching handlers receive a `pgx.Tx` via the wrapper — all DB effects go through it (atomicity), locks use `SELECT ... FOR UPDATE` ordered by `wallet_id` ascending.

## Steps

1. Create `internal/workflow/handlers/<name>.go`:
   - Struct with deps injected via constructor.
   - **Idempotency guard first**: check `step_executions` for a completed record of (instance_id, step_name, attempt) — if present, return the recorded output, no side effects.
   - Business logic. Wrap partner/slow calls with the step's `timeout_ms` context.
   - Classify errors: return `workflow.RetryableError{...}` for transient (engine retries with backoff, up to `retry`); plain error for terminal (engine runs `on_failure`).
2. Register in `internal/workflow/registry.go` and add the consumer group binding (stream `wf:steps:<name>`).
3. If a definition change is needed, INSERT a NEW version row in `workflow_definitions` (never mutate the active version) and flip `is_active`.
4. Write `<name>_test.go` covering, at minimum:
   - happy path
   - **replay test**: Execute called twice with same input → side effect happens once (E06)
   - retryable vs terminal error classification
   - timeout: context cancelled mid-execution → no partial side effects (E04)
5. If the step performs debit/credit: also add a concurrent test — two instances hitting the same wallet, assert no deadlock and balance is exact (E02/E03).
6. Run `go test ./internal/workflow/... -race` — must pass with `-race`.

## Naming

Step names UPPER_SNAKE (`FRAUD_CHECK`), handler names kebab or single word matching stream (`fraud`). Keep the mapping table in `registry.go` documented.

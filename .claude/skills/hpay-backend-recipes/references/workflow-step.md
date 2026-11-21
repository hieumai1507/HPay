# Create a workflow step handler

> Trigger: Create a new workflow step handler for the HPay workflow engine. Trigger when the user asks to "add a step handler", "add a new step to a workflow", or when a new workflow definition needs a handler that doesn't exist yet.

# gen-workflow-step — Step handler + registration + resilience tests

## Engine contract (verified against `internal/workflow/types.go`, `consumer.go`, `engine.go`)

Read `docs/domain/workflow-engine.md` first; it lists every registered handler and the current gaps.

```go
type StepHandler interface {
    Name() string
    Execute(ctx context.Context, in StepInput) (StepOutput, error)
}
```

- Delivery is **at-least-once** (outbox -> Redis Streams consumer group). Every handler MUST be idempotent on its own: **today the engine does NOT make money steps replay-safe.** The handler commits its own DB transaction first, and only then the worker records `step_executions` and the outbox event in a second transaction (`consumer.go` `recordAndEmit`). A crash or redelivery between the two re-runs the handler (audit BE-C1). Until that is fixed, a step with money effects must carry its own uniqueness guard (e.g. a unique key on what it posts), not rely on the worker.
- Handlers receive a `StepInput` (not a `pgx.Tx`). They open and commit their own transaction using the pool injected through the constructor. They never publish events directly; they return a `StepOutput` and the worker emits the result.
- The worker already skips a step that has a recorded `step_executions` row for the same (instance, step, attempt) and re-emits the recorded result. A handler does not need to re-check that table, but must still be safe when the row was never written (the crash window above).
- Compensation steps always run with attempt 101 (`engine.go`), and a failed compensation ends the instance as `failed` with only a log line (no retry, no `compensation_failed` status; audit BE-H4). Do not rely on compensation retrying.
- Money-touching handlers lock wallets in ascending id order in one statement (use `ledger.Post`, which does this).

## Steps

1. Create `internal/workflow/handlers/<name>.go`:
   - Struct with dependencies injected via constructor.
   - Business logic. Wrap slow or external calls with a context deadline derived from the step's `timeout_ms`.
   - Classify errors: return `workflow.RetryableError{Err: ...}` for transient failures (engine retries with backoff up to `retry`); a plain error is terminal (engine runs `on_failure`).
2. Register the handler where the others are registered: `cmd/server/main.go` (there is no `registry.go` registration file to edit and no manual consumer-group binding; streams are created automatically from the registry names, see the `registry.Names()` log line).
3. If a definition change is needed, INSERT a NEW version row in `workflow_definitions` through a migration (never mutate the active version) and flip `is_active`. Note that nothing enforces a single active definition per name.
4. Write `<name>_test.go` (see `internal/workflow/integration_test.go` and `test/integration/helpers_test.go` for the harness) covering at minimum:
   - happy path
   - **replay test**: run the step twice with the same input and assert the side effect happened once; for a money step this test will FAIL today unless the step has its own guard, which is exactly what it is for (E06, BE-C1)
   - retryable vs terminal error classification
   - timeout: context cancelled mid-execution leaves no partial side effect (E04)
5. If the step performs debit/credit: also add a concurrent test (two instances on the same wallet, no deadlock, exact balance; E02/E03).
6. Run the tests with `-race` through `.\scripts\test-backend.ps1 -GoArgs "./internal/workflow/... -race -count=1"`.
7. Update `docs/domain/workflow-engine.md` (handler table) in the same change.
## Naming

Step names UPPER_SNAKE (`FRAUD_CHECK`), handler names kebab or single word matching stream (`fraud`). Keep the mapping table in `registry.go` documented.

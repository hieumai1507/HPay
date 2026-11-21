---
title: Add a workflow step
type: guide
status: verified-against-code
last_verified: 2026-11-21
---

# Add a workflow step

Sources: `internal/workflow/{types,registry,engine,consumer,recovery}.go`, `internal/workflow/handlers/handlers.go`, `internal/kyc/handlers.go`, `internal/outbox/relay.go`, `cmd/server/main.go`, migration `000003`, recipe `references/workflow-step.md`.

## How the engine works (what you must respect)

- A workflow definition is a row in `workflow_definitions` (`name`, `version`, `steps` JSONB, `is_active`). `Engine.Start` loads the ACTIVE definition by name, inserts a `workflow_instances` row, and writes a `workflow.step.start` event to the `outbox` for the first step, all in the caller's transaction.
- The outbox relay (200 ms poll) is the only writer to Redis Streams. A start event goes to stream `wf:steps:<handler>`; results go to `wf:events`.
- The worker reads each handler stream with consumer group `workers`, runs `handler.Execute`, then in ONE transaction records a `step_executions` row and writes the result event to the outbox (`recordAndEmit`). The unique index `(instance_id, step_name, attempt)` is the idempotency guard: a redelivered message finds the existing row and re-emits the recorded result instead of executing again.
- `Engine.OnResult` guards state by (status, current_step, attempt) and advances: next auto step, pause on a `human_approval` step, or complete. Failure follows the step's `on_failure`: `fail`, `compensate` (runs completed compensatable steps in reverse with `Compensating=true`), or `manual`.

## Step definition fields (`StepDef`)

`name` (UPPER_SNAKE, unique in the definition), `type` (`auto` or `human_approval`), `handler` (required for `auto`), `retry` (max attempts for retryable errors), `timeout_ms` (handler context timeout; 10 s if 0), `on_failure` (`fail` | `compensate` | `manual`), `config` (JSON; `{"compensatable":true}` marks a step reversible). The first step MUST be `auto` (`ParseSteps` rejects otherwise).

## Steps

1. Create the handler in `internal/workflow/handlers/<name>.go` (or in your domain package, like `kyc` and `fraud` do). Implement `workflow.StepHandler`:
   ```go
   type StepHandler interface {
       Name() string
       Execute(ctx context.Context, in StepInput) (StepOutput, error)
   }
   ```
   `Name()` is the handler key (single lower-case word or snake case, e.g. `validator`, `wallet`, `kyc_scorer`), NOT the step name. `StepInput` gives `InstanceID`, `TransactionID`, `Step`, `Attempt`, `Compensating`, `Context`. Return `StepOutput{Context: map[string]any{...}}`; it is merged into the instance context.
2. Behavior rules:
   - Handle `in.Compensating`: return an empty output if there is nothing to reverse, otherwise reverse exactly what the forward execution did, derived from immutable rows (see `Wallet.Execute`).
   - Idempotent side effects: the engine guarantees at-least-once delivery. The worker's `step_executions` check protects against replays of a recorded attempt, but it does NOT protect a crash between your side-effect commit and the `recordAndEmit` commit: your side effect must itself be safe to repeat. This is exactly audit finding BE-C1 for `MOVE_FUNDS` (open): put a natural uniqueness guard in the DB for any money effect.
   - Error classification: wrap transient errors in `workflow.RetryableError{Err: err}` (retried with backoff up to `retry`); return a plain error for terminal failure (the `on_failure` policy runs). A step that returns `ledger.ErrInsufficientBalance` is terminal.
   - Money: do all DB effects through one `pgx.Tx` you begin and commit inside `Execute`; the ledger locks wallets in id order inside `ledger.Post`. Never publish to Redis yourself.
   - Poison input (bad context) should be a terminal error, not a retry (see `requestIDFromContext` in `kyc`).
3. Register the handler at boot. `handlers.RegisterAll(reg, pool)` registers `validator`, `wallet`, `confirmer`; other packages register their own (`kyc.RegisterHandlers`, `fraud.RegisterHandler`, `handlers.RegisterActionPipeline`, `handlers.RegisterRoundUp`) from `cmd/server/main.go` inside the `RunsWorkers()` block. Add your registration there. `Registry.Register` panics on a duplicate handler name. The consumer group and stream (`wf:steps:<handler>`) are created automatically from `registry.Names()` when the worker starts; there is no separate binding file.
4. Add the step to a definition. Definitions are data: add a NEW migration that inserts a new version row (`name`, `version+1`, `steps`, `is_active=true`) and sets the previous version `is_active=false`. Never edit the active row in place. `GetActiveDefinitionByName` selects `WHERE name=$1 AND is_active`, so only one active version per name may exist. Existing definitions: `p2p_transfer` (v3 active; v2 added ROUND_UP in `000009`, v3 wired the action pipeline in `000010`), `org_disbursement` (v2 active, `000010`), `kyc_verification` (v1). Migrations `000009` and `000010` are working examples: they set the old version `is_active = false` and insert the next version.
5. Tests (all with `-race`, through `scripts/test-backend.ps1`):
   - Happy path.
   - Replay: deliver the same step event twice; assert one effect (`TestEdge_E04`, `TestEdge_E06` in `internal/workflow/integration_test.go` are the model).
   - Retryable vs terminal classification.
   - Compensation if the step is compensatable (`TestEdge_E07_CompensationReversesMovedFunds`).
   - If it moves money: concurrent instances on one wallet, assert no deadlock and exact balance (E02/E03 in `test/integration/ledger_test.go`).
   - Use `env.drain(t)` (pumps the outbox with `Worker.PumpLocal`) in integration tests instead of sleeping. `PumpLocal` skips start events whose handler is not in its registry, so a pump never steals another package's events.
   - Packages that pump the outbox must have `TestMain` calling `testutil.RunSerialized(m)`.
6. Update docs: `docs/domain/workflow-engine.md` and the definition table.

## Recipe check (workflow-step.md) and corrections

| Recipe claim | Finding |
|--------------|---------|
| Idempotency guard first: handler checks `step_executions` for a completed (instance, step, attempt) | The WORKER (`processStepMessage`) does this check before calling the handler. Handlers do not need to query `step_executions` |
| "The worker wrapper writes `step_executions` and enqueues the outbox event in the SAME DB transaction as the handler's side effects" | Wrong. The handler commits its own transaction first (`Wallet.Execute`), then `recordAndEmit` runs in a separate transaction. This is the BE-C1 gap |
| Handlers "receive a `pgx.Tx` via the wrapper" | Wrong. Handlers receive only `StepInput` and open their own transaction from the pool |
| Consumer group binding in `registry.go`, stream `wf:steps:<name>` | Streams are `wf:steps:<handler name>` (handler, not step name), created automatically in `Worker.Run`; `registry.go` only holds the map |
| `workflow.RetryableError{...}` | Correct |
| Handler file `internal/workflow/handlers/<name>.go` and registration in `registry.go` | Registration is a call in `cmd/server/main.go` (via `RegisterAll` or a package `Register*` function), not in `registry.go` |
| "Step names UPPER_SNAKE, handler names kebab or single word" | Handler names in code are lower-case single words or snake_case (`kyc_scorer`); no kebab |
| Compensation | The recipe does not mention it. Compensation attempts use the fixed attempt number 101 (audit: cannot be retried), and a failed compensation only logs "COMPENSATION FAILED" and sets the instance `failed` (see [debug-a-stuck-transaction.md](debug-a-stuck-transaction.md)) |

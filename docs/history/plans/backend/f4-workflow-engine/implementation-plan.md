[← F3](../f3-ledger-wallets/implementation-plan.md) · f4-workflow-engine · [solution-architecture →](solution-architecture.md)

# F4 Workflow Engine — Implementation Plan

## 1. Scope & non-goals

| Ships | Not in F4 |
| --- | --- |
| Engine core: Start / OnResult / advance driven by the JSONB definition; step type `auto` runs a handler through a Redis Streams consumer group (XACK, at-least-once), `human_approval` suspends the instance (resume in F6); retry with backoff (outbox `available_at`); `on_failure`: fail \| compensate (framework — the reversing ledger entries are executed in F5 when the wallet handler exists) \| manual; outbox relay (claim SKIP LOCKED); recovery: XAUTOCLAIM dead messages + sweep stuck instances with a lease UPDATE…RETURNING (learned from the reference wallet, E20); worker wiring into `--role=worker/all`; test handlers (echo/flaky/boom) + definition test | 4 business handlers + P2P + fee (F5); resume after approval (F6); admin API to view instances (F6/A3); notification |

## 2. Task breakdown

| # | Task | Files | Size |
| --- | --- | --- | --- |
| T1 | Migration 000011: outbox + `available_at` (backoff) + index; fix the unpublished partial index | db/migrations/000011_* | S |
| T2 | Queries: workflow.sql (definition, instance FOR UPDATE, step_executions, stuck lease) + outbox.sql (claim SKIP LOCKED, mark published) + FailTransactionGuarded | internal/store/queries/* | M |
| T3 | `internal/workflow`: types (StepDef, StepHandler, RetryableError), registry, engine (Start/OnResult/advance/pause/fail/compensate-framework) | internal/workflow/*.go | L |
| T4 | Streams consumer: step worker wrapper (idempotency by unique(instance,step,attempt) → 23505 = replay, skip the side-effect but still re-emit the result event) + engine event consumer | internal/workflow/consumer.go | L |
| T5 | `internal/outbox` relay + recovery (XAUTOCLAIM + DB sweep lease) + wiring in the main worker | internal/outbox/relay.go, internal/workflow/recovery.go, cmd/server | M |
| T6 | Tests E04/E05/E06/E20 + unit definition parse/advance + CI REQUIRED_EDGE_IDS | *_test.go, ci.yml | L |

## 3. Schema changes

000011: `ALTER TABLE outbox ADD COLUMN available_at TIMESTAMPTZ NOT NULL DEFAULT now()` + replace the unpublished index with `(available_at, id) WHERE published_at IS NULL`. Retry backoff = write an outbox row with a future available_at (2^attempt seconds, capped at 60s) — no external scheduler needed.

## 4. API changes

No new endpoints (the engine is internal infrastructure; the P2P API arrives in F5). `--role=worker` now really runs: relay + consumers + recovery.

## 5. Test plan

| ID | Test | Approach |
| --- | --- | --- |
| E04 | `TestEdge_E04_RedeliveryNoDoubleSideEffect` | Process the same message twice (simulating a crash before XACK): the handler side-effect count is 1, the result event is still re-emitted |
| E05 | `TestEdge_E05_OutboxSurvivesBrokerOutage` | Start a workflow while the relay is not running (Redis "down") → the event stays in the outbox; run the relay for one tick → the message appears on the stream, marked published |
| E06 | `TestEdge_E06_DuplicateResultEventIdempotent` | Feed the same result event into the engine twice → the instance advances exactly one step (guard current_step + status) |
| E20 | `TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim` | A stuck instance; 2 sweepers run in parallel → only 1 can claim (UPDATE…RETURNING lease), only 1 event re-emitted |
| — | End-to-end noop flow: Start → echo × 2 steps → completed; flaky handler: fails retryable 2 times then ok (attempt increases, backoff written to available_at); boom handler: on_failure=fail → instance failed + transaction failed guarded | integration |

> E17 (total Redis outage) is half-proven by E05 (no event is lost); the other half (consumers recover on their own when Redis comes back) is behavior of the XREADGROUP loop with retry — recorded through code review, not automatically testable in the current CI.

## 6. Rollout & risk

- The engine is generic — it does not touch the F3 sync topup flow. P2P is only enabled when F5 adds the definition + handlers.

- Compensation in F4 is a framework (the instance moves compensating→compensated when no step needs reversing); executing the money reversal comes with the F5 wallet handler — stated explicitly so nobody assumes it is done.

- Worker and API run in the same process in dev (--role=all); splitting processes is a deployment decision, not code.

## 7. Execution checklist

- [x] gen-feature-plan · [ ] T1→T6 · [ ] fintech-review · [ ] commit `feat: F4 workflow engine - streams consumers, outbox relay, retry/backoff, recovery lease`

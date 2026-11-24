# G19: consumer idempotency for non-money steps

## Goal
A start message can be delivered twice for the same (instance, step, attempt): the handler's side effect commits before `recordAndEmit` writes the step record, so a crash, timeout or XAUTOCLAIM in between reruns the handler. Money steps are covered by the ledger posting guard (ADR-0010). This change reviews every other handler and makes each one safe to run twice.

## Acceptance
1. Every registered handler is classified in `domain/outbox-and-events.md` (pure read, naturally idempotent write, duplicating write, external call).
2. Running any registered handler twice with the same `StepInput` adds no side effect and does not fail (table test `handler_redelivery_test.go`).
3. A new handler without a scenario in that table fails the build (`EveryRegisteredHandlerHasAScenario`).
4. Outbox relay duplicates (XADD then crash before `MarkOutboxPublished`) are tolerated by every consumer; stated in the doc.

## Assumptions (no one to ask)
- No migration. `fraud_flags` gets an advisory-lock check-then-insert instead of a unique index (index recorded as follow-up).
- External calls keep the existing key `ref_code:action_code`, which is stable across attempts, so it also covers recovery re-emits with a new attempt number.
- Files not touched: `internal/offerguard`, `internal/transfer`, `workflow/consumer.go`, `outbox/relay.go`.

## Findings
BUG-0019 (fraud flag duplicated), BUG-0020 (CONFIRM fails on redelivery). All other handlers were already safe.
---
title: Outbox and Events
type: domain
status: verified-against-code
last_verified: 2026-11-21
code:
  - src/backend/internal/outbox/relay.go
  - src/backend/internal/workflow/consumer.go
  - src/backend/internal/workflow/recovery.go
  - src/backend/internal/store/queries/outbox.sql
  - src/backend/cmd/server/main.go
  - src/backend/db/migrations/000003_services_transaction_design.up.sql
related:
  - domain/workflow-engine.md
  - domain/service-catalog.md
audit:
  - BE-C1
  - BE-H7
---

# Outbox and Events

## 1. Purpose
Guarantees that an event is published if and only if the business transaction that produced it committed (transactional outbox), and moves those events to Redis Streams where workflow workers and the engine consume them. It also documents every background goroutine the server starts and how they behave when several replicas run. It does NOT define what the events mean for the workflow state machine (see `workflow-engine.md`). Only workflow events use the outbox today.

## 2. Key concepts
- **Outbox row**: `outbox(id identity, aggregate, event_type, payload jsonb, available_at, published_at, created_at)`.
- **Relay** (`outbox.Relay`): the only production writer to Redis Streams (`relay.go:1-3`).
- **Event types** (`workflow/types.go:125-128`): `workflow.step.start`, `workflow.step.result`. Aggregates used: `workflow` (engine, worker) and `approval` (approval finalize).
- **Streams**: `wf:steps:<handler>` (consumer group `workers`), `wf:events` (group `engine`) (`consumer.go:20-27`).

## 3. Business rules

| ID | Rule | Enforced by (code or DB) | Test |
|----|------|--------------------------|------|
| BR-OB-01 | Business code writes outbox rows in its own DB transaction; nothing publishes before commit. | `workflow/engine.go:dispatchStepAfter` (417-427) called with caller `tx` from `Engine.Start`; `approval/service.go:566`; `consumer.go:206` | `TestEdge_E05_OutboxSurvivesBrokerOutage` |
| BR-OB-02 | Only rows with `published_at IS NULL AND available_at <= now()` are claimed, in `id` order, `FOR UPDATE SKIP LOCKED`, so concurrent relays do not publish the same row. | `queries/outbox.sql:ClaimOutboxBatch` (6-11) | none (no multi-relay test) |
| BR-OB-03 | Relay publishes each claimed row with XADD and marks only the successfully published ones in the same tx; on the first XADD error it stops, marks the already-added rows and commits (or rolls back if none). | `relay.go:RelayOnce` (47-84) | `TestEdge_E05_...` |
| BR-OB-04 | `step.start` events go to `wf:steps:<payload.handler>`; every other event type (and a start event without handler) goes to `wf:events`. | `relay.go:destinationStream` (88-98) | none |
| BR-OB-05 | Delay/backoff is expressed only by `available_at`; the relay does no sleeping. | `engine.go:dispatchStepAfter` (424), `outbox.sql:9` | `TestWorkflowRetryThenSuccessWithBackoff` |
| BR-OB-06 | A stream message is ACKed only after successful processing; failed processing leaves it pending for XAUTOCLAIM (>= 60s idle). Malformed payloads are ACKed (dropped). | `consumer.go:handleMessage` (88-98), `processStepMessage` (103-107), `processResultMessage` (238-242), `recovery.go:56-78` | none |
| BR-OB-07 | A start message is executed at most once per `(instance, step, attempt)`; replays re-emit the stored result. | `consumer.go:processStepMessage` (111-116), unique index `idx_step_exec` | `TestEdge_E04_RedeliveryNoDoubleSideEffect` |
| BR-OB-08 | Result events are idempotent at the engine: stale or duplicate events no-op by status/current_step. | `engine.go:OnResult` (97-139) | `TestEdge_E06_DuplicateResultEventIdempotent` |
| BR-OB-09 | Consumer groups are created at id `0` with MKSTREAM on worker start (BUSYGROUP ignored). | `consumer.go:ensureGroup` (56-61) | none |

## 4. Lifecycle / state machine
An outbox row has no status column; its state is derived.

```mermaid
stateDiagram-v2
    [*] --> Pending: INSERT (published_at NULL)
    Pending --> Pending: available_at in future (backoff)
    Pending --> Published: relay XADD ok, MarkOutboxPublished
    Published --> [*]
```
| Transition | Owner |
|-----------|-------|
| insert | `store/queries/outbox.sql:InsertOutbox` via engine, worker, approval |
| Pending -> Published | `relay.go:RelayOnce` or `consumer.go:PumpLocal` (`MarkOutboxPublished`) |
Rows are never deleted or archived by code (no purge job found), so the table grows unbounded.

## 5. Main flows
```mermaid
sequenceDiagram
    participant B as Business tx
    participant DB as outbox table
    participant R as Relay (200ms)
    participant RS as Redis
    participant W as Worker loop
    B->>DB: INSERT event, COMMIT
    R->>DB: BEGIN; SELECT ... FOR UPDATE SKIP LOCKED (batch 100)
    R->>RS: XADD stream {type, payload}
    alt XADD fails (Redis down)
        R->>R: stop batch; mark only published ones; retry next tick
    else ok
        R->>DB: UPDATE published_at=now(); COMMIT
    end
    W->>RS: XREADGROUP count 10 block 2s
    W->>W: process
    alt processing error
        W->>W: no XACK, stays pending
    else ok
        W->>RS: XACK
    end
    Note over R,W: crash after XADD but before COMMIT => row re-published (duplicate message)
```

### Delivery guarantee
At-least-once end to end. Duplicates can arise from (a) relay crash/rollback after XADD, (b) worker crash before XACK, (c) XAUTOCLAIM of a slow message. Idempotency by component:

| Component | Idempotent? | How |
|---|---|---|
| Relay publish | No (may publish a row twice) | not de-duplicated; downstream must cope |
| Step start processing | Recording: yes. Handler effect: yes per handler (G19) | unique `(instance, step, attempt)` guards the record; the handler effect is not atomic with it (audit BE-C1), so each handler is itself safe to run twice, see the table below |
| Result processing (engine) | Yes | status/current_step guard, row lock |
| Recorded-result replay | Adds a duplicate result event | engine drops it |
| Approval finalize insert | Yes | 23505 ignored (`approval/service.go:544-550`) |

### Handler redelivery matrix (G19)
The handler effect commits before `recordAndEmit`, so a crash, timeout or XAUTOCLAIM between the two re-runs the handler with the same `(instance, step, attempt)`. Review result, one row per registered handler. Test: `test/integration/money/handler_redelivery_test.go` runs each handler twice and compares side-effect snapshots; `EveryRegisteredHandlerHasAScenario` fails when a handler is registered without a row here and a scenario there.

| Handler | Side effects | Class | Redelivery behaviour |
|---|---|---|---|
| `validator` | none (reads) | pure read | same answer |
| `wallet` (MOVE_FUNDS) | ledger post + balances | guarded write | ledger posting guard, `ErrAlreadyPosted` returns success (ADR-0010, BUG-0001) |
| `confirmer` | `UPDATE transactions ... WHERE status='processing'` | guarded UPDATE | second run matches no row; already `completed` is reported as success `replayed` (BUG-0020). No notification is sent yet; one added later needs a key from instance + step |
| `fraud` | `INSERT fraud_flags` + advisory goroutine | duplicating insert, now guarded | advisory lock per transaction, reuse the existing flag, no second advisory (BUG-0019) |
| `round_up` | ledger post of the rounding | guarded write | same posting guard, kind `round_up` |
| `action_pipeline` | outbound HTTP per Directional Integration action | external call | the call IS repeated; header `Idempotency-Key = ref_code:action_code`, stable across attempts and recovery re-emits, so the partner must dedupe. Gap: delivery depends on the partner honouring the key; GL actions are not executed here |
| `kyc_scorer` | LLM call + `SetKYCScore` | external call + guarded read | replay guard returns the stored score when `ai_score >= 0`. A crash between the LLM answer and `SetKYCScore` pays for a second call; the result is a score, not a money effect |
| `kyc_classifier` | `ClassifyKYCGuarded` + user kyc status + offer in one tx | guarded UPDATE | returns the recorded status when the request is no longer `pending`; offer follows only when the guarded change applied |
| `boom` | test only | n/a | n/a |

### Other consumers of relay duplicates
`relay.go` publishes at-least-once: a crash after XADD and before `MarkOutboxPublished` publishes the same row again. The stream has exactly two kinds of consumers, both tolerant: a duplicated `step.start` hits `processStepMessage`, which re-emits the recorded result when the step is already recorded and otherwise runs a handler from the matrix above (E04, plus the redelivery test); a duplicated `step.result` hits the engine, which drops it by status and `current_step` (E06, BR-OB-08). No other code reads the streams or the outbox table, so no further consumer needs a dedupe key. The advisory goroutine in `fraud` is started only when the flag is newly created.

## 6. Invariants
- An event row exists iff its producing transaction committed (BR-OB-01).
- A row is marked published only after its XADD returned OK in the same relay transaction (`relay.go:66-80`).
- A row held by one relay (row lock) is skipped by others (BR-OB-02).
- Backoff delays never block a relay (BR-OB-05).

## 7. Data
`outbox` (migration 000003 lines 69-78): `id BIGINT identity PK`, `aggregate VARCHAR(50) default 'system'`, `event_type VARCHAR(50)`, `payload JSONB`, `available_at TIMESTAMPTZ default now()`, `published_at`, `created_at`. Partial index `idx_outbox_pending (available_at) WHERE published_at IS NULL` (the claim query orders by `id`, so the index only helps the filter). See `../reference/data-model.md`.

Redis keys: `wf:events`, `wf:steps:validator|wallet|confirmer|action_pipeline|round_up|fraud|kyc_scorer|kyc_classifier` (one per registered handler name, `consumer.go:46-50`). Streams are never trimmed by code (no XTRIM found).

## 8. Interfaces
No HTTP endpoint exposes the outbox or streams. Internal interface: `outbox.NewRelay`, `Relay.Run`, `Relay.RelayOnce(ctx, batch)` (exported for tests), `Worker.PumpLocal`.

### PumpLocal
`Worker.PumpLocal` (`consumer.go:266-317`) claims due outbox rows, marks them published BEFORE processing, commits, then runs the events in-process, bypassing Redis. It skips `step.start` events whose handler is not in its registry (left unpublished). It is a test/dev harness; it is not wired in `main.go` (the only non-test reference is its own definition). Because it marks published before processing, an error loses the event (audit Medium "Outbox"). Nothing technically blocks its use in production.

### Background goroutines (cmd/server/main.go)
All six start only when the role runs workers (`worker` or `all`; `config.go:35`), at `main.go:146-151`, using one `workerCtx` cancelled on shutdown (`defer stopWorkers()`, line 92). The HTTP server is a separate goroutine (`main.go:82`) for api/admin/all roles.

| Goroutine | Interval | Work | Multi-replica behavior |
|---|---|---|---|
| `outbox.Relay.Run` (`relay.go:30`) | ticker 200 ms, batch 100 | claim and XADD | Safe: `FOR UPDATE SKIP LOCKED` row locks. Events from different relays may interleave, so cross-row ordering is not guaranteed |
| `workflow.Worker.Run` (`consumer.go:45`) | blocking XREADGROUP (count 10, block 2s); one loop per handler stream + one for `wf:events` | execute steps / apply results | Safe by Redis consumer group (each message to one consumer), BUT all replicas use consumer name `c1` (`consumer.go:24`); pending-entry ownership is therefore shared and XAUTOCLAIM can hand a message to any replica |
| `workflow.Recovery.Run` (`recovery.go:40`) | ticker 30 s | `XAUTOCLAIM` (idle >= 60 s, count 20, per stream) then `SweepStuck` (running, idle > 2 min, batch 20) | Sweep: safe (UPDATE lease + SKIP LOCKED, test E20). XAUTOCLAIM: duplicates execution risk bounded by `idx_step_exec` |
| `approval.Sweeper.Run` (`approval/sweeper.go:22`) | ticker 30 s | expire overdue pending approval requests, up to 50 per tick (`ListDueApprovalIDs` limit 50) | Safe: guarded `FinalizeApprovalRequest` claim per request in its own tx; loser gets `ErrAlreadyFinalized` and continues (`approval/service.go:496-505,593-594`) |
| `moneyrequest.Sweeper.Run` (`moneyrequest/sweeper.go:23`) | ticker 30 s | expire pending requests past `expires_at`, 50 per tick | Safe: guarded `UPDATE ... WHERE status='pending'` returns rows affected (`money_request.sql:67-70`) |
| `recon.Cron.Run` (`recon/cron.go:22`) | sleeps until the next 00:15 local server time, then daily; reconciles the previous day | `RunForDate(now-1d)` | Every replica runs it. Safe by `reconciliation_runs.run_date UNIQUE` (000005:137): the loser gets `ErrAlreadyRan` (23505) and logs at debug. Work (statement queries, report build, AI summary) is duplicated before the insert. A run missed while the process is down is not back-filled |

There is no leader election, advisory lock or Redis lock anywhere in these loops. HTTP-only roles (`api`, `admin`) start none of them.

## 9. Edge cases
- E05 broker outage: events remain in `outbox`; relay resumes (`TestEdge_E05_...`).
- E17 Redis down: relay logs and continues; consumer sleeps 1 s between failures.
- E04 redelivery, E06 duplicate result, E20 lost start event (sweeper re-emits): see `workflow-engine.md`.
- Poison event: a row whose XADD keeps failing blocks everything behind it in `id` order for that relay pass (`relay.go:69-74` breaks the batch).

## 10. Known gaps (as-is vs intended)
- Audit Medium "Outbox": at-least-once, no dead-letter, no attempts counter, poison event blocks the head of the queue; index ordered by `available_at` vs query `ORDER BY id`; `PumpLocal` marks published before processing.
- BE-C1 / G19: duplicate delivery re-runs the handler. Every handler is now safe (matrix in section 5). Open: no unique index on `fraud_flags(transaction_id)` (needs a migration), and `action_pipeline` relies on the partner honouring `Idempotency-Key`.
- Audit Medium "Workflow": `consumer.go:77` uses `time.Sleep(1s)` instead of selecting on `ctx.Done()`.
- BE-H7: `main.go:117` compares `Env != "production"` while config documents `dev|ci|demo` (affects which outbound gateway runs, not the outbox).
- Not in audit: no outbox purge, no stream trim, fixed consumer name `c1`, recon cron does not catch up missed days, no leader election.
- The old design docs describe asynq and golang-migrate; neither is used. Scheduling is the goroutines above.

## 11. Code map
| File | Responsibility |
|---|---|
| `outbox/relay.go:30,47,88` | Run, RelayOnce, destinationStream |
| `workflow/consumer.go:45,102,164,266` | Worker.Run, processStepMessage, recordAndEmit, PumpLocal |
| `workflow/recovery.go:40,56,82` | Run, autoClaim, SweepStuck |
| `store/queries/outbox.sql` | InsertOutbox, ClaimOutboxBatch, MarkOutboxPublished |
| `cmd/server/main.go:90-153` | worker wiring and goroutine start |

## 12. Open questions
- Should outbox rows be purged and streams trimmed (retention policy)? (no owner/date)
- Should the consumer name be per replica (hostname) as the comment in `consumer.go:24` suggests? (no owner/date)

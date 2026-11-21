---
title: Debug a stuck transaction
type: guide
status: verified-against-code
last_verified: 2026-11-21
---

# Debug a stuck transaction

Goal: find out why a transaction is not reaching `completed` or `failed`, and what the system will do about it on its own.

Sources: `internal/workflow/{engine,consumer,recovery}.go`, `internal/outbox/relay.go`, `internal/store/queries/{workflow,outbox}.sql`, migrations `000002`, `000003`, `000005`, `000011`. The SQL below was run against the local development database on 2026-11-21 (read-only SELECTs).

Connect (compose project `hpay`):
```powershell
docker exec -it hpay-postgres-1 psql -U HPay -d HPay
# or one-shot:  "SELECT ...;" | docker exec -i hpay-postgres-1 psql -U HPay -d HPay
```

## 1. The mental model

`transactions` (business row) -> `workflow_instances` (state machine, one per transaction) -> `step_executions` (one row per finished step attempt) -> `outbox` (events waiting to go to Redis Streams) -> Redis streams `wf:steps:<handler>` and `wf:events`.

A transaction is stuck when its instance stops advancing. The cause is one of: no worker running, outbox not relayed (Redis down), a handler that keeps failing or timing out, a parked approval, or an instance in a state the recovery sweep does not handle.

## 2. Status meanings

`transactions.status` (`txn_status`): `created`, `pending_approval` (parked on human approval), `processing`, `processing_async`, `completed`, `failed`, `reversed`. Terminal: `completed`, `failed`, `reversed`. `FailTransactionGuarded` only moves `created`, `processing`, `pending_approval`, `processing_async` to `failed`.

`workflow_instances.status` (`wf_status`):

| Status | Meaning | Who moves it on |
|--------|---------|-----------------|
| `running` | a step is dispatched or executing | worker result -> `OnResult` |
| `waiting_approval` | parked at a `human_approval` step | approval module when finalized or expired (sweeper, 30 s interval; requests expire after 24 h by default) |
| `recovering` | lease taken by the recovery sweep; step is being re-emitted | `ReEmitCurrentStep` sets it back to `running` |
| `compensating` | failed after a compensatable step; reversing in reverse order | compensation results |
| `completed` | all steps done | terminal |
| `failed` | a step failed terminally, or compensation failed | terminal (manual) |
| `compensated` | compensation finished | terminal |

`step_executions.status`: `completed` or `failed` (only finished attempts are recorded; a step in flight has no row).

`outbox.published_at IS NULL` means the event has not reached Redis yet.

## 3. Find the transaction and its instance

```sql
SELECT t.id, t.ref_code, t.type, t.status, t.amount, t.fee, t.failure_reason, t.created_at,
       w.id AS instance_id, w.status AS wf_status, w.current_step, now() - w.updated_at AS idle
FROM transactions t
LEFT JOIN workflow_instances w ON w.transaction_id = t.id
WHERE t.ref_code = 'REF-XXXXXXXX';
```
Expected: one row. `instance_id` NULL means the workflow was never started (the request transaction rolled back or the type has no workflow).

## 4. Find all stuck instances

```sql
SELECT w.id, w.status, w.current_step, now() - w.updated_at AS idle, t.ref_code, t.status AS txn_status
FROM workflow_instances w
LEFT JOIN transactions t ON t.id = w.transaction_id
WHERE w.status IN ('running','recovering','compensating')
ORDER BY w.updated_at
LIMIT 50;
```
`waiting_approval` is normal while an approval is pending; check it separately in section 7.

## 5. Look at the step trail

```sql
SELECT id, step_name, attempt, status, left(error, 120) AS error, started_at, finished_at
FROM step_executions
WHERE instance_id = '<instance_id>'
ORDER BY id;
```
How to read it:
- The last recorded step vs `workflow_instances.current_step`: if `current_step` is a step with NO row yet, it is dispatched but not finished (or its start event is unpublished, see section 6).
- `attempt` 1, 2, 3...: forward retries with backoff. `attempt` 101: a compensation execution.
- `status = failed` rows show the handler error. Terminal errors trigger `on_failure`; retryable ones create attempt+1 until the step's `retry` limit.
- Unique key is `(instance_id, step_name, attempt)`, so a replay cannot create a second row.

## 6. Check the outbox

```sql
SELECT count(*) FILTER (WHERE published_at IS NULL) AS unpublished, count(*) AS total FROM outbox;

SELECT id, event_type, aggregate, available_at, published_at, payload->>'step' AS step,
       payload->>'handler' AS handler, payload->>'instance_id' AS instance_id
FROM outbox
WHERE published_at IS NULL
ORDER BY id
LIMIT 20;
```
Interpretation:
- `unpublished` is normally 0 or a handful for a few hundred milliseconds. A growing number means the relay is not publishing: the server was started with `--role=api` or `admin` only (no worker), or Redis is down. The relay logs `xadd failed; keeping outbox rows`. Nothing is lost (outbox rows stay; E05, E17). After Redis returns, events publish in id order.
- The relay claims rows ordered by `id` with `FOR UPDATE SKIP LOCKED`. There is no attempt counter or dead-letter: a poison event that cannot be published blocks nothing but is retried forever (audit: outbox Medium).
- Events for one instance: add `AND payload->>'instance_id' = '<instance_id>'`.

## 7. Parked on approval

```sql
SELECT a.id, a.status, a.subject_type, a.expires_at, a.created_at
FROM approval_requests a
WHERE a.status = 'pending'
ORDER BY a.created_at DESC
LIMIT 20;
```
`approval_status`: `pending`, `approved`, `rejected`, `expired`, `cancelled` (maker cancelled, `000011`). The sweeper expires overdue requests every 30 seconds and fails the instance cleanly (E10).

## 8. Money consistency checks

Balances must equal the ledger sum for non-system wallets (system wallets may go negative):
```sql
SELECT w.id, w.balance,
       COALESCE(SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END), 0) AS ledger_sum
FROM wallets w LEFT JOIN ledger_entries l ON l.wallet_id = w.id
WHERE w.owner_type <> 'system'
GROUP BY w.id
HAVING w.balance <> COALESCE(SUM(CASE l.direction WHEN 'credit' THEN l.amount ELSE -l.amount END), 0);
```
Expected: 0 rows (verified on the dev database). Ledger entries for one transaction:
```sql
SELECT l.id, l.wallet_id, l.direction, l.amount, l.balance_after, l.created_at
FROM ledger_entries l JOIN transactions t ON t.id = l.transaction_id
WHERE t.ref_code = 'REF-XXXXXXXX' ORDER BY l.id;
```
A completed transfer has one debit and one credit of equal amount (plus fee legs if a fee applies). MORE than one pair for a transaction suggests the MOVE_FUNDS double post (audit BE-C1, open): `ledger_entries` has no unique guard on posting.

## 9. What the system does by itself (as implemented)

1. Outbox relay: every 200 ms publishes unpublished rows.
2. Redelivery: messages read but not ACKed stay pending in the Redis consumer group; `XAUTOCLAIM` in the recovery loop (every 30 s, idle >= 60 s) reprocesses them. The worker re-emits the recorded result for an already recorded attempt.
3. Stuck sweep (every 30 s): `ClaimStuckInstances` selects instances with `status = 'running'` and `updated_at` older than 2 minutes (`FOR UPDATE SKIP LOCKED`), sets them `recovering` (the UPDATE is the lease), and `ReEmitCurrentStep` re-dispatches the current step with the next free attempt number, then sets `running`. Two concurrent sweeps do not double-claim (E20).
4. Retries: `RetryableError` leads to attempt+1 after backoff, up to the step's `retry`.
5. Failure: terminal error -> `on_failure`: `fail` (instance `failed`, transaction `failed` via the guarded update), `compensate` (reverse completed compensatable steps; success -> `compensated`).

## 10. Where recovery does NOT help (known gaps)

| Situation | Behavior today | Audit |
|-----------|----------------|-------|
| Instance stuck in `compensating` | the sweep only claims `running`; nothing recovers it | Compensation fail (partial) |
| Compensation step fails (e.g. receiver already spent the money) | `onCompensateResult` logs `COMPENSATION FAILED - manual intervention required` and sets the instance `failed`; it does NOT call `FailTransactionGuarded`, so the transaction can stay `processing`; the `wf_status` enum has no `compensation_failed` | BE-H4, Compensation fail |
| Compensation retry | compensation always uses attempt 101; an existing record is only re-emitted, never re-run | confirmed |
| Instance `recovering` forever | the lease is released only by `ReEmitCurrentStep`; if the process dies between the claim and the re-emit, it stays `recovering` and the sweep does not reclaim it (it only matches `running`). The dev database shows `recovering` rows left behind by tests | observation |
| Two server replicas | schedulers are in-process goroutines; sweeps are lease-safe, the recon cron has no lease | ADR 0005 |

## 11. Manual recovery (only after understanding the case; take a backup first)

These are operational patterns, not shipped tooling. Always do them in a transaction and record the reason in the audit trail.

- Instance `recovering` with a stale `updated_at`: put it back to `running` so the sweep picks it up again after 2 minutes:
  ```sql
  BEGIN;
  UPDATE workflow_instances SET status = 'running', updated_at = now() - interval '5 minutes'
  WHERE id = '<instance_id>' AND status = 'recovering';
  COMMIT;
  ```
- Transaction stuck non-terminal after the instance is `failed` (compensation failure): confirm with section 8 whether money moved. If no ledger entries exist, fail it with the guarded rule: `UPDATE transactions SET status='failed', failure_reason='manual: <why>' WHERE id='<id>' AND status IN ('created','processing','pending_approval','processing_async');`. If money moved and could not be returned, do NOT edit balances or ledger (append-only trigger blocks it); use the admin reversal flow (`POST /v1/admin/reversals`, requires approval) and see ADR 0009 for the planned `needs_manual` status.
- Never `UPDATE` or `DELETE` `ledger_entries` (blocked by trigger on purpose) and never edit `wallets.balance` by hand.

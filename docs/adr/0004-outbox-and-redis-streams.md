---
title: ADR 0004 Transactional outbox and Redis Streams
type: decision
status: verified-against-code
last_verified: 2026-11-21
---

# ADR 0004: Transactional outbox and Redis Streams

Status: Accepted (verified in code 2026-11-21)

## Context
The workflow engine runs steps asynchronously. Publishing an event before the database commit can lose or invent work; publishing after commit can lose events on a crash.

## Decision
- Business code and the engine write events to the `outbox` table in the SAME database transaction as the state change (`InsertOutbox`).
- `outbox.Relay` is the only publisher to Redis Streams. It polls every 200 ms, claims unpublished rows `ORDER BY id ... FOR UPDATE SKIP LOCKED`, `XADD`s them, and marks them published in the same transaction. If Redis fails it stops and rolls back, leaving the rows for the next pass.
- Routing: `workflow.step.start` goes to `wf:steps:<handler>`; everything else goes to `wf:events`.
- Consumers use consumer groups (`workers`, `engine`). A message is ACKed only after processing; un-ACKed messages are re-claimed with `XAUTOCLAIM` (idle at least 60 s). Delivery is at-least-once, so handlers and the engine must be idempotent; `step_executions` has a unique key `(instance_id, step_name, attempt)`.

## Consequences
- No event is published before commit and none is lost after a Redis outage (E05, E17 tests).
- Duplicates are possible; every consumer must tolerate them. Known gap: a handler that commits its own side effect before the step record can still repeat it (audit BE-C1, open).
- No dead-letter or attempt counter on outbox rows (audit, Medium); ordering is by id only.
- Redis is required at runtime for progress, but not for accepting requests (events queue in Postgres).
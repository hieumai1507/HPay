---
id: ADR-0011
title: Idempotency keys are per user and bound to the request, stored on the transaction
status: proposed
date: 2026-11-21
supersedes: []
superseded_by: []
spec: specs/w1-idempotency/spec.md
affects:
  - architecture/crosscutting-concepts.md
  - domain/transfer-and-fees.md
  - domain/money-requests.md
  - domain/wallet.md
  - reference/data-model.md
  - reference/api.md
  - reference/error-codes.md
constitution: [C-03]
---

# ADR-0011: Idempotency keys are per user and bound to the request, stored on the transaction

## Context
`transactions.idempotency_key` is globally `UNIQUE` (migration 000002:57). Services treat any unique violation whose constraint name contains `idempotency` as "return the earlier transaction", so another user's key, or the same user's key with a different payload, silently returns someone else's or a stale result (audit BE-H1). The engine path invents a random key when none is sent, and the header is enforced on two routes only. Constitution C-03 requires per-user scope, payload binding and mandatory keys. See `domain/transfer-and-fees.md`, `architecture/crosscutting-concepts.md` section 2.

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| A. `UNIQUE (initiated_by, idempotency_key)` plus a `request_hash` column on `transactions` (chosen) | Smallest change; atomic with the money write (one insert, one constraint, same DB transaction); concurrency is settled by the unique index; no new moving part | Keys live as long as transactions (no expiry); hash column is nullable for old rows; only transactions and money requests are covered, other future objects need their own columns | Low: one migration, no new store |
| B. Separate `idempotency_keys` table (user, key, hash, response, expiry) | Can cache the full response and expire keys; generic for any endpoint | Two writes that must stay atomic with the money write; response cache duplicates data and can drift; a second table to migrate, back-fill and clean; more code on a money path | Medium: new table, retention job, failure modes between the two writes |
| C. Redis-only keys with TTL | Cheap, natural expiry | Redis is not a source of truth (ADR-0004 treats it as a transport); a Redis outage or eviction silently disables protection on the money path (violates C-03/C-13 spirit); not atomic with the insert | High: unsafe for money |
| D. Keep global uniqueness, only add the hash | No schema constraint change | Cross-user collision and leak remain | Does not fix BE-H1 |

## Decision
Use option A. Drop the global unique constraint; add `UNIQUE (initiated_by, idempotency_key)` and a nullable `request_hash VARCHAR(64)` (SHA-256 hex of a canonical, length-prefixed encoding of the fields that define the operation, with an operation tag). Lookup is always `initiated_by = caller AND idempotency_key = key`. Same key and same hash (or a stored NULL hash, meaning a pre-migration row) returns the original result; a different hash returns HTTP 409 `ERR_IDEMPOTENCY_KEY_REUSED`. A missing key is rejected (HTTP 400 `ERR_IDEMPOTENCY_KEY_REQUIRED` at the route, `ErrIdempotencyKeyRequired` in the services); the random fallback is removed. Money requests (create, split) get the same columns and a per-requester partial unique index; splits are serialized with `pg_advisory_xact_lock`. The shared helpers live in `internal/wallet/idempotency.go`. `RequireIdempotencyKey` is added to `POST /v1/money-requests` and `/split`.

## Constitution check
C-03: implemented by this decision. C-08: migration 000015 has a down and is tested on populated data. C-05: lookups cannot return another user's row. No deviation.

## Consequences
- A client that retries with a fresh key per attempt is not deduplicated (unchanged); a client that reuses a key for a different action now gets 409 instead of a wrong success.
- Mobile and admin must send a stable key per user action on money-request create and split (now mandatory).
- Keys are never expired; the table grows with transactions. Acceptable now; a retention rule would need option B or a cleanup job.
- Old rows have `request_hash = NULL` and accept a replay with any payload; this closes as old keys age.
- The down migration restores global uniqueness and fails if two users share a key; that is deliberate.
- `internal/org` still uses the legacy key-only lookup (follow-up, outside this slice): payload binding is missing there, and a repeat by user B of a key that user A also used can resolve to A's oldest row until org adopts `wallet.FindReplay`.
- Monitor: rate of `ERR_IDEMPOTENCY_KEY_REUSED` (client bugs) and 23505 on `uq_txn_user_idempotency`.

## Documentation to update
`architecture/crosscutting-concepts.md` (Idempotency), `domain/transfer-and-fees.md`, `domain/money-requests.md`, `domain/wallet.md`, `reference/data-model.md`, `reference/api.md`, `reference/error-codes.md`, `constitution.md` (C-03 status), `audit/AUDIT-REPORT.md` (BE-H1).

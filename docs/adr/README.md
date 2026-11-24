---
title: Architecture decision records
type: decision
status: verified-against-code
last_verified: 2026-11-21
---

# Architecture decision records

One decision per file. Format: Status, Context, Decision, Consequences. Accepted ADRs are immutable: to change a decision write a new ADR that supersedes it and mark the old status "Superseded by NNNN" (that status line is the only permitted edit).

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-monorepo.md) | Monorepo with a docs knowledge base | Accepted |
| [0002](0002-english-only-documentation.md) | English-only documentation | Accepted |
| [0003](0003-money-as-int64-minor-units.md) | Money as int64 minor units | Accepted (implemented) |
| [0004](0004-outbox-and-redis-streams.md) | Transactional outbox and Redis Streams | Accepted (implemented) |
| [0005](0005-in-process-schedulers-not-asynq.md) | In-process schedulers, not asynq | Accepted (implemented) |
| [0006](0006-docker-based-backend-tests.md) | Docker-based backend tests | Accepted (implemented) |
| [0007](0007-kyc-limits-attached-to-offers.md) | KYC limits attached to offers | Accepted, implemented (weekly windows open) |
| [0008](0008-server-side-pin.md) | Server-side PIN | Accepted, implemented by ADR-0014; reset by ADR-0016 |
| [0009](0009-reversal-failure-needs-manual-and-fail-closed-fraud.md) | Reversal failure needs manual, fail-closed fraud | Accepted, reversal part implemented (ADR-0013); `compensation_failed` open |
| [0010](0010-ledger-postings-guard.md) | Guard ledger postings with a unique (transaction, kind) record | **Accepted** (approved 2026-11-21) |
| [0011](0011-idempotency-per-user-bound-to-request.md) | Idempotency keys are per user and bound to the request | **Accepted** (approved 2026-11-21) |
| [0012](0012-explicit-app-env.md) | APP_ENV is required and validated | **Accepted** (approved 2026-11-21) |

Template for a new ADR: `../_templates/adr.md` (front matter with status, supersedes, affects, constitution). Number it next in sequence and add it to this table.

Lifecycle: an ADR is written as **Proposed** during `hpay-feature` stage 3 (design gate) and reviewed by the user before any code; it becomes **Accepted** in stage 9 once shipped and verified (the existing "Accepted, not yet implemented" rows were decided by the user but have no code yet); it can later be **Superseded by NNNN**, **Deprecated** or **Rejected**. The engineering principles every ADR is checked against are in [`../constitution.md`](../constitution.md).
| [0013](0013-reversal-uniqueness-and-needs-manual.md) | One reversal per transaction in the database; impossible reversals parked as needs_manual | **Accepted** (approved 2026-11-21) |
| [0014](0014-transaction-pin-gate.md) | Hold a transaction in awaiting_auth until the server verifies the PIN | **Accepted** (approved 2026-11-21) |
| [0015](0015-admin-rbac.md) | Enforce admin permissions on the server from the database role | **Accepted** (approved 2026-11-21) |
| [0016](0016-pin-reset.md) | Reset a forgotten PIN with password plus a one-time code, with an operator fallback | **Accepted** (approved 2026-11-21) |
| [0017](0017-totp-second-factor.md) | TOTP built in-house as the 2fa band factor, and a pluggable code-delivery provider starting with Twilio (Messaging) | **Accepted** (approved 2026-11-21) |
| [0018](0018-pin-pepper-and-hash-cost.md) | Protect the PIN hash with a server-side pepper and a lighter argon2 cost | **Proposed** (awaiting owner decision) |

---
title: HPay engineering constitution
type: decision
status: accepted
last_verified: 2026-11-21
code: []
---

# HPay engineering constitution

The short list of principles every change must respect. It exists so that a plan can be checked against something concrete before any code is written (`hpay-feature` stage 3, "Constitution check") and so that reviewers have one reference (`hpay-review`).

How to read it
- **Binding for all new and changed code**, even where today's code still violates a principle. Existing violations are tracked in `audit/AUDIT-REPORT.md` and the "Known gaps" section of each `domain/` page; they are fixed by `hpay-bugfix`, not copied into new code.
- The "Status today" column is a snapshot taken from the audit and the verified domain pages on 2026-11-21. It tells you where the principle is already protected and where it is only a rule. Re-verify before relying on it.
- A plan may deviate from a principle only with a written justification in the plan's Constitution check and an ADR. An unjustified deviation blocks the plan.
- Changing this file needs an ADR (`adr/`) and the user's approval. Never edit it to make a plan pass.

| ID | Principle | Status today | Where it is (or should be) enforced |
|----|-----------|--------------|-------------------------------------|
| C-01 | **Money is an integer in minor units** end to end (DB `BIGINT`, JSON integer, no floats, no `Number()`/`parseInt()` on amounts in the apps). Inputs have an upper bound; multiplication checks overflow. | partial: backend uses int64, no float money found; overflow and upper bounds are missing; admin and mobile use `Number()` in places | `domain/ledger.md`, `fee`, frontends (`mobile/src/lib/money.ts`) |
| C-02 | **The ledger is append-only and double-entry.** Entries are never updated or deleted; every transaction's debits equal its credits; a posting can happen at most once per transaction and posting kind. | mostly enforced: append-only trigger (migration 000007) and at-most-once posting per (transaction, kind) (ADR-0010, migration 000014); balance equality is still checked in Go only | `internal/ledger`, `domain/ledger.md` |
| C-03 | **Idempotency is scoped per user and bound to the request.** Same key and same payload returns the original result; same key and a different payload is an error; keys are required on every money-moving write. | mostly enforced: per-user unique key plus request hash, 409 on mismatch, key mandatory on every mounted money route (ADR-0011, migration 000015); gap: `internal/org` still replays by key only | `domain/transfer-and-fees.md`, middleware |
| C-04 | **Locks are taken in a fixed order** (wallets by ascending id, in one statement) and never inside a loop. | enforced for wallet locking | `internal/ledger` |
| C-05 | **State changes only through the owning transition function**, guarded by the expected current state. No raw `UPDATE ... SET status` elsewhere. | mostly enforced: transitions are guarded; reversal and approval gaps closed (ADR-0013); compensation failure still has gaps (BE-H4 for workflows) | `domain/workflow-engine.md`, `domain/reversal.md` |
| C-06 | **Events leave the system only through the outbox, written in the same DB transaction as the state change.** Consumers are idempotent. | partial: outbox exists; money steps (MOVE_FUNDS, round-up) are now replay-safe through the ledger guard; other consumers are not verified | `domain/outbox-and-events.md` |
| C-07 | **Authentication is verified on the server.** PIN/OTP/2FA that a flow requires is checked server-side with hashing, attempt counter and lockout; a field that is accepted but unused is a FAIL. Transaction reads check `initiated_by`. | enforced for engine transactions (ADR-0014): PIN (argon2id, 5 attempts, 15 minute lock), OTP band, 2fa refused until a provider exists; not yet covered: PIN reset, org disbursement uses maker-checker | `domain/identity-and-auth.md` |
| C-08 | **Authorization has two layers.** Ownership predicate in SQL for user-owned rows; admin routes check the specific role/permission and write an audit row (actor, reason) for destructive or configuration actions. | mostly enforced: ownership predicate on engine confirm (ADR-0014); admin routes enforce a permission from the database role on every request (ADR-0015); AI key/setting writes audited; gaps: other admin writes not all audited, KYC file route has no admin check (BE-H9) | `domain/identity-and-auth.md` |
| C-09 | **Environment behavior is explicit.** `APP_ENV` is required and validated; dev-only behavior (OTP in responses, mock gateway, mock bank route) is impossible outside dev; no default secret outside dev. | enforced (ADR-0012): required validated `APP_ENV`, mock bank route absent in production, no default or placeholder secret outside dev and ci | `reference/configuration.md` |
| C-10 | **Outbound URLs from configuration or the database are validated** (https, host allowlist, private/loopback/metadata addresses blocked, redirects limited) before any request. | enforced for the action pipeline gateway (`internal/gateway/guard.go`, BUG-0014); a future admin endpoint that writes `services.actions` must call the guard when saving | `domain/service-catalog.md` |
| C-11 | **No secrets or PII in the repository or in logs** (passwords, OTP, tokens, full phone, balances at info level or above, KYC images). Third-party API keys are not stored in plaintext. | partial: `.env` is ignored; AI keys are stored in plaintext in the database | `reference/configuration.md`, `domain/ai-integration.md` |
| C-12 | **Day and month limits use one fixed time zone** (Asia/Ho_Chi_Minh), not server local time. | not enforced (BE-H2 note) | `internal/offerguard` |
| C-13 | **Risk and money decisions fail closed.** If a check cannot be completed (fraud/LLM unavailable, reversal impossible), the transaction is held or flagged for review; it is never silently let through and never produces a negative balance. Product decision recorded in `adr/0009-*`. | partial: reversal failure now fails closed (needs_manual, ADR-0013); fraud advisory is still fail-open | `domain/fraud-and-recon.md`, `domain/reversal.md` |
| C-14 | **Every behavior change ships with a test and a knowledge-base update.** Tests are written first for money, auth and workflow code; the domain/reference pages change in the same change; each bug fix adds a record to `bugs/`. | process rule enforced by the `hpay-feature` and `hpay-bugfix` skills | `.claude/skills/` |
| C-15 | **The code is the fact; documentation describes what exists.** Planned work lives in `specs/` and `planning/`; `history/` is never a source of truth. Everything is written in English. | process rule | `docs/README.md` |

## Constitution check (used in every plan)

Copy this table into the plan's "Constitution check" section and fill it in for the principles the change touches; write `n/a` with a reason for the others only when it is not obvious.

| ID | Touched? | Compliant? | How (or justified deviation + ADR number) |
|----|----------|------------|-------------------------------------------|
| C-01 | | | |
| ... | | | |

A plan that touches money, auth, workflow or migrations must show an explicit answer for C-01 to C-08 and C-13.

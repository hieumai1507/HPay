---
id: ADR-0014
title: Hold a transaction in awaiting_auth until the server verifies the PIN
status: proposed
date: 2026-11-21
supersedes: []
superseded_by: []
spec: specs/w3-transaction-auth/spec.md
affects:
  - domain/transfer-and-fees.md
  - domain/identity-and-auth.md
  - reference/data-model.md
  - reference/api.md
  - architecture/runtime-views.md
constitution: [C-03, C-05, C-07, C-08, C-11]
---

# ADR-0014: Hold a transaction in awaiting_auth until the server verifies the PIN

Status: **proposed**. It implements the direction the user already accepted in ADR-0008; it is written under the lead's instruction to complete the plan before review, so the user approves the concrete design with the final change set.

## Context
Money moves when `engine.Start` runs inside `ExecuteEngineRequest`; `confirm` and `verify` only echo state and have no ownership check (audit BE-C2). ADR-0008 fixed the policy (server-side PIN, attempt counter, lockout, confirm verifies). This ADR chooses how the workflow is held.

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| **A. New transaction status `awaiting_auth`; the request inserts the transaction without starting the workflow; `confirm` verifies the credential, then moves `awaiting_auth -> processing` with a guarded UPDATE and starts the workflow in the same transaction** | Reuses the existing workflow definitions unchanged; the gate is outside the engine so it cannot be skipped by a step order mistake; idempotent through the guarded transition; unconfirmed requests never touch the ledger | New status in a few queries (quota sums, expiry); the transaction row exists before authentication | Low to medium |
| B. An `AUTH` step inside every workflow definition, parking like `human_approval` | Everything in one engine | Every definition and all existing instances change; the pause hook currently supports only organization wallets; high blast radius | High |
| C. Do not create the transaction until `confirm` (keep the request in memory or Redis) | No new status | Loses the idempotency anchor (the key lives on the transaction), needs a second store, and the quoted fee could drift | Medium |
| D. Verify the PIN in the request call itself and drop the confirm step | Simplest | Breaks the documented request/confirm contract the mobile app already follows and removes the quote-then-confirm UX | Medium |

## Decision
Adopt **A**. `awaiting_auth` is a non-terminal status owned by the transfer service: `awaiting_auth -> processing` (confirm succeeded), `awaiting_auth -> failed` with reason `auth_expired` (10 minutes, swept by the existing sweeper loop pattern). It does not consume offer quota. PIN verification lives in the `auth` package behind one function so callers cannot compare hashes themselves; failures are counted per user with an atomic SQL update and audited; a lock refuses even a correct PIN until it expires. OTP uses the existing OTP service; `2fa` is refused until a provider exists (fail closed).

## Constitution check
- C-07 now enforced for money movement; C-08 ownership predicate in SQL; C-03 confirm starts the workflow once; C-05 guarded transitions; C-11 the PIN is never logged or returned.
- No deviation. One accepted limitation: lockout is per user, so a stranger who knows a phone number cannot lock a victim's PIN because PIN checks require an authenticated session of that user; an attacker with the victim's session could lock them (ADR-0008 already notes the cooling period).

## Consequences
- Easier: the money path has one authenticated gate; the contract that `auth_required` already advertises is now true.
- Harder: every client must call `confirm` with a credential; existing users have no PIN and must set one before their first money movement; large amounts that require `2fa` are refused until a provider is chosen.
- Risk: expiry must run reliably or unconfirmed rows linger (harmless to money, visible in lists). Follow-ups: 2fa provider, PIN reset flow, per-transaction confirm rate limit.

## Documentation to update
`domain/transfer-and-fees.md`, `domain/identity-and-auth.md`, `reference/data-model.md`, `reference/api.md`, `architecture/runtime-views.md`, `adr/0008-server-side-pin.md` (status line), `adr/README.md`, `constitution.md` (C-07, C-08).
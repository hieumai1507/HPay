---
title: PIN pepper and lighter argon2 cost (implementation of ADR-0018)
type: spec
status: in-progress
last_verified: 2026-11-24
adr: ../../adr/0018-pin-pepper-and-hash-cost.md
audit: [G25]
---

# Spec: PIN pepper and lighter argon2 cost

Stages 1 to 3 of `hpay-feature` are collapsed into this spec and `plan.md`: ADR-0018 (accepted by the owner on 2026-11-24) already decided the design, the hash format, the migration by rehash, rotation and loss, and the test plan. This file records what is built from it, the assumptions made where the ADR is silent, and the acceptance criteria.

## Problem
`users.pin_hash` holds `argon2id(PIN)` with the login-grade cost (m=64 MiB, t=3, p=2) and no secret. A 6 digit PIN has only 1,000,000 values, so a copy of the column lets an attacker test every PIN offline. The same cost caps payment intake at about 10 requests per second on 4 CPUs (load test baseline).

## Goals
1. The PIN hash is `argon2id(HMAC-SHA256(pepper, pin), salt, m=19456 KiB, t=2, p=1)`, with the pepper held only in configuration.
2. Existing users notice nothing: legacy hashes keep verifying and are rewritten to the new format by the first successful verification.
3. The pepper can be rotated with `PIN_PEPPER_ID` and `PIN_PEPPER_OLD`, and its loss is recoverable through the existing PIN reset (ADR-0016).
4. A runbook with a SQL count of hashes per key id.

## Non-goals
Login password hashing (stays m=64 MiB, t=3, p=2, no pepper). Any endpoint, any SQL migration, any admin or mobile change. HSM or KMS (option D of the ADR).

## Acceptance criteria
- **AC-1** Config: `PIN_PEPPER` (base64, 32 bytes) is required when `APP_ENV` is not dev or ci; dev and ci get a fixed test value; a malformed, wrong-length value, a bad key id (must match `[a-z0-9-]{1,16}`), a malformed `PIN_PEPPER_OLD` entry, an empty id, or a duplicate id (including one equal to the current id) refuses startup; no error message contains the secret value; startup logs only the key id.
- **AC-2** A new PIN hash is `$argon2id$v=19$m=19456,t=2,p=1,k=<id>$<salt>$<hash>` built from `HMAC-SHA256(pepper, pin)`; the right PIN verifies, a wrong PIN, a wrong pepper and a wrong key id do not.
- **AC-3** The parser accepts the `k` field and legacy hashes (no `k`, raw PIN, their own parameters); it refuses unknown, duplicate or missing fields and parameters above the bounds (m at most 131072 KiB, t at most 8, p at most 8) before any memory is allocated.
- **AC-4** An unknown user or a user without a PIN still spends one derivation with the new parameters. A hash whose key id is not configured fails verification as an ordinary wrong PIN (same client error, same failure counter), without a panic; the log names the key id and the user id only.
- **AC-5** After a successful verification, a legacy hash, a hash with a non-current key id or older parameters is recomputed and stored in the same request with `UPDATE ... WHERE id = $1 AND pin_hash = <old value>`. A failed verification never rewrites. A concurrent change or reset of the PIN is not overwritten. A failed rewrite does not fail the verification (warn log with the user id only).
- **AC-6** Every creator of a PIN hash (set, change, reset confirm, `testkit.SetPIN`) writes the new format. Operator reset only removes the hash. `cmd/seed` creates no PIN.
- **AC-7** Rotation: a hash under `p1` verifies after `p2` becomes current with `p1` in `PIN_PEPPER_OLD`, and is rehashed under `p2`. After `p1` is removed, an unmigrated user fails cleanly and PIN reset still works.
- **AC-8** Leak property: the stored hash plus a wrong pepper, or plus no pepper (raw PIN), cannot verify the known PIN.
- **AC-9** Neither the PIN, the pepper nor the HMAC output appears in slog output, in returned errors or in audit rows during set, verify (success and failure), rehash and rotation.
- **AC-10** `docs/runbooks/pin-pepper.md` has key generation, storage rules, rotation, loss and the count query.
- **AC-11** Every derivation, old and new, goes through the existing CPU-count semaphore.
- **AC-12** The load test is re-run with the baseline settings and the before and after numbers are recorded with the caveat that they come from one laptop in Docker.

## Invariants touched
C-01 to C-08 and C-13 are untouched for money paths. Auth: the PIN attempt counter and lockout (BR-AUTH-28), the cooling period (BR-AUTH-39) and the reset flow (BR-AUTH-36 to 38) keep their behaviour. No secret in logs, audit rows or the database.

## Decisions and assumptions (ADR silent or open)
1. **Parameter bounds**: m at most 131072 KiB (128 MiB), t at most 8, p at most 8, m at least 8 x p, t and p at least 1, salt 8 to 64 bytes, hash 16 to 64 bytes. Both formats use them. The strongest hash in use today (m=65536, t=3, p=2) is inside them.
2. **Unusable hash** (unknown key id, malformed): treated as a wrong PIN, including the failure counter and lockout, after one dummy derivation. The reset flow (ADR-0016) clears the counter. This avoids an oracle on which users still hold a retired key.
3. **A user without a PIN** gets `ErrPINNotSet` exactly as before, now after one dummy derivation.
4. **No audit row for a rehash** (it is an internal storage upgrade, not a user action). A debug log line holds the user id and the new key id.
5. **`ChangePIN`** verifies the current PIN without rehashing, because it stores a new hash right after.
6. **A service with no pepper set fails closed** (`ErrPepperNotConfigured`); only `Wire` and tests set it. Production and demo cannot start without `PIN_PEPPER`.
7. **`VerifyPassword` refuses a hash that carries `k=`** so a PIN hash can never be checked through the password path.
8. **The rewrite does not touch `updated_at`, the counters or `pin_reset_at`**, so the cooling period is not restarted.
9. **Equal-key aliasing** (the same bytes under two ids) is allowed; only ids must be unique.
10. No SQL migration: `pin_hash` is text. One new sqlc query, `RehashUserPIN`.

## Architecture impact
`amend`: `architecture/crosscutting-concepts.md` and `architecture/deployment.md` mention the TOTP key as a secret; they gain the pepper. Pages read: both of those. Domain: `domain/identity-and-auth.md`. Reference: `reference/configuration.md`. Quality: `quality/security-model.md`, `quality/performance/2026-11-24-load-test-baseline.md`. ADR-0018 is accepted and is not edited except for an "Implemented" note.

## User guide impact
`not affected`: backend only, no screen, label or flow changes.

## Edge cases covered
E11 style brute force is unchanged. New: concurrent verify of a legacy hash, concurrent PIN change during a rehash, key retired while hashes remain, hostile hash parameters.

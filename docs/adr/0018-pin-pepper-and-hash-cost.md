---
id: ADR-0018
title: Protect the PIN hash with a server-side pepper and a lighter argon2 cost
status: proposed          # awaiting owner decision; nothing in this ADR is implemented
date: 2026-11-24
supersedes: []
superseded_by: []
spec: specs/w3-transaction-auth/spec.md
affects:
  - domain/identity-and-auth.md
  - reference/configuration.md
  - quality/performance/2026-11-24-load-test-baseline.md
constitution: []
---

# ADR-0018: Protect the PIN hash with a server-side pepper and a lighter argon2 cost

An accepted ADR is immutable. This one is **proposed**: no code, no parameter and no migration changes until the owner decides. Related: ADR-0008 (server-side PIN), ADR-0014 (PIN gate), ADR-0016 (PIN reset), ADR-0017 (TOTP key handling).

Legend used below: **[measured]** came from a run recorded in `quality/performance/2026-11-24-load-test-baseline.md`; **[estimate]** is arithmetic or an expectation that has not been measured; **[code]** is read from the source.

## Context
- The transaction PIN has 6 digits, so only 1,000,000 values exist **[code: `auth.ValidatePIN`]**. If an attacker gets a copy of the `users.pin_hash` column (SQL injection, backup leak, a stolen replica), every PIN can be tried offline. Argon2 cost only slows each guess; the work splits across any number of machines, so the cost changes the price of the attack (hours instead of seconds), not whether it works **[estimate]**. With a hash whose salt is stored next to it, one guess costs one argon2 evaluation; 1,000,000 guesses at the current cost are on the order of hours on a few cores, and minutes on a rented fleet **[estimate, not benchmarked]**.
- Online guessing is already limited: an attempt counter and a 15 minute lockout **[code: `auth.VerifyPIN`, ADR-0014]**. The risk in this ADR is therefore the database-only leak, not the API.
- The PIN is hashed with the same argon2id routine as the password: m=64 MiB, t=3, p=2, encoded as `$argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>` **[code: `auth/password.go`, `VerifyPIN` calls `VerifyPassword`]**. The encoded string already carries the parameters, which the migration below relies on.
- The load test showed PIN verification is the intake bottleneck: about 10 confirm requests per second on 4 CPUs, 600 of 600 transfers correct, about 9.6 requests per second **[measured]**. 50 concurrent verifications would need about 3.2 GB of memory **[estimate from 64 MiB x 50]**; a semaphore now bounds concurrency to the CPU count **[measured: memory and latency stable; throughput unchanged]**.
- A lighter cost raises throughput but also makes offline guessing cheaper, which is the wrong direction unless something else compensates.

## Options considered

| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| A. Keep as is (m=64 MiB, t=3, p=2, no pepper) | No work, no new secret | Intake stays about 10 req/s per 4 CPUs; a database-only leak still exposes all PINs in hours | None now; the throughput problem stays |
| B. Lower the cost only (OWASP minimum m=19 MiB, t=2, p=1) | Simple; several times cheaper per check, more intake **[estimate; to be measured]** | Offline brute force becomes cheaper too; a database leak is worse than today | Small code change; security gets worse |
| C. Pepper plus B's cost: `HMAC-SHA256(pepper, pin)` then argon2id with the lighter cost | A database-only leak reveals nothing that can be tested offline without the key; recovers throughput; key id allows rotation | New secret to manage and lose; an attacker with the database AND the application key (full server compromise) is back to brute force at the lighter cost | Medium: config, hash format, rehash-on-verify, runbook |
| D. PIN verification inside an HSM or cloud KMS | Strongest: the key never leaves the device, per-guess rate limiting in hardware | New dependency, cost, latency on the payment path, vendor lock-in | Large; deferred until the product justifies it |

## Decision (proposed)
Adopt **C with B's cost**, as a follow-up to be built with `hpay-feature` after the owner accepts this ADR. D stays deferred and is not blocked by C (the pepper step can later be done by a KMS call). No hashing parameter is changed by this document.

1. **Pepper.** New config `PIN_PEPPER` (base64, 32 random bytes), loaded like `TOTP_ENCRYPTION_KEY`: required when `APP_ENV` is not dev or ci, dev and ci get a fixed test value, malformed or short values refuse startup. A key id is configured next to it (`PIN_PEPPER_ID`, default `p1`); old keys stay in a list (`PIN_PEPPER_OLD=p0:<base64>`) while hashes that use them exist.
2. **Hash input.** `argon2id(HMAC-SHA256(pepper, pin_bytes), salt, m=19 MiB, t=2, p=1)`. HMAC output is 32 bytes and replaces the PIN as the argon2 password.
3. **Hash format.** Self-describing, so old and new hashes verify side by side: `$argon2id$v=19$m=19456,t=2,p=1,k=<pepper id>$<salt>$<hash>`. A hash without `k=` is a legacy hash: verified exactly as today (raw PIN, its own parameters). Parsing already reads `m,t,p` from the string; the parser must accept the extra `k` field and refuse unknown ones.
4. **Migration without downtime and without a SQL migration.** No schema change: `pin_hash` is text. On every successful `VerifyPIN` (including the change-PIN and reset flows), if the stored hash is legacy or uses a non-current key id or older parameters, recompute with the current pepper and parameters and update the row in the same request. Users who never log in again keep their legacy hash until they do; a report lists the remaining count. Both formats are verified for as long as any legacy hash exists.
5. **Constant-time compare** of the derived hash (`subtle.ConstantTimeCompare`), as today; an unknown user or a user without a PIN still spends one dummy derivation (no timing oracle).
6. **No secrets in logs.** The pepper, the HMAC output and the PIN never appear in logs, errors, audit rows or traces; startup logs say only the key id.

### What a database-only leak reveals
- Before (today): salt and argon2 hash of each PIN; all 1,000,000 values can be tested offline, hours on modest hardware **[estimate]**.
- After C: the same table, but each hash is of `HMAC(pepper, pin)`; without the pepper nothing can be tested, so a database-only leak (backup, replica, SQL injection that reads tables) reveals nothing about PINs **[design property, to be proven by test, see below]**. If the pepper is also taken (host compromise, config leak), the attacker is at the lighter cost, i.e. worse than today **[estimate]**. The pepper therefore must not be stored in the database, in backups of the database, or in the same secret file as `DATABASE_URL` where avoidable.

### Rotation and loss
- **Rotation (planned):** add the new key with a new id as current, keep the old one in `PIN_PEPPER_OLD`, deploy; rehash-on-verify moves users over as they authenticate; when the report shows zero hashes for the old id (or after a cutoff date), remove it. Hashes still on a removed key fail verification and the user uses PIN reset.
- **Loss (unplanned):** if the current pepper is lost and there is no copy, no PIN with that key id can be verified. Recovery is the existing ADR-0016 PIN reset (password plus one-time code, operator fallback) for every affected user. A reset campaign and a status message must be prepared before go-live. Store the pepper in the secret manager with versioned backups held by two people.

## Test plan (not yet written)
- Unit: HMAC then argon2 round trip; wrong PIN, wrong pepper and wrong key id fail; parser accepts `k=`, refuses unknown fields and bad parameters; legacy hashes still verify; constant-time compare used (review plus a test on the helper).
- Config: `PIN_PEPPER` required in production and demo, malformed and short values refused, dev and ci defaults (same shape as `TestConfig_TOTPKeyRequiredOutsideDev`).
- Migration: a legacy hash verifies and is rewritten to the new format in the same request; a failed verify does not rewrite; two concurrent verifies of one user do not corrupt the row; both formats verify side by side.
- Rotation: hash under `p1`, rotate to `p2` keeping `p1` old, verify succeeds and rehashes; remove `p1`, verify of an unmigrated user fails cleanly and PIN reset works.
- Leak property: a copy of the table plus a wrong pepper cannot verify the known PIN.
- Logging: a test that captures logs and errors during set, verify (success and failure) and rotation and asserts neither the PIN nor the pepper appears.
- Load: re-run the load test in `quality/performance` with the new parameters and record before and after.

## Expected effect (to be re-measured)
The current baseline is about 10 intake requests per second on 4 CPUs at m=64 MiB, t=3, p=2 **[measured]**. At m=19 MiB, t=2, p=1 each check costs roughly one third to one fifth of the CPU and about a third of the memory, so intake could rise several-fold until another limit (database, workflow) takes over **[estimate; not measured]**. After implementation the load test must be re-run on the same settings (200 users, 50 workers, 60 s, `TEST_DB_MAX_CONNS=40`) and the before and after numbers recorded in the performance note. Do not claim the gain until that run exists.

## Constitution check
No principle is weakened by C: secrets stay out of the repository and the database, and money paths are unchanged. Option B alone would weaken the offline-attack posture and is not recommended without the pepper.

## Consequences
- Easier: higher PIN verification throughput, a database leak alone no longer exposes PINs, a clear path to an HSM or KMS later.
- Harder: one more production secret with a rotation and loss runbook; backups of the pepper must be kept apart from database backups; support must be ready for a mass PIN reset if the key is lost.
- Risks: pepper in the same trust zone as the application (full host compromise defeats it); operator error during rotation. Mitigations: key ids, `PIN_PEPPER_OLD`, a count of hashes per key id, rehash on verify.
- Monitor: count of legacy hashes, PIN verify failures by reason, PIN verify latency, memory during the load test.
- Follow-up: `hpay-feature` run after acceptance; PLAN.md G25.

## Documentation to update
`domain/identity-and-auth.md` (PIN hashing rules), `reference/configuration.md` (`PIN_PEPPER`, `PIN_PEPPER_ID`, `PIN_PEPPER_OLD`), `quality/performance/` (before and after run), `adr/README.md`, runbook for pepper rotation and loss.
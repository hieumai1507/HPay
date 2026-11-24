---
title: Plan for the PIN pepper
type: plan
status: done
last_verified: 2026-11-24
spec: spec.md
---

# Plan: PIN pepper (ADR-0018)

## Tasks
| Id | Files | Depends on | Size | Test that proves it | Effort |
|----|-------|-----------|------|---------------------|--------|
| T1 | `docs/specs/w5-pin-pepper/*`, `test-cases.json`, workbook | none | S | cases rendered | high |
| T2 | `internal/platform/config/config.go`, `config_test.go`, `.env.example` | T1 | M | `TestConfig_PinPepper*` | high |
| T3 | `internal/auth/password.go` (strict, bounded parser shared by both formats), new `internal/auth/pin_pepper.go` (`PepperSet`, `HashPIN`, `VerifyPINHash`, `BurnPINWork`) | T1 | M | `TestPinHash_*` in `internal/auth` | high |
| T4 | `internal/auth/pin.go`, `pin_reset.go`, `service.go`, `sender_twilio.go` (`Wire`), `internal/store/queries/auth.sql` (`RehashUserPIN`), `cmd/server/main.go`, `internal/api/router.go` | T2, T3 | M | `TestPinRehash_*` (internal/auth), `test/integration/authn/pin_pepper_test.go` | high |
| T5 | `test/testkit/env.go` (pepper for the test service) | T4 | S | existing PIN tests still pass | medium |
| T6 | `docs/runbooks/pin-pepper.md`, domain, reference, quality, ADR note, README, PLAN | T4 | M | review | low |
| T7 | `docs/quality/performance/2026-11-24-load-test-baseline.md` new section | T4 | S | the load test run | medium |

Tasks T2 and T3 touch disjoint files. Commits: `test(auth)` (red tests and stubs), `feat(auth)` (implementation), `docs`.

## Schema and API
No migration (`pin_hash` is text). One sqlc query. No endpoint, so `openapi.yaml` is unchanged.

## Rollout and rollback
Deploy with `PIN_PEPPER` set (production and demo refuse to start without it). Legacy hashes keep working. Rollback to the previous build is only safe while no hash has been rewritten, because the old build cannot read the `k=` format: after the first rewrite, a rollback needs a mass PIN reset. This is stated in the runbook. Keep the pepper in the secret manager before the first deploy.

## Risks
Pepper loss (runbook: mass reset). A wrong pepper in one environment makes every PIN there fail and counts failures, so lockouts follow; the runbook says to check the count query before and after a deploy. Lighter cost raises the cost of an offline attack only when the pepper is also known; accepted by ADR-0018.

## Constitution check
| Principle | Touched | Answer |
|-----------|---------|--------|
| Secrets never in the repo or database (config) | yes | pepper only from the environment; dev and ci use a fixed test value that protects nothing |
| Money paths | no | PIN verification result is unchanged for users |
| Audit without secrets | yes | no new audit rows; tests check existing rows |
| Concurrency safe | yes | guarded UPDATE for the rewrite; semaphore for every derivation |

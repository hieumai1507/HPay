---
title: Backend verification, w6-usd (US dollars as cents)
type: handoff
status: current
last_verified: 2026-11-26
---

# Backend verification: w6-usd

Branch `worktree-agent-a1bf9618184c251e1`, main merged in (merge commit 902e01f). Task T1 of `docs/specs/w6-usd/plan.md`.

## Commands and results
| Check | Command | Result |
|-------|---------|--------|
| Full suite | `.\scripts\test-backend.ps1 -Db usdfull1 -GoArgs "./... -race -count=1"` (fresh database, merged tree) | all 23 packages with tests `ok`, no FAIL |
| gofmt | `gofmt -l` in `golang:1.26` on LF copies of every non-generated .go file | 0 unformatted (run before the merge; src/backend is identical after it) |
| vet | `docker run golang:1.26 go vet ./...` in src/backend | exit 0 (same note) |
| OpenAPI | `python src/backend/scripts/check-openapi-sync.py` | exit 0, but the script is an empty file in the repository, so this proves nothing. By hand: `yaml.safe_load` parses 73 paths, no VND or dong text, 9 places say "amount in cents". NOT verified by a real checker |
| Migrations | `migrate/migrate:v4.17.1 up`, `down -all`, `up` on fresh database `usdmig` | all three succeeded; wallets default `'USD'`, 3 system wallets USD, p2p fixed fee 25 |
| Seed | `go run ./cmd/seed --e2e-admins` on `usdmig`, then SQL | exit 0; 15 wallets, 0 drift (balance = ledger sum), all USD, u1 = 49,800 cents; 1 open fraud flag, 1 pending disbursement, 1 failed instance, 1 manual_review user |
| Load | see `docs/quality/performance/2026-11-25-usd-cents.md` | PASS, 0 wallets differ from ledger sum |

Per package (race, count 1): cmd/seed, internal/api, approval, auth (135 s), fee, fraud, gateway, kyc, ledger, moneyrequest, offerguard, platform/config, qr, storage, user, wallet, workflow, workflow/handlers, test/integration/admin, authn, kyc, money, org: all ok. Packages with no test files are unchanged.

## Test cases
35 cases in `docs/specs/w6-usd/test-cases.json`: 26 Pass, 1 Blocked (TC-USD-034, OpenAPI checker is empty), 8 Not Run (TC-USD-005 to 011 admin and mobile, TC-USD-035 docs and guide). The migration and seed cases (030, 032) were run before the merge; src/backend did not change in the merge.

## Red then green
Red: `go test` on the converted tests failed to compile (`roundUpDiff`, `wallet.DefaultCurrency` undefined) and the integration packages failed on the old migration values (admin, authn, kyc, offerguard). Green: constant, migrations, seed, round-up unit, then the fixes below.

## Decisions and fixes
- Auth band edges are exclusive at the upper bound (existing behaviour): 200,000 cents is already 2FA. The new matrix test asserts 199,999 otp and 200,000 2fa.
- `PINCoolingCap` 2,500,000 became 10,000 and `orgTxnLimit` 1,000,000,000 became 4,000,000 (converted by 250; not in the spec table).
- Seed: demo amounts divided by 250, the e2e reversible pool is 40 transactions of 100 cents.
- Load test amounts are 40 to 399 cents.
- Migration 000006 (not named in the brief) also held dong defaults and was converted.

## Docs to update by the main session
`domain/wallet.md`, `domain/transfer-and-fees.md`, `product/kyc-tiers-and-limits.md`, `reference/configuration.md` (`ROUND_UP_UNIT`), and note that `check-openapi-sync.py` is empty.
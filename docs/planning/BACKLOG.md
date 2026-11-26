# Backlog

Work that is agreed but not scheduled. Pick items from the top. Move an item to a spec under `docs/specs/` when you start it, and delete it here when done.

## Load tests (added 2026-11-25)

Today there is one load test: `src/backend/test/loadtest/p2p_load_test.go` (`TestP2PLoad`, build tag `loadtest`, not in CI). It covers only the P2P transfer path: request, confirm with PIN, workflow, ledger integrity. How to run it: `src/backend/scripts/loadtest/README.md`; skill gate: `hpay-feature` stage 7 step 6. Results: `docs/quality/performance/`.

Keep every load test in `src/backend/test/loadtest/` behind the `loadtest` tag, so `go test ./...` and CI never run them. Add one file per feature.

| # | Item | Why |
|---|------|-----|
| L1 | Find out why 12 transfers failed in the after-ADR-0018 run (`2026-11-24-load-test-baseline.md`) | Not investigated; may be a real bug under load |
| L2 | End-to-end p50 grew from 43.7 s to 67.2 s after intake got faster: the workflow is now the limit. Profile consumers, relay and step handlers | Intake is no longer the bottleneck |
| L3 | Load test for offer limits: many parallel requests from one user against the daily limit (per-user lock, `offerguard.CheckWindows`) | Checks the limit holds under contention |
| L4 | Load test for top-up and withdraw, and for bill/service payments through the engine | Only P2P is covered |
| L5 | Load test for login and PIN verify (argon2 cost, rate limits) | The pepper change moved the cost here |
| L6 | Load test for admin reads (workflow list, KYC queue, audit) with large tables | No data volume tests exist |
| L7 | Rewrite or delete `scripts/loadtest/k6-transfer.js` (stale: old endpoints, hits rate limits) | Dead file |
| L8 | Optional scheduled or manual GitHub workflow (`workflow_dispatch`) that runs the load test and uploads the numbers | Numbers are noisy on a laptop; a fixed runner is better |
| L9 | Repeat each before/after load run 3 times and report the median | One run per column is not reliable |

## Found by the review of the US-dollar change (2026-11-26, not fixed)

Fixed the same day: the split-bill screen crash on a total below one cent per person (HIGH), the per-transaction limit test at 40,001 cents, the wrong "$400.00/day" copy in the mobile KYC and onboarding screens (now $5,000.00, the seeded L1 daily limit), and the raw cents in the fraud AMOUNT_SPIKE text. These remain:

| # | Item | Why |
|---|------|-----|
| R1 | Existing databases: migrations 000002, 000004, 000006 and 000023 were edited in place, so a database that already ran them keeps `VND` wallets and every lookup for `USD` finds nothing (`EnsureMainWallet` would add a second, empty wallet). ADR-0019 says no such database exists | Add a guard or a conversion migration before any real database is created |
| R2 | Mobile `parseAmountToBigInt` has no maximum on the money-request and transfer-amount screens (`requests/create.tsx`, `requests/split.tsx`, `transfer/amount.tsx`); a huge number is sent and the server refuses it with a generic error | AC-4 says over-maximum is rejected in the app |
| R3 | `formatAmountWhileTyping` turns a pasted `1.234.567` into `1.23` silently; AC-3 says it is rejected with a message | A user pasting a Vietnamese-formatted number gets $1.23 |
| R4 | Raw cents without a unit in the recon Excel detail JSON (`recon/xlsx.go`) and in the text sent to the LLM summary (`recon/recon.go:217`) | Reads as dollars |
| R5 | `fee.Calc` has no overflow check on `amount * percent_bp`, and `FeePreview` has no upper bound on `amount` (latent: the seeded p2p policy has 0 bp) | A huge amount wraps negative |
| R6 | The load test now sends 40 to 399 cent amounts against a 10,000 cent daily cap, so most requests are refused by the daily limit and the integrity check runs under lighter load than before | Use larger amounts or an L1 sender so the test keeps its pressure |
| R7 | Stale comments and messages: `offer_limits_test.go` ("2.5M", "3M"), `limit_window_race_test.go` ("2.5M", "1,249,500"), `wallet/currency_test.go` ignores a `WalkDir` error and only finds double-quoted literals | Misleading on failure |
| R8 | Stale non-code references: `.claude/skills/hpay-backend-recipes/references/seed-demo-data.md` (12,450,000), `hpay-qa-testcases/scripts/generate_test_cases.py` (all dong), onboarding mock balances vs the seed, `openapi.yaml` stays at version 0.10.0 and several amount fields have no "cents" note | Drift |
| R9 | No component test of mobile `AmountDisplay`; admin e2e has no money assertion | Display is covered only by unit tests |

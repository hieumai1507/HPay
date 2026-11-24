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
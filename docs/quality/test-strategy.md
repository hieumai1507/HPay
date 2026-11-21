---
title: Test strategy
type: quality
status: verified-against-code
last_verified: 2026-11-21
---

# Test strategy

Sources: test files under `src/` (counted 2026-11-21), `docs/audit/AUDIT-REPORT.md` section 6, `docs/handoff/baseline.md`, `src/backend/scripts/check-edge-tests.sh`, `.github/workflows/`. How to run everything: [guides/run-tests.md](../guides/run-tests.md).

## 1. Layers and what exists

| Layer | Tooling | Location | Count (2026-11-21) | Needs |
|-------|---------|----------|--------------------|-------|
| Backend unit and white-box | `go test`, colocated `*_test.go` | `src/backend/internal/**` | 20 files in 14 packages | nothing for pure logic; DB for some |
| Backend workflow integration (white-box) | `go test` | `internal/workflow/integration_test.go`, `handlers/` | 3 files | Postgres, Redis optional (`TEST_REDIS_ADDR`) |
| Backend cross-module integration (black-box) | `go test` | `src/backend/test/integration/{money,authn,kyc,org,admin}/` (shared harness in `src/backend/test/testkit/`) | 5 packages, 144 tests | `TEST_DATABASE_URL` (skips otherwise) |
| Backend gates | shell/python/CI | `scripts/check-edge-tests.sh`, `check-openapi-sync.py`, CI `migrate` job (E18, ledger trigger), `govulncheck`, Trivy | 5 gates | CI |
| Admin end-to-end | Playwright (chromium) | `src/admin/e2e/*.spec.ts` | 10 spec files | full stack, e2e seed |
| Admin unit | none | | 0 | |
| Mobile unit and component | Jest | `src/mobile/**/__tests__`, `*.test.ts(x)` | 15 files, 80 tests | nothing |
| QA workbooks (manual and semi-automated) | xlsx, skill `hpay-qa-testcases` | `docs/qa/test-cases/*.xlsx` (5 workbooks) | 5 | humans |

Backend totals: 46 `*_test.go` files, 147 top-level `Test*` functions (baseline: 160 tests run including subtests, 0 skipped when run with a database).

Backend files per package: api 1, approval 1, auth 4, fee 1, fraud 1, kyc 1, ledger 1, moneyrequest 1, offerguard 1, qr 1, storage 2, user 1, wallet 1, workflow 2 (+ handlers 1), `test/integration` 26.

Placement rule: unit and white-box tests are colocated in the same package; scenarios spanning modules (API -> engine -> ledger) go to `test/integration/`. Edge-case tests are named `TestEdge_<ID>_<slug>`. Shared database rule: packages that pump the outbox must not run concurrently (`-p 1`, `testutil.RunSerialized`); a helper must only consume outbox rows it owns.

CI (`backend-ci.yml`): `go test -p 1 ./... -race -count=1` with Postgres and Redis service containers, then the E01-E20 name gate (E18 is exempt, covered by the `migrate` job). `admin-ci.yml` runs the Playwright suite blocking. There is no CI job for mobile Jest, admin typecheck or lint (only `npm run build` inside the e2e job type-checks the admin). `golangci-lint` is disabled (TODO).

## 2. Baseline (docs/handoff/baseline.md)

Backend 160 of 160 pass with a database; admin and mobile `tsc` pass; mobile Jest 15 suites, 80 tests pass; admin ESLint 6 errors and 6 warnings; mobile ESLint 3 errors and 8 warnings (pre-existing); Playwright not run in the baseline.

## 3. What is missing (audit section 6 and baseline notes)

Packages with no colocated tests: `transfer`, `reversal`, `recon`, `outbox`, `org`, `vault`, `product`, `gateway`, `config` (some are exercised only through `test/integration`). Money-path fixes there must add tests first.

Missing tests listed by the audit, all still open:

| Missing test | Related finding |
|--------------|-----------------|
| Post to the ledger twice with the same transaction id | BE-C1 |
| Crash recovery of MOVE_FUNDS (failure between commit and `recordAndEmit`, then replay) | BE-C1 |
| Offerguard race (N goroutines, same user) | BE-H2 |
| Idempotency across users, and same key with a different payload | BE-H1 |
| Concurrent reversal, and reversal when the receiver is out of funds | BE-H3, BE-H4 |
| IDOR on confirm and verify, wrong PIN | BE-C2 |
| Expired or forged signed URL, path traversal in `storage.Local.Get` | BE-H9 |
| Property-based tests for `fee.Calc` | Medium (fee) |
| Frontend: no admin unit tests at all; no shared submit-guard test; hard-coded PIN not covered | Admin/Mobile #1 to #3 |
| Comparison of OpenAPI against frontend calls (contract tests) | audit section 2, OpenAPI vs frontend |

Structural weaknesses to remember: edge tests prove the named scenario, not the whole property. For example `TestEdge_E02_ConcurrentOverdraw` covers wallet overdraw, not the offer-limit race (BE-H2); `TestEdge_E01_IdempotentTopup` does not cover cross-user key reuse (BE-H1). The E06 "duplicate event" test does not cover a crash between the handler commit and the step record (BE-C1). The edge-ID gate only checks that a function with the right name exists, not that it is meaningful.

## 4. Edge-case catalog and coverage

Legend: covered = a test with the ID in its name exists in `src/`; partial = related test exists but not under the ID or only a leg of the scenario; missing = nothing found. "Covered" does not mean the whole property is proven (see section 3). IDs come from `docs/history/master-plan.md` and the QA generator (`.claude/skills/hpay-qa-testcases/scripts/generate_test_cases.py`).

### Backend (E01 to E20)

| ID | Scenario | Test function | Status |
|----|----------|---------------|--------|
| E01 | Idempotent topup (same key) | `TestEdge_E01_IdempotentTopup` (test/integration/money/wallet_test.go) | covered |
| E02 | Concurrent overdraw, no negative balance | `TestEdge_E02_ConcurrentOverdraw` (test/integration/money/ledger_test.go) | covered |
| E03 | Cross transfer A to B and B to A, no deadlock | `TestEdge_E03_CrossTransferNoDeadlock` (test/integration/money/ledger_test.go) | covered |
| E04 | Redelivery, no double side effect | `TestEdge_E04_RedeliveryNoDoubleSideEffect` (internal/workflow/integration_test.go) | covered (crash window BE-C1 untested) |
| E05 | Outbox survives broker outage | `TestEdge_E05_OutboxSurvivesBrokerOutage` (internal/workflow/integration_test.go) | covered (Redis leg needs `TEST_REDIS_ADDR`) |
| E06 | Duplicate result event idempotent | `TestEdge_E06_DuplicateResultEventIdempotent` (internal/workflow/integration_test.go) | covered |
| E07 | Compensation reverses moved funds | `TestEdge_E07_CompensationReversesMovedFunds` (test/integration/money/transfer_test.go) | covered (failing compensation untested) |
| E08 | Checker race, single finalize | `TestEdge_E08_CheckerRaceSingleFinalize` (test/integration/org/approval_test.go) | covered |
| E09 | Policy snapshot survives policy change | `TestEdge_E09_PolicySnapshotSurvivesPolicyChange` (test/integration/org/approval_test.go) | covered |
| E10 | Expired approval fails clean | `TestEdge_E10_ExpiredApprovalFailsClean` (test/integration/org/approval_test.go) | covered |
| E11 | OTP brute force | `TestEdge_E11_OTPBruteForce` (internal/auth/otp_test.go) | covered |
| E12 | Locked user fails mid-workflow | `TestEdge_E12_LockedUserFailsMidWorkflow` (test/integration/money/transfer_test.go) | covered |
| E13 | Limit by KYC tier | `TestEdge_E13_LimitByKycTier` (test/integration/money/offer_limits_test.go; since ADR-0007 the limit comes from the offer the KYC tier selects) | covered |
| E14 | Self transfer rejected | `TestEdge_E14_SelfTransferRejected` (internal/ledger/ledger_test.go) | covered |
| E15 | Duplicate bank callback | `TestEdge_E15_DuplicateBankCallback` (test/integration/money/wallet_test.go) | covered |
| E16 | Recon cutoff, no false mismatch | `TestEdge_E16_ReconCutoffNoFalseMismatch` (test/integration/admin/recon_test.go) | covered |
| E17 | Redis down, no data loss | `TestEdge_E17_RedisDownNoDataLoss` (test/integration/money/redis_down_test.go) | covered (recovery leg needs `TEST_REDIS_ADDR`) |
| E18 | Migration up, down, up | CI job `migrate` in `backend-ci.yml` (also `make migrate-verify`); not a Go test, exempt in `check-edge-tests.sh` | covered by CI |
| E19 | Money request expiry race with pay | `TestEdge_E19_ExpiryRacePay` (test/integration/money/moneyrequest_test.go); mobile `TestEdge_E19_PayExpiredRequest_SafetyCheck` (requests.test.ts) | covered |
| E20 | Stuck recovery lease, no double claim | `TestEdge_E20_StuckRecoveryLeaseNoDoubleClaim` (internal/workflow/integration_test.go) | covered |

### Mobile contract (M01 to M12)

| ID | Scenario | Test | Status |
|----|----------|------|--------|
| M01 | Double tap on transfer | `TestEdge_M01_DoubleTapTransfer_SingleRequest` (m3-core.test.tsx), `TestEdge_M01_IdempotencyKey_StablePerConfirmFlow` (idempotency.test.ts) | covered (audit says the real confirm screen guard was weak: Admin/Mobile #3) |
| M02 | Timeout right after pressing transfer | `TestEdge_M02_TransferPollTimeout_ProcessingCopy` (m3-core.test.tsx) | covered |
| M03 | Async transaction not finished | `TestEdge_M03_TransferFailed_ErrorMapDisplay` (m3-core.test.tsx); polling backoff test is groundwork | covered |
| M04 | Access token expires mid-operation | `TestEdge_M04_Refresh_SingleFlight_RetryOnce` (refresh.test.ts) | covered |
| M05 | Refresh token revoked | `TestEdge_M05_RefreshRevoked_ForceLogout` (refresh.test.ts) | covered |
| M06 | Large or unusual amounts | `TestEdge_M06_BigIntMoney_NoPrecisionLoss` (m3-core.test.tsx), `TestEdge_M06_Money_BigIntOnly` (money.test.ts), `TestEdge_M06_SplitBigIntRounding` (split.test.ts) | covered (audit: `Number()` still used in several screens) |
| M07 | App backgrounded on OTP/confirm screen | `TestEdge_M07_AppBackground_PrivacyOverlay` (security.test.tsx), `useSecureScreen (M07)` | covered |
| M08 | Approve deep link while logged out | `TestEdge_M08_DeepLink_PendingAuth` (security.test.tsx) | covered |
| M09 | Checker opens an order already decided | no test found (backend returns `ERR_ALREADY_FINALIZED`, covered by E08) | missing on mobile |
| M10 | Blurry or dark KYC photo | `TestEdge_M10_KYCImageHeuristic_Validation` (kyc.test.ts) | covered (QA marks it HUMAN for the real camera) |
| M11 | KYC upload loses network | `TestEdge_M11_KYCUpload_NetworkDropResume` (kyc.test.ts) | covered |
| M12 | Long history list, continuous scrolling | `Test_CursorPagination_NextPage` (m3-core.test.tsx) tests paging only | partial |

### Admin (A01 to A07)

| ID | Scenario | Test | Status |
|----|----------|------|--------|
| A01 | Two admins act on the same task | `src/admin/e2e/kyc.spec.ts`, `fraud.spec.ts`, `my-tasks.spec.ts` (tests titled "A01 ...") | covered (e2e) |
| A02 | New instance inserted while paging | `src/admin/e2e/workflow.spec.ts` "A02 (adapted)" | covered (adapted) |
| A03 | Session expires mid-review, filters preserved | `src/admin/e2e/edge-cases.spec.ts` "A03 ..." | covered |
| A04 | Signed KYC image URL expires | `src/admin/e2e/kyc.spec.ts` "A04 ..." | covered |
| A05 | Two tabs edit the same org policy | no e2e; backend race covered by `TestOrgAdminPolicy_ConcurrentInsertsRaceToOneWinner` and `TestOrgAdminPolicy_VersionConflictMapsToSentinelError` (test/integration/org/org_admin_policy_test.go, QA case TC-EDG-A05) | partial |
| A06 | Report export stays non-blocking | `src/admin/e2e/recon.spec.ts` "A06 ..." | covered |
| A07 | PII masked in workflow step JSON | `src/admin/e2e/edge-cases.spec.ts` "A07 ..." | covered |

Totals: 20 of 20 backend IDs have a gate or test; mobile 10 covered, 1 partial, 1 missing; admin 6 covered, 1 partial. The Playwright tests were not executed in this verification, only located by grep.

## 5. Priorities

1. Tests for the Critical and High backend findings (section 3) before fixing them.
2. A mobile test for M09 and a real scroll or virtualization test for M12; an admin e2e or component test for A05.
3. Run mobile Jest and admin typecheck/lint in CI.
4. Isolate the test database from the dev database (the script currently uses the dev database).

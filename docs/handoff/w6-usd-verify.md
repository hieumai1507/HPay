---
title: Verification, w6-usd (US dollars as cents), admin, mobile, docs
type: handoff
status: current
last_verified: 2026-11-26
spec: ../specs/w6-usd/spec.md
---

# w6-usd verification (2026-11-26)

Counts for the 35 cases of `docs/specs/w6-usd/test-cases.json`: 35 Pass, 0 Fail, 0 Blocked, 0 Not Run. The backend half is in `w6-usd-backend-verify.md`; this note covers admin, mobile, the OpenAPI check, the guide, the site and the cases that the backend note left open.

## Commands run in this session (merged tree, main)
| Check | Command | Result |
|-------|---------|--------|
| Backend, all packages | `.\scripts\test-backend.ps1 -Db usdfinal2 -GoArgs "./... -race -count=1"` (fresh database) | 23 packages `ok`, 0 FAIL, 0 data races |
| Admin | `npx tsc --noEmit`, `npm run lint`, `npm test`, `npm run build` | all exit 0; Vitest 8 files, 105 tests |
| Admin e2e | `npx playwright test --project=chromium` against `vite preview` on :4173 and the API on :8080, database `hpay_demo` freshly seeded with cents | 31 passed, 1 skipped, 0 failed |
| Mobile | `npx tsc --noEmit`, `npm run lint`, `npm test` | all exit 0; Jest 34 suites, 307 tests |
| Generated types | `npm run gen:api` in admin and mobile, then `tsc` in both | no VND text left; both type-check |
| OpenAPI sync | `python src/backend/scripts/check-openapi-sync.py` | exit 0, 80 operations. The script had been an empty file; see "Found on the way" |
| Guide | `node docs/guides/admin/check-guide.mjs` | OK all checks passed (36 images, 36 figures) |
| Site | `node site/build.mjs` then `node site/check.mjs` | site check ok |
| Dong sign | grep of site pages, README, guide markdown for the dong sign and VND | 0 |

The first Playwright attempt failed 20 tests with `ERR_CONNECTION_REFUSED`: the preview server was not running (a background task of the shell had ended). It was started as its own process and the same suite then passed. That was an environment problem, not a code problem.

## Case results
- Backend cases (TC-USD-001 to 004, 012 onward except those below): recorded in the backend note; the full suite was re-run on the merged tree (row 1 above).
- TC-USD-005, 006 (admin display and em dash), 008, 009 (parser accept and reject), 011 (admin bound): Vitest `money.test.ts` and `money-roundtrip.test.ts`.
- TC-USD-007 (mobile display), 008, 009, 010 (mobile bound $40,000.00 accepted, one cent more rejected): Jest `money.test.ts`, `money-bounds.test.ts`, `money-roundtrip.test.ts`.
- TC-USD-034: the OpenAPI check, now real.
- TC-USD-035 is a HUMAN case. It was executed as a read of the changed pages plus the two checkers and a grep, not as a full proofread of every page. Three VND mentions remain on purpose: two cite a Vietnamese regulation as the origin of an open legal question (`domain/identity-and-auth.md`, `domain/transfer-and-fees.md`), one points at a history document (`product/kyc-tiers-and-limits.md`).

## Gaps and limits
- No component test of mobile `AmountDisplay`; it is covered through `formatAmountDisplay` and its `$` prefix is one line.
- 36 guide screenshots were retaken by scripts and marked with callouts. About 20 were opened and checked by eye, including every one that shows an amount (10, 15, 18, 19, 34); the rest were saved as valid PNG files but not all inspected.
- The mobile app has never run on a device; nothing here says otherwise.
- GitNexus tools were not available to the agents, so blast radius came from grep and the test suites. This session did not run `detect_changes` either.
- Demo data: the KYC request and one fraud flag in `hpay_demo` were put back to pending and open by direct SQL for screenshots (local demo database only).
- Not run again after the final merge: gofmt and `go vet` (run before the merge by the backend task; `src/backend` changed afterwards only in the OpenAPI file and the sync script).
- Performance: the load test was run once for this change (see `docs/quality/performance/2026-11-25-usd-cents.md`): 0 wallets differ from their ledger sum. Numbers are noisy.

## Found on the way (not part of the spec)
- `src/backend/scripts/check-openapi-sync.py` had been an empty file since commit 7335331, so the CI step "openapi spec in sync with router" checked nothing. Restored from `d83923a`; it immediately found two real drifts (fixed in commit 6a483fa). A deliberate drift makes it exit 1.
- `hpay-api` shares Redis with the load test; running both makes the test's transfers stick in `processing`. Stop the container before a load run.
- `docs/guides/admin/check-guide.mjs` hard-coded the date 2026-11-24; it now accepts any dated version line.

## Independent review (read-only reviewer agent, 2026-11-26)
The reviewer read the diff and ran git and grep only (no builds, no tests). It found 12 items. I checked the HIGH one myself before fixing it.

Fixed in this change, each with a test that failed first or checks the boundary:
- **HIGH, split-bill screen crash.** `requests/split.tsx` called `splitEven` during render; since this change `splitEven` throws for a total below one cent per person, so typing `0.01` with two people crashed the screen. Reproduced by a new screen test (2 of 3 cases failed with "splitEven: total must be at least 1 cent per person"), fixed by showing no shares and a message ("The total must be at least $0.01 for each person") instead of throwing. `app/(main)/requests/__tests__/split.test.tsx`, 4 tests pass.
- **MEDIUM, per-transaction limit boundary.** `offerguard_test.go` only tested 10,000,001 against a 40,000 cent cap, so a bug such as "amount > cap + 1000" would pass. It now also checks 40,001.
- **MEDIUM, wrong limit in mobile copy.** The KYC intro, KYC status and onboarding screens said "$400.00/day"; the seeded L1 daily limit is 500,000 cents, $5,000.00. Corrected.
- **MEDIUM, raw cents in text.** The fraud AMOUNT_SPIKE evidence read "amount 20000 is 5 times the 30-day average (4000)". It now reads "amount $200.00 is 5 times the 30-day average ($40.00)" (`fraud.usd`, tests in `internal/fraud/usd_test.go`). The recon Excel and the recon LLM text still carry raw cents (R4 in the backlog).

Re-run after those fixes: backend `-race` 23 packages ok (fresh database `usdfinal3`); mobile `tsc` 0, lint 0, Jest 35 suites and 311 tests; `go build`, `go vet` on the changed packages and gofmt on the changed Go files clean.

Not fixed, recorded in `docs/planning/BACKLOG.md` (R1 to R9): existing databases would not be converted (ADR-0019 says none exist; no guard), no maximum on some mobile amount screens, a pasted `1.234.567` becomes `1.23` while typing, recon export and LLM text in raw cents, no overflow check in `fee.Calc`, the load test now sends small amounts against a 10,000 cent cap, stale comments, stale non-code references, and display tests. The reviewer could not check: roughly 60 integration tests (skimmed), the generated `schema.d.ts` diffs, and several mobile screens (home, history, qr).

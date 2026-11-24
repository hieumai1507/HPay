---
title: Daily and monthly limits can be exceeded by parallel confirms, and their windows follow the server time zone
id: BUG-0017
status: fixed
severity: high
area: backend/offerguard
tags: [race, money-math, state-machine]
audit: [BE-H2]
constitution: [C-12, C-13]
found: 2026-11-24
fixed: 2026-11-24
commit: pending
regression_test: src/backend/test/integration/money/limit_window_race_test.go ; src/backend/internal/offerguard/window_test.go
related: [BUG-0012, BUG-0018]
---

# BUG-0017: limit window race and time zone

## Symptom
A user could move more than the daily or monthly limit of their offer. At L0 the daily limit is 2,500,000 VND; five held requests of 1,500,000 each all passed the check and all five were started by confirming them in parallel (5 started in the red test run). Day and month windows started at midnight in the server time zone (UTC in the container), not in Vietnam.

## Root cause
`offerguard.Check` ran only when a request was created. Since ADR-0014 a request waits as `awaiting_auth`, and the volume query (`SumUserTxnAmountSince`) deliberately ignores `awaiting_auth`, so held requests do not count. Every sibling therefore saw the same used volume and passed; `ExecuteEngineConfirm` never re-checked, so the moment money starts to move was unprotected. The window start used `now.Location()` (`offerguard.go`), which breaks constitution C-12.

## Other places with the same flaw
- `TOPUP_BANK`, vault and legacy paths call `CheckAmount` (per-transaction only, no volume), so they have no daily or monthly limit to race on; unchanged.
- The interval throttle (`interval_seconds`) has the same read-then-insert pattern but is NOT re-checked at confirm (it would measure request time, not confirm time and could refuse an honest second confirm); it stays request-time only.
- Offers that need no confirmation start the workflow inside the request transaction; they are still checked only at request time (no lock). No seeded offer uses that path for a limited service. Recorded below.

## Fix
`ExecuteEngineConfirm` takes `pg_advisory_xact_lock` keyed on the user inside the short transaction that starts the workflow, then calls the new `offerguard.CheckWindows` (daily, monthly, interval) before `StartAwaitingAuthTransaction`. The lock is released at commit, after the status change that makes the amount count, so the next confirm of the same user sees it and is refused with `ERR_LIMIT_DAILY` or `ERR_LIMIT_MONTHLY`. The re-check passes amount plus fee (the volume sum includes fees), re-checks the enabled flag, minimum and per-transaction cap, and does not re-run the interval throttle (it measures the spacing of requests, not of confirms). A held request created before today's Vietnam midnight is refused with `ERR_AUTH_EXPIRED` and must be requested again, because the quota counts by creation time and would otherwise dodge today's volume (found in security review). An empty service code on a held request now fails closed. `windowStarts` computes day and month starts in a fixed UTC+7 zone (no tzdata needed in the distroless image).

## Regression test
`TestLimitWindow_ConcurrentConfirmsRespectDailyLimit`: five holds of 1,500,000 at L0, confirmed in parallel; exactly one starts and four get `ErrLimitDaily` (red before the fix: 5 started). Mutation check: weakening the exclusive lock to a shared lock made it fail on three runs (5 started, then 2 started). `TestLimitWindow_ConfirmCountsTheFee` (fee counted at confirm), `TestLimitWindow_RequestFromAnEarlierDayIsRefusedAtConfirm` (midnight crossing), and `TestWindowStarts_UseVietnamTime` (failed to compile before). Cases TC-B17-001 and TC-B17-002 in `BUG-0017-testcases.json`, workbook `docs/qa/test-cases/BUG-0017-testcase.xlsx`.

## How to recognize this again
- Completed volume of one user and type above the offer's daily or monthly limit.
- Several transactions of one user confirmed within the same second after being requested earlier.
- Limits that reset at 07:00 local Vietnam time (UTC midnight) instead of midnight.

## Docs updated
`domain/service-catalog.md`, `domain/transfer-and-fees.md`, `architecture/crosscutting-concepts.md`, `product/kyc-tiers-and-limits.md`, `reference/data-model.md`, `constitution.md` (C-12), `audit/AUDIT-REPORT.md`, `planning/PLAN.md` (G18).

## Not done
- The interval throttle bypass and the creation-time measurement were fixed in BUG-0018; the request-from-an-earlier-day refusal described above was replaced there by counting from `started_at`.
- A one-time code (otp, 2fa) is consumed before the limit refusal, so a refused confirm needs a new challenge.
- `OFFERGUARD_ENABLED=false` disables every limit in any environment (not gated on APP_ENV). Fixed: config refuses it outside dev and ci (BUG-0021 task 1, commit `fix(config)`).
- Money-request pay (`moneyrequest`) starts its workflow with the per-transaction check only and is outside the lock.
- A request that needs no confirmation (auth matrix says none) is only checked at request time, without the lock.
- The limit counts transactions by `created_at`, so a request created before midnight and confirmed after counts in the earlier day.
- Weekly rolling limits and the balance ceiling are separate gaps (G9).
- Money-request pay outside the lock: fixed in BUG-0021.

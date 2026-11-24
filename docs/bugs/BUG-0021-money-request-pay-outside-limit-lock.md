---
id: BUG-0021
title: Money-request pay ignores the daily, monthly and interval limits, so parallel pays exceed them
status: fixed
severity: high
area: backend/moneyrequest
tags: [race, money-math, state-machine]
audit: [BE-H2]
found: 2026-11-24
fixed: 2026-11-24
commit: see git log (fix(moneyrequest) [BUG-0021])
regression_test: src/backend/test/integration/money/moneyrequest_pay_limit_test.go:TestMoneyRequestPay_ConcurrentPaysRespectDailyLimit
related: [BUG-0017, BUG-0018]
---

# BUG-0021: money-request pay is outside the limit lock

## Symptom
A payer on OFFER_CUST_L0 (daily limit 2,500,000 VND) with five pending money requests of 1,500,000 each paid all five in parallel and all five started (5 started, 0 refused in the red test run). The same user is refused on the transfer confirm path (BUG-0017).

## Root cause
`moneyrequest.Service.Pay` called `offerguard.CheckAmount` (minimum and per-transaction cap only, no volume) and then inserted the transaction and started the workflow. It never took the per-user `limit-window` lock and never called `CheckWindows`, so the daily and monthly volume and the interval throttle were not enforced at all on this path, and parallel pays could not see each other. The BUG-0017 fix covered only `transfer.ExecuteEngineConfirm`. The row lock on the money request only serializes pays of the same request, not of different requests by one payer.

## Other places with the same flaw
- `TOPUP_BANK`, vault and the unmounted legacy paths use `CheckAmount` without volume; unchanged (see BUG-0017).
- A request that needs no confirmation is checked only at request time without the lock (still open, see BUG-0017 Not done).
- No HTTP route calls `Pay` today (docs/domain/money-requests.md), so this was reachable only through the service; the fix keeps it safe for when a route is added.

## Fix
`Pay` computes the fee, then takes `pg_advisory_xact_lock(hashtextextended('limit-window:' || payer_id, 0))` on its own transaction (after the request row lock, the same order as the transfer path) and calls `offerguard.CheckWindows(payer, "MONEY_REQUEST", p2p, amount, fee)` with the transaction's Queries. `CheckWindows` also covers the enabled flag, minimum and per-transaction cap, so the earlier `CheckAmount` call is replaced. The lock is held until commit, after the insert that makes the amount count (the insert sets `started_at = now()`), so the next pay sees it. No second pooled connection is requested. The first rejected pay returns `ErrLimitDaily`, `ErrLimitMonthly` or `ErrTooFrequent` and the request stays `pending`.

## Regression test
- `TestMoneyRequestPay_ConcurrentPaysRespectDailyLimit`: five requests of 1,500,000, paid in parallel behind a start gate; exactly one starts and four get `ErrLimitDaily`. Red before the fix (5 started).
- `TestMoneyRequestPay_WaitsForThePayersExclusiveLimitLock`: deterministic check of the lock mode; Pay must wait while another transaction holds the payer's lock in shared mode.
- Mutation check: weakening the lock to `pg_advisory_xact_lock_shared` makes both tests fail (the concurrency test failed with 2 started and 3 refused; the race can still pass by timing, which is why the deterministic test was added).

## How to recognize this again
- Completed P2P volume of one payer above the offer's daily or monthly limit, where the transactions came from money requests.
- Several money-request transactions of one payer started within the same second.

## Docs updated
`domain/money-requests.md` (BR-MRQ-08), `bugs/INDEX.md`, pointer lines in BUG-0017 and BUG-0018, `qa/test-cases/BUG-0021-testcase.xlsx`.

## Not done
- The money request is checked at Pay time only; split requests have no limit check at creation (by design, limits apply when money starts).
- No HTTP route exposes Pay yet.
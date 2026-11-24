---
title: The interval throttle and the quota windows measure when a request was made, not when money started to move
id: BUG-0018
status: fixed
severity: medium
area: backend/offerguard
tags: [race, state-machine, money-math]
audit: [BE-H2]
constitution: [C-12, C-13]
found: 2026-11-24
fixed: 2026-11-24
commit: pending
regression_test: src/backend/test/integration/money/limit_window_race_test.go
related: [BUG-0017, BUG-0012]
---

# BUG-0018: limits must measure when a transaction starts

## Symptom
Follow-up to BUG-0017. (1) An offer with `interval_seconds` (L2 P2P: 3 s) was meant to allow one transaction per interval. A user could hold several requests (each waits up to 10 minutes for its PIN) and confirm them together: all started, the throttle never fired. (2) The daily and monthly volume was counted by `created_at`, so a request made before midnight and confirmed after it counted in the old day, and a request made yesterday and confirmed today used none of today's quota.

## Root cause
`LastUserTxnAt` and `SumUserTxnAmountSince` filter on `transactions.created_at`, the time of the request. Since ADR-0014 a request can start long after it is created, and `awaiting_auth` rows are excluded from both queries. The column that matters, the moment money starts to move, did not exist.

## Other places with the same flaw
None known: only these two queries feed the offer windows. The first fix (BUG-0017) refused confirms of requests from an earlier day (`ErrWindowCrossed`); that workaround is gone now that the window is exact.

## Fix
Migration 000027 adds `transactions.started_at`. It is set to `now()` when a transaction is inserted as `processing` and by `StartAwaitingAuthTransaction`, and cleared when a transaction is held (`HoldTransactionForAuth`). Both queries use `COALESCE(started_at, created_at)`, so rows from before the migration keep their old meaning with no backfill. The per-user advisory lock added in BUG-0017 now also protects the throttle, because `CheckWindows` re-runs it at confirm.

## Regression test
`TestLimitWindow_VolumeIsCountedFromWhenTheTransactionStarted`: a 1,500,000 transfer started today but created two days ago still counts, so a new 1,100,000 request is refused (`ErrLimitDaily`). `TestLimitWindow_IntervalThrottleHoldsAtConfirm`: four held requests under a 3600 s interval, confirmed in parallel; exactly one starts and three get `ErrTooFrequent`. Mutation checks: counting volume by `created_at` again made the first test fail; weakening the advisory lock to a shared lock made the second fail (2 started). Test helper `testkit.AgeUserTransactions` moves both timestamps; five existing authn tests that aged only `created_at` to get past the throttle now use it.

## How to recognize this again
- Several transactions of one user with `started_at` within the same second although the offer has `interval_seconds` > 0.
- Volume of one user per Vietnam day above the offer's daily limit when summed by `started_at`.

## Docs updated
`domain/service-catalog.md`, `domain/transfer-and-fees.md`, `reference/data-model.md` (new column), BUG-0017 record, PLAN G18.

## Not done
- A request that needs no confirmation is still checked only at request time without the lock (no seeded offer uses that path for a limited service).
- Money-request pay is outside the lock.
- The one-time code of an otp or 2fa confirm is consumed before a limit refusal, so a refused confirm needs a new challenge.
- `OFFERGUARD_ENABLED=false` disables every limit in any environment. Fixed: config refuses it outside dev and ci (BUG-0021 task 1, commit `fix(config)`).
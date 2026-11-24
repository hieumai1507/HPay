---
title: US dollars as cents - test cases
type: spec
status: in-progress
last_verified: 2026-11-25
---

# Test cases: US dollars stored as cents

Source of the workbook: `test-cases.json` in this folder. Render it with `python -X utf8 .claude/skills/hpay-qa-testcases/scripts/render_feature_workbook.py w6-usd` (writes `docs/qa/test-cases/w6-usd-testcase.xlsx`).

| TC | AC | Type | Pri | Expected | Test | Status |
|----|----|------|-----|----------|------|--------|
| TC-USD-001 | AC-1 | Happy Path | P0 | The main wallet currency is USD and its balance is 0 | `TestCurrency_NewUserWalletIsUSD` | Not Run |
| TC-USD-002 | AC-1 | Edge Case | P0 | All three are USD | `TestCurrency_SystemWalletsAreUSD` | Not Run |
| TC-USD-003 | AC-1 | Edge Case | P1 | The database default is USD | `TestCurrency_DatabaseDefaultIsUSD` | Not Run |
| TC-USD-004 | AC-1 | Security/AuthZ | P0 | Only wallet.DefaultCurrency defines the string; no other file writes it | `TestCurrency_NoLiteralOutsideConstant` | Not Run |
| TC-USD-005 | AC-2 | Happy Path | P0 | The admin shows $1,234.56, $0.05 and $0.00 | `formatUsd` | Not Run |
| TC-USD-006 | AC-2 | Edge Case | P1 | An em dash is shown, never $0.00 | `formatUsd missing` | Not Run |
| TC-USD-007 | AC-2 | Happy Path | P0 | The mobile app shows $1,234.56, $0.05 and $0.00; a missing value is an em dash | `AmountDisplay` | Not Run |
| TC-USD-008 | AC-3 | Happy Path | P0 | The values are 1200, 1250, 1250 and 123456 cents | `parseUsdToCents` | Not Run |
| TC-USD-009 | AC-3 | Negative | P0 | Each is rejected with a message and nothing is submitted | `parseUsdToCents rejects` | Not Run |
| TC-USD-010 | AC-4 | Boundary | P0 | 40,000.00 is accepted, 40,000.01 is rejected (C-01 upper bound) | `MAX_AMOUNT` | Not Run |
| TC-USD-011 | AC-4 | Boundary | P1 | The bound is accepted, one above it is rejected | `MAX_MINOR_UNITS` | Not Run |
| TC-USD-012 | AC-5 | Boundary | P0 | The cap passes; cap plus one cent is refused with the per-transaction error | `TestOfferLimits_PerTxnCapsByTier` | Not Run |
| TC-USD-013 | AC-5 | Boundary | P0 | 10,000 passes, 12,000 is refused (falls back to L0) | `TestOfferLimits_NoOfferRowFallsBackToL0` | Not Run |
| TC-USD-014 | AC-5 | Edge Case | P0 | Exactly one request starts and the daily limit holds | `TestLimitWindow_ConcurrentConfirmsRespectDailyLimit` | Not Run |
| TC-USD-015 | AC-5 | Edge Case | P0 | The second confirm is refused because amount plus fee exceeds the daily cap | `TestLimitWindow_ConfirmCountsTheFee` | Not Run |
| TC-USD-016 | AC-5 | Edge Case | P0 | Exactly one payment starts | `TestMoneyRequestPay_ConcurrentPaysRespectDailyLimit` | Not Run |
| TC-USD-017 | AC-5 | Boundary | P0 | PIN below 8,000, OTP from 8,000 to 200,000, 2FA above | `TestOfferLimits_SeededAuthMatrix` | Not Run |
| TC-USD-018 | AC-5 | Boundary | P1 | 10,000 is allowed, 10,001 is refused with the cooling error | `TestPINReset_CoolingPeriodLimitsLargeTransactions` | Not Run |
| TC-USD-019 | AC-6 | Happy Path | P0 | The fee is 25 cents in both cases and lands on the system fee wallet | `TestP2PFixedFeeIs25Cents` | Not Run |
| TC-USD-020 | AC-6 | Boundary | P0 | 0 (rounds down) | `TestCalc_PercentRoundsDownToTheCent` | Not Run |
| TC-USD-021 | AC-6 | Boundary | P0 | 25 and 25 | `TestCalc_PercentRoundsDownToTheCent` | Not Run |
| TC-USD-022 | AC-6 | Edge Case | P1 | The fee is raised to the min and clamped to the max; never negative | `TestCalc` | Not Run |
| TC-USD-023 | AC-7 | Happy Path | P0 | Shares are 334, 333, 333 and sum to 1,000 | `TestSplitEven` | Not Run |
| TC-USD-024 | AC-7 | Negative | P0 | Rejected: every share must be at least one cent | `TestSplitEvenRejectsTooManyShares` | Not Run |
| TC-USD-025 | AC-7 | Happy Path | P1 | Three requests are created and their amounts sum to 3,000 | `TestSplitBillThreeWay` | Not Run |
| TC-USD-026 | AC-8 | Happy Path | P0 | 66 cents go to the vault; the sender pays amount plus fee plus 66 | `TestRoundUp_AppliesDifference` | Not Run |
| TC-USD-027 | AC-8 | Boundary | P0 | No round-up; the vault stays at 0 | `TestRoundUp_ExactAmount_Skipped` | Not Run |
| TC-USD-028 | AC-8 | Edge Case | P1 | 66, 0 and 99 cents | `TestRoundUpDiff` | Not Run |
| TC-USD-029 | AC-8 | Edge Case | P1 | Default 100; 500 is honoured; the old name is ignored; a bad value falls back to 100 | `TestRoundUpUnitFromEnv` | Not Run |
| TC-USD-030 | AC-9 | Happy Path | P0 | Every step succeeds; the down files stay valid (C-08) | `migrate up/down/up` | Not Run |
| TC-USD-031 | AC-9 | Boundary | P0 | L0, L1, L2 per transaction, day, month, ceilings and the business starter limits equal the table exactly | `TestOfferLimits_SeededTableMatchesSpec` | Not Run |
| TC-USD-032 | AC-9 | Happy Path | P0 | Zero wallets differ; the demo logins, KYC images, open fraud flag, pending disbursement and failed instance exist | `seed assertLedger + SQL` | Not Run |
| TC-USD-033 | AC-10 | Edge Case | P0 | Zero wallets differ from their ledger sum and the money is conserved | `TestP2PLoad` | Not Run |
| TC-USD-034 | AC-11 | Happy Path | P1 | The sync script passes and the text says amounts are USD cents | `check-openapi-sync` | Not Run |
| TC-USD-035 | AC-11 | Happy Path | P1 | Every amount reads in dollars and the checker passes | `guide checker` | Not Run |

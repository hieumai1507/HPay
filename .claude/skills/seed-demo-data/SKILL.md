---
name: seed-demo-data
description: Generate or refresh demo seed data for hpay-backend. Trigger on "prepare demo", "seed data", "make the demo look real", or before recording a demo video / interview walkthrough.
---

# seed-demo-data — Realistic demo state in one command

Implemented as `cmd/seed/main.go` (idempotent — safe to re-run; it upserts by fixed UUIDs). Extend it, don't write ad-hoc SQL scripts.

## Required demo state

1. **System wallets**: `SYSTEM_CASH_IN`, `SYSTEM_FEE` (created by migration, verify present).
2. **Users** (fixed phone numbers, password `Demo@123`, OTP printed to console in dev):
   - `0900000001` Hiếu Mai — KYC verified, balance 12,450,000₫, has vaults ("Du lịch Đà Lạt" 70% of 6M goal with round-up ON), history ≥ 10 mixed transactions across several days.
   - `0900000002` Minh Anh — verified, counterpart for P2P + one incoming money request pending.
   - `0900000003` — KYC `manual_review` (ai_score 64, reasons filled) sitting in the admin queue.
   - `0900000004` — account `locked` with a `confirmed_fraud` flag + reversal approval request pending in My Tasks.
3. **Organization** "DEH Corp": owner + 1 maker + checkers L1/L2 (`0900000011..14`), policy "Chi lớn" v3 (L1 ≥10M: 1 sig; L2 ≥40M: +1 sig), org wallet 842,500,000₫, and:
   - one disbursement `pending_approval` at 1/2 signatures (workflow instance parked at WAIT_APPROVAL),
   - one completed disbursement with full step trail.
4. **Workflow variety** (for the admin monitor): 1 instance `failed` at CREDIT (receiver locked → E07 material), 1 `compensated`, 1 `processing_async` awaiting recovery, plus healthy completed ones.
5. **Fraud flags**: 1 open `VELOCITY_5MIN` (score 87, reasons incl. AI explanation text), 1 cleared.
6. **Recon**: yesterday's `bank_statements` with exactly 3 `missing_external` rows in the 23:58–00:02 window (cutoff-story for the AI summary) + matching rows for the rest; a completed `reconciliation_run`.
7. **Money requests**: 1 pending incoming for user 1, 1 split group (3 payers: 2 paid 1 pending) — feeds the Split tracking screen.

## Rules

- All timestamps relative to `now()` (yesterday/today), so the demo never looks stale.
- Amounts must reconcile: seeded balances = SUM of seeded ledger entries per wallet. The seeder asserts this at the end and fails loudly if not.
- Print a summary table (logins, org, what to click first) when done.

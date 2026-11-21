[← F4](../f4-workflow-engine/implementation-plan.md) · f5-p2p-fee · [solution-architecture →](solution-architecture.md)

# F5 P2P Transfer & Fee — Implementation Plan

## 1. Scope & important design change

> **Deliberate deviation from docs 00 §4:** merge DEBIT + CREDIT into ONE `MOVE_FUNDS` step. Reason: `ledger.Post` is already atomic across multiple legs in 1 DB tx — splitting into 2 steps is artificial and creates a dangling-money state that then has to be compensated. Merged, "CREDIT fails after DEBIT is done" **cannot happen by construction** (stronger evidence than any test). Compensation is still really implemented for the case where a step AFTER the money has moved fails terminally (the E07 test goes through this path). We will propose updating docs 00 §4 after the merge.

| Ships | Not included |
| --- | --- |
| Definition `p2p_transfer` v1 (VALIDATE → FRAUD_CHECK → MOVE_FUNDS → CONFIRM); 4 handlers registered into the worker; fee engine (F1 fee_policies table: fixed + bp + min/max, picks the policy for the exact tier first, falls back to NULL tier); REAL engine compensation (walk backwards over compensatable steps, queue in context, wallet handler reverses the entries from the txn row); API: transfer, fee-preview, timeline; E07/E12 tests + E2E; openapi.yaml updated in the same commit | Real fraud (F9 — F5 uses an always-safe stub); notification; daily limit; E15 (no async bank callback yet) |

## 2. Task breakdown

| # | Task | Size |
| --- | --- | --- |
| T1 | Migration 000012: seed definition p2p_transfer v1 (is_active) + 2 fee policies (p2p 0.1% max 10k; tier verified/auto_approved: free) | S |
| T2 | Queries: fee.sql (GetFeePolicy prefers tier), GetUserStatusByWalletID (E12), GetInstanceByTransactionID + GetTransactionForStep | M |
| T3 | `internal/fee`: pure Calc (clamp min/max) + Lookup; unit tests | S |
| T4 | Engine compensation upgrade: StartEvent.Compensate, `_compensate` queue in context, OnResult guard accepts status compensating, attempt number 101+ to avoid hitting the unique constraint | L |
| T5 | Handlers `internal/workflow/handlers`: validator (user status + limit + soft balance), fraud stub, wallet (MOVE_FUNDS atomic: re-check user status E12, Post 3 legs including fee; compensate: reverse from the txn row), confirmer (guarded CompleteTransaction); registered in main.go | L |
| T6 | `internal/transfer`: Transfer() — resolve receiver by phone, fee calc, insert txn (idempotency like topup) + engine.Start in the SAME DB tx; API handlers + routes + errmap + openapi | M |
| T7 | Tests: E07 (compensation reverses money when a step after MOVE_FUNDS fails terminally), E12 (user locked mid-workflow → fail, money does not move), E2E P2P with full fee + timeline; CI adds E07 E12 | L |

## 3. API changes (into openapi.yaml in the same commit)

| Endpoint | Description | Errors |
| --- | --- | --- |
| `POST /v1/transactions/transfer` | {receiver_phone, amount, note} + Idempotency-Key → 202 {ref_code, status, fee}; duplicate key → 200 old transaction | ERR_RECEIVER_NOT_FOUND, ERR_SELF_TRANSFER, ERR_AMOUNT_INVALID, ERR_LIMIT_EXCEEDED, ERR_INSUFFICIENT_BALANCE (early soft-check) |
| `GET /v1/transactions/fee-preview?type=p2p&amount=` | {fee, total} for my current tier — mobile shows it before confirm | ERR_VALIDATION |
| `GET /v1/transactions/{refCode}/timeline` | Public view of step_executions (friendly step names, status, time) — Processing/Detail screen | ERR_TXN_NOT_FOUND |

## 4. Test plan

| ID | Test |
| --- | --- |
| E07 | `TestEdge_E07_CompensationReversesMovedFunds`: a test definition with a boom step (on_failure=compensate) AFTER MOVE_FUNDS → instance compensated, txn failed, both wallet balances back to the original state, ledger net = 0 (6 lines), SYSTEM_FEE also refunded |
| E12 | `TestEdge_E12_LockedUserFailsMidWorkflow`: lock the receiver after Start, before MOVE_FUNDS runs → txn failed, NO ledger entries at all |
| — | E2E: transfer 1,000,000 VND tier none → fee 1,000 VND, sender −1,001,000, receiver +1,000,000, SYSTEM_FEE +1,000, instance completed, timeline 4 steps; idempotent retry returns the old txn; fee unit tests clamp min/max + tier fallback |

## 5. Rollout & risk

- Compensation uses attempt ≥ 101 by convention (avoids hitting unique(instance,step,attempt) of the forward attempts ≤ retry) — noted in a code comment.

- Handlers are registered in main → the worker role must run for P2P to complete; dev uses --role=all as before.

- Docs 00 §4 will drift (DEBIT/CREDIT → MOVE_FUNDS): create a follow-up to fix the docs after the merge.

## 6. Checklist

- [x] plan · [ ] T1→T7 · [ ] fintech-review · [ ] commit `feat: F5 p2p transfer via engine - fee engine, real compensation, timeline API`

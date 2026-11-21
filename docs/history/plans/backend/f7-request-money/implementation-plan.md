[← F6](../f6-approval-engine/implementation-plan.md) · f7-request-money · [solution-architecture →](solution-architecture.md)

# F7 Request Money & Split Bill — Implementation Plan

## 1. Scope & non-goals

> **Same advantage as F6:** the `money_requests` schema (enum, columns, indexes) was already seeded by migration 000008 (F1) — no schema change needed. Stage 2 (payer pays) fully reuses the `p2p_transfer` workflow + `fee.Service` that exist since F5 — Request Money is not a new money flow, just a UX/state layer in front of P2P.

| Ships | Not done in F7 |
| --- | --- |
| Create a single request (1 payer, TTL 72h); Split Bill (N payers sharing one `split_group_id`, even split with handling of the leftover-VND rounding — or custom amounts entered by the requester, the sum must match); Pay flow: lock the `money_requests` row (FOR UPDATE) → create the P2P transaction + `engine.Start` in the SAME DB tx → attach `paid_txn_id` — E19 is shut down by the transaction boundary, not just by checking a flag; Decline (payer) / Cancel (requester), both guarded by status='pending'; Expiry sweeper (same pattern as the F6 approval sweeper); API: create, create-split, inbox (I am the payer), sent (I am the requester), detail (including the status of requests sharing the same split_group_id), decline, cancel, pay; openapi.yaml in the same commit; E19 test + full-flow E2E + split rounding unit test | Real push notifications (no notify infrastructure yet — the inbox is a polling API, following the F5/F6 precedent of leaving "notification" for a later phase); QR with amount / payment link (P2, doc 04 §2.8); custom split does NOT check for duplicate payers (out of scope, left to the UI to prevent) |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Queries: money_request.sql (Insert, GetForUpdate, ListInboxPending by payer, ListSent by requester, ListBySplitGroup, FinalizeGuarded(status,paid_txn_id) WHERE status='pending', DeclineGuarded, CancelGuarded, ListDueExpiry claim-lease in the E10 style) | internal/store/queries/money_request.sql | M | — |
| T2 | `internal/moneyrequest`: split.go (`SplitEven(total, n) []int64` — largest-remainder, the sum always matches total, no float) + unit tests | internal/moneyrequest/split.go | S | — |
| T3 | `internal/moneyrequest` service: Create (1 payer), CreateSplit (N payers, even or custom amounts with sum validation), Pay (tx: FOR UPDATE request → guard status=pending & expires_at>now() [this is what blocks E19] → resolve both wallets → fee.Service (using the p2p policy) → InsertTransaction + engine.Start in the SAME tx → FinalizeGuarded paid_txn_id — REUSES the idempotency pattern of transfer.Service but does not call Transfer() directly because it must share one DB tx with the request claim), Decline, Cancel, Inbox, Sent, Detail (with siblings if split) | internal/moneyrequest/service.go | L | T1,T2 |
| T4 | `internal/moneyrequest` sweeper: ExpireDue (like approval.Sweeper — claim-lease, guarded UPDATE, batch) | internal/moneyrequest/sweeper.go | S | T3 |
| T5 | API: request_handler.go (POST /money-requests, POST /money-requests/split, GET /money-requests/inbox, GET /money-requests/sent, GET /money-requests/:id, POST /money-requests/:id/pay [Idempotency-Key], POST /money-requests/:id/decline, POST /money-requests/:id/cancel); routes into the `me` group (public API, not /org); errmap; openapi.yaml; wire the sweeper into main.go worker role | internal/api/request_handler.go, router.go, errmap.go, cmd/server/main.go | L | T3,T4 |
| T6 | Tests: E19 (pay exactly at expiry — 2 scenarios: sweeper wins first / payer wins first, both consistent), single-request E2E (create→pay→P2P completed→request paid), Split Bill (3 payers, 2 pay 1 declines, aggregated group status), guarded Cancel/Decline (wrong actor not allowed), rounding unit test (100_000 / 3 = 33334+33333+33333); CI adds E19 | test/integration/moneyrequest_test.go, .github/workflows/ci.yml | L | T5 |

## 3. Schema changes

No new migration — `money_requests` already has enough columns from 000008. If a missing one is discovered while coding (for example a column marking who claimed the expiry) → stop and propose a docs fix before adding a migration (docs-first).

## 4. API changes (into openapi.yaml in the same commit)

| Endpoint | Description | Errors |
| --- | --- | --- |
| `POST /v1/money-requests` | {payer_phone, amount, note} → 1 request, expires 72h | ERR_AMOUNT_INVALID, ERR_SELF_REQUEST, ERR_RECEIVER_NOT_FOUND (payer does not exist) |
| `POST /v1/money-requests/split` | {payer_phones[], total_amount, note, amounts?[]} → N requests sharing one split_group_id; without `amounts` → split evenly (largest-remainder); with it → the SUM must equal total_amount | ERR_VALIDATION (amounts mismatch / empty), ERR_RECEIVER_NOT_FOUND |
| `GET /v1/money-requests/inbox?status=pending` | Requests where I am the payer | — |
| `GET /v1/money-requests/sent` | Requests where I am the requester | — |
| `GET /v1/money-requests/{id}` | Detail + list of siblings if split_group_id is set (only the requester or the payer of that specific request may view it) | ERR_REQUEST_NOT_FOUND |
| `POST /v1/money-requests/{id}/pay` | Payer pays — 202 {ref_code, txn_status} + request→paid; Idempotency-Key required (used for the underlying P2P transaction) | ERR_REQUEST_NOT_PAYABLE (already paid/declined/canceled), ERR_REQUEST_EXPIRED (E19), ERR_INSUFFICIENT_BALANCE, ERR_FORBIDDEN (not the payer) |
| `POST /v1/money-requests/{id}/decline` | Payer declines | ERR_REQUEST_NOT_PAYABLE, ERR_FORBIDDEN |
| `POST /v1/money-requests/{id}/cancel` | Requester cancels | ERR_REQUEST_NOT_PAYABLE, ERR_FORBIDDEN |

## 5. Test plan

| ID | Test |
| --- | --- |
| E19 | `TestEdge_E19_ExpiryRacePay`: (a) set `expires_at` to the past RIGHT BEFORE calling Pay() (sweeper not yet run) → Pay must block itself with the `expires_at > now()` condition inside the tx, return ERR_REQUEST_EXPIRED, and create NO transaction; (b) run 2 concurrent goroutines: 1 calls Pay(), 1 calls sweeper.ExpireDue() right as it expires → FOR UPDATE serializes them, exactly 1 of the 2 wins (paid OR expired), never both, and there is no "orphan" transaction if expired wins |
| — | Single-request E2E: A requests 200k from B → B pays → P2P completes (via the F5 handlers) → request status=paid + correct paid_txn_id; Split Bill with 3 people, 300k split evenly → 2 pay, 1 declines → GET detail shows 3 independent statuses; Cancel by the requester while still pending; Decline/Cancel by the wrong actor → 403; rounding: SplitEven(100000,3) = [33334,33333,33333], SplitEven(100000,4)=[25000,25000,25000,25000], the sum always equals the input |

## 6. Rollout & risk

- **Pay() does not call `transfer.Service.Transfer()` directly** — it deliberately duplicates the "create transaction + engine.Start" part because it must share one DB tx with locking/finalizing the money_request (the same atomicity principle used in the F6 pause hook). Risk: two places hold the same P2P-transaction-creation logic (transfer.Service and moneyrequest.Service) — a small duplication is accepted to keep atomicity correct, and is clearly noted in a code comment so the two do not drift when F5 changes its logic later.

- Split Bill custom amounts: validate the sum at the service layer BEFORE inserting any row (fail-fast, no half-way rollback).

- The new sweeper runs in parallel with approval.Sweeper — no contention (different tables), same 30s interval convention.

- No change to the existing P2P flow — payer pay = a normal P2P transfer with receiver=requester, so all the old F5 tests are unaffected.

## 7. Execution checklist

- [x] gen-feature-plan · [ ] T1→T6 (gen-migration if needed, gen-api-test for E19) · [ ] fintech-review · [ ] commit `feat: F7 request money - split bill, pay via p2p engine, expiry race guard`

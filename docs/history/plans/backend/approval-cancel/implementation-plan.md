[← HPay Docs](../../../overview-v2.md) · repo hpay-backend · plan

# Implementation Plan — Maker Cancels a Pending Approval

> **Feature slug:** `approval-cancel` · Source: mockup [03-business.html frame 19](../../../../design/mockups/README.md) — the "Cancel order" button already has UI on the `order-status.tsx` screen (hpay-mobile) but is only a placeholder Alert, with no API behind it. Accompanies the mobile feature `approval-result-screens` (frames 22/23) — which does not depend on the backend and only consumes existing data.

> **Update after implementation:** the original down migration deliberately failed unconditionally (irreversible-by-design) — but this violated the CI gate `E18: migrate up→down→up`, which runs on an empty DB, where recreating the enum type is completely safe (no row is in the `cancelled` state). Fixed: the down migration is now **conditional** — it only `RAISE EXCEPTION`s if a `cancelled` row actually exists (protecting real data); otherwise (empty DB/CI) it recreates the type via rename→create→alter→drop and runs cleanly. Verified manually: it correctly blocks when 2 real cancelled rows exist, and runs cleanly after they are cleaned up — see `db/migrations/000011_approval_cancelled_status.down.sql`.

## 1. Current state (investigated)

| Component | Reality | Conclusion |
| --- | --- | --- |
| `approval_status` enum | `pending \| approved \| rejected \| expired` — no state distinguishes "maker cancelled it themselves" from "checker rejected" | ❌ need to add the value `cancelled` so the two different meanings are not conflated in later reports/audit |
| `Service.finalize()` | Accepts any `store.ApprovalStatus` + `approved bool` + `reason string`, runs the registered executor (org_transaction → `resumeWorkflow`, marks the step failed + resumes the transaction to failed) — the logic is already general, no change needed | ✅ reuse as is, just call it with `status=cancelled, approved=false` |
| Route/handler for cancel | None | ❌ add new |

## 2. Scope & Non-goals

### Ships

- Migration adding the enum value `cancelled` to `approval_status`.

- `Service.Cancel(ctx, makerID, requestID) error` — only `created_by` (the maker) can cancel, and only when `status=pending`; reuses `finalize()` as is.

- `POST /v1/org/approvals/{id}/cancel`.

### Non-goals (defer)

- A checker/admin cancelling on behalf of someone else's order — only the maker cancels their own order, per the mockup.

- Fee/money refund — `CreateDisbursement` does not debit money before approval (keeping the old reject behavior), so there is nothing to refund.

## 3. Task breakdown

| # | Task | Files | Size |
| --- | --- | --- | --- |
| T1 | Migration `000011_approval_cancelled_status`: `ALTER TYPE approval_status ADD VALUE 'cancelled'` — 1 separate file, with no other DDL (it cannot run in the same transaction as other statements); the down file leaves a large comment + fails deliberately (a used enum value cannot be reverted without recreating the type) | `db/migrations/000011_*.up.sql`/`.down.sql` | S |
| T2 | `Service.Cancel`: load the request, check `viewerID == CreatedBy` (do not use `eligibility()` because that is the rule for checkers), check `status==pending`, call `finalize(..., cancelled, false, "cancelled by maker")` | `internal/approval/service.go` | M |
| T3 | Handler + route `POST /v1/org/approvals/:id/cancel` | `internal/api/approval_handler.go`, `internal/api/router.go` | S |
| T4 | openapi.yaml + `ERR_APPROVAL_NOT_CANCELLABLE` when status is not pending, or when the caller is not the maker → `ERR_APPROVAL_NOT_FOUND` (do not reveal existence) | `api/openapi.yaml`, `internal/api/errmap.go` | S |
| T5 | Tests: the maker can cancel their own pending order → txn failed, no ledger entry; a checker/other person cancelling on behalf is blocked with 404; cancelling an already approved/rejected/expired order is blocked | `test/integration/approval_cancel_test.go` (new) | L |

## 4. Schema changes

1 migration: add the value `cancelled` to the enum `approval_status` (see T1).

## 5. API changes

```text
POST /v1/org/approvals/{id}/cancel
→ 200 {status: "cancelled"}
→ 404 ERR_APPROVAL_NOT_FOUND — not the maker of this order, or the order does not exist
→ 409 ERR_ALREADY_FINALIZED — the order is no longer in the pending state
```

## 6. Test plan

| Case | Test |
| --- | --- |
| Maker cancels a pending order | `TestApprovalCancel_MakerCancelsPending` — txn moves to `failed`, no ledger entry, step_execution records failed just like the existing reject branch |
| Checker cannot cancel the maker's order | `TestApprovalCancel_CheckerCannotCancel` — 404, existence not revealed |
| Someone outside the org cannot cancel | `TestApprovalCancel_OutsiderGetsNotFound` |
| A completed order cannot be cancelled | `TestApprovalCancel_AlreadyFinalizedRejected` — approved/rejected/expired are all blocked with `ErrAlreadyFinalized` |

## 7. Rollout & risk

Low risk — adds 1 enum value (forward-only, safe because Postgres `ADD VALUE` is append-only) and 1 new action that reuses the existing `finalize()` as is, which has test coverage (reject/expire take the same path). Does not change the behavior of existing flows.

## 8. Execution checklist

- `gen-migration` → T1, verify `make migrate-up` (the down deliberately fails; do not run a real migrate-down on a DB with data)

- Manual → T2–T4

- `gen-api-test`-style → T5

- `fintech-review` before committing

---
HPay · plan generated by the gen-feature-plan skill · repo hpay-backend · 2026-07-24

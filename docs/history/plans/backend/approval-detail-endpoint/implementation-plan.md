[← HPay Docs](../../../overview-v2.md) · repo hpay-backend · plan

# Implementation Plan — Approval Detail Endpoint

> **Feature slug:** `approval-detail-endpoint` · An extension of F6 (maker-checker). `GET /v1/org/approvals` (Inbox), `challenge` and `decide` already exist, but there is no endpoint to view the details of a single request together with its signature history — both the maker's "Order status" screen (mockup frame 19) and the "SIGNATURES" section of the checker screen (frame 21) need this data, and no API returns it yet.

> **Update after implementation:** it was discovered that `CreateDisbursement` returns only the `ref_code` immediately — the `approval_requests` row is only created afterwards by the pause hook when the engine reaches `WAIT_APPROVAL` (asynchronously via outbox/worker). The maker's client therefore has no request id to call `GET /v1/org/approvals/{id}` right after creating the order. Added `GET /v1/org/approvals/by-ref/{ref_code}` + `Service.DetailByRefCode` (same permission logic + shape as `Detail`, differing only in the lookup key) to solve this — see T1b/T2b below.

## 1. Current state (investigated)

| Component | Reality | Conclusion |
| --- | --- | --- |
| `internal/api/approval_handler.go` | Only has `Inbox` / `Challenge` / `Decide` — no handler to view a single request by id | ❌ must be added |
| `internal/store/queries/approval.sql: GetApprovalRequest` | Exists but returns only raw fields (no join of org/txn/maker), and is not used by any API | ⚠️ need a separate join query for the detail DTO |
| `ListApprovalActions` | Returns `checker_id, level, action, acted_at` — indeed the signature history needed for the timeline, but lacks the checker name and is not exposed through any API | ⚠️ need a version that additionally joins `users` + 1 new handler |
| hpay-mobile `approval-detail.tsx` (checker) | Already has a detail screen for the checker, but gets its data by client-side filtering the Inbox list — no "SIGNATURES" section, and unusable for the maker | ❌ must be changed to call the real detail API |
| "Order status" screen for the maker (mockup frame 19) | No route yet in `app/(main)/org/` | ❌ screen not built at all |

## 2. Scope & Non-goals

### Ships

- Backend: `GET /v1/org/approvals/{id}` — full status + the list of signatures (checker name, level, action, timestamp), serving both the maker and the checker for review.

- Access: the maker (creator) OR any member of that org may view; an unrelated person receives 404 (existence not revealed, following the principle `eligibility()` already uses for Decide).

### Non-goals (defer)

- Detail for admin approvals (reversal / account_lock / policy_change) — the admin web already has its own path via `ListPendingAdminApprovals`, outside the scope of this endpoint.

- A "my orders list" endpoint for the maker — the mobile screen receives the request id directly from the transaction/disbursement creation flow, so no new index is needed.

## 3. Task breakdown

| # | Task | Files | Size |
| --- | --- | --- | --- |
| T1 | Add 2 queries: `GetApprovalRequestDetail` (joins organizations/transactions/users to get org_name, ref_code, amount, fee, maker_name) and `ListApprovalActionsWithNames` (joins users to get checker_name); `sqlc generate` | `internal/store/queries/approval.sql` | S |
| T2 | `Service.Detail(ctx, viewerID, requestID)` — load the request, check permission (created_by or `GetOrgMember`), load signatures, decode policy_snapshot to compute `signatures_required` | `internal/approval/service.go` | M |
| T3 | Handler + route `GET /v1/org/approvals/:id` | `internal/api/approval_handler.go`, `internal/api/router.go` | S |
| T4 | Update `openapi.yaml` (add the path, short description following the convention of the existing approvals endpoints) | `api/openapi.yaml` | S |
| T5 | Tests: the maker can view, a checker (org member) can view, a stranger gets 404, signature order is correct, `signatures_required` comes from the frozen snapshot | `test/integration/approval_detail_test.go` (new) | L |
| T1b | *(added during work)* Add query `GetApprovalRequestDetailByRefCode` (same join, different `WHERE t.ref_code=$1`) | `internal/store/queries/approval.sql` | S |
| T2b | *(added during work)* Refactor the shared DTO-building part into `buildDetail()`; add `Service.DetailByRefCode` + handler `DetailByRefCode` + route `GET /v1/org/approvals/by-ref/:ref_code` | `internal/approval/service.go`, `internal/api/approval_handler.go`, `internal/api/router.go` | M |

## 4. Schema changes

No migration — `approval_requests`/`approval_actions`/`organizations`/`transactions`/`users` already have enough columns, only new queries are added.

## 5. API changes

```text
GET /v1/org/approvals/{id}
→ 200 { id, status, current_level, org_id, org_name, ref_code, amount, fee,
        maker_id, maker_name, created_at, expires_at, finalized_at,
        policy_name, signatures_required,
        signatures: [{checker_id, checker_name, level, action, acted_at}] }
→ 404 if it does not exist or the caller has no permission to view (existence not revealed)
```

No new `ERR_*` is needed — reuses `approval.ErrNotFound`, already mapped in `errmap.go` (same as the current Challenge/Decide).

## 6. Test plan

| Case | Test |
| --- | --- |
| The maker can view their own request | `TestApprovalDetail_MakerCanView` — even when the maker is not any checker/org member |
| A checker (org member) can view | `TestApprovalDetail_EligibleCheckerCanView` — viewing is not blocked even if the level does not match current_level (viewing ≠ approving) |
| A stranger is blocked | `TestApprovalDetail_UnrelatedUserGetsNotFound` — 404, not 403 |
| Signature history in the correct order | `TestApprovalDetail_SignatureHistoryOrdered` — 2 checkers sign at 2 levels, the response follows `acted_at` order |
| `signatures_required` from the snapshot | `TestApprovalDetail_SignaturesRequiredReflectsPolicySnapshot` — does not change when the policy is edited later (E09) |

## 7. Rollout & risk

Low risk — a new, read-only endpoint that does not touch the existing Inbox/Challenge/Decide. No feature flag needed, no backfill. Can be merged/deployed independently of the mobile client.

## 8. Execution checklist

- Write the 2 new queries, `sqlc generate` → T1

- `impact()` on `Service` before adding the method → T2

- Handler + route → T3

- Update `openapi.yaml`, run `scripts/check-openapi-sync.py` → T4

- `gen-api-test`-style → T5

- `fintech-review` before committing

- `detect_changes()` before committing

---
HPay · plan generated by the gen-feature-plan skill · repo hpay-backend · 2026-07-24

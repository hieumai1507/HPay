[← HPay Docs](../../../README.md) · repo `hpay-mobile` + `hpay-backend` · plan

# Implementation Plan — Maker Status & Checker Signatures

**Feature slug:** `maker-checker-detail-screens` · **Source:** [mockup 03-business.html frame 19 & 21](../../../design/mockups/03-business.html) + [ui-design-spec §8](../../../design/ui-design-spec.html). Accompanying backend: [docs/plans/backend/approval-detail-endpoint/](../../backend/approval-detail-endpoint/implementation-plan.md) (merged first, provides `GET /v1/org/approvals/{id}`).

## 1. Current state surveyed

| Component | Reality | Conclusion |
|---|---|---|
| "Order status" screen for the maker (frame 19) | There is no route in `app/(main)/org/`; after creating a disbursement, the maker has nowhere to track signature progress | ❌ build entirely new |
| `app/(main)/org/approval-detail.tsx` (checker) | Already exists, shows amount/maker/fee/level/expiry + approve/reject buttons + OTP modal — but has NO "SIGNATURES" section (frame 21) and does not show the history of who has signed | ⚠️ extend, do not rewrite |
| `useApprovalDetail(id)` in `useOrgQueries.ts` | Fake — calls `GET /v1/org/approvals` (the entire inbox) and then `.find(id)` on the client; breaks as soon as the item leaves the inbox (approved/rejected) or when the maker calls it (the maker is not in their own inbox) | ❌ must switch to calling `GET /v1/org/approvals/{id}` directly (new endpoint) |
| `ApprovalItem` type | Hand-typed to the old Inbox shape, missing `status`, `signatures`, `signatures_required`, `maker_id` | ❌ add a new `ApprovalDetail` type matching the backend DTO |

## 2. Scope & Non-goals

### Ships

- `useApprovalDetail(id)` actually calls `GET /v1/org/approvals/{id}`, returning the full `ApprovalDetail` type (including `signatures[]`, `signatures_required`, `status`).
- New screen `app/(main)/org/order-status.tsx` (frame 19): amount hero + status chip "Pending approval · x/y signatures", a vertical timeline per level (signed/waiting/not yet their turn), and a "Back to home" button.
- Add a "SIGNATURES" block to `approval-detail.tsx` (frame 21): a list of each checker who has signed with level + time, sharing the same `signatures[]` data.
- After a disbursement is created successfully, `disburse.tsx` navigates the maker to `order-status` instead of only showing an Alert.

### Non-goals (defer)

- A dedicated OTP-success screen (frame 22) and the conflict/race screen "order already processed" (frame 23) — they still use `Alert.alert` as before; upgrading them to a full-screen result is a separate task, not part of this request.
- A "my orders" list for the maker — `order-status` takes the `id` directly via a navigation param from `disburse.tsx`, no listing route needed.
- Push notification when a new signature arrives — the screen uses pull-to-refresh + refetch on a short interval, no background polling.

## 3. Task breakdown

| # | Task | Files | Size |
|---|---|---|---|
| T1 | New `ApprovalDetail` type + switch `useApprovalDetail(id)` to call the real `GET /v1/org/approvals/{id}` (remove the client-side filter hack) | `src/queries/useOrgQueries.ts` | S |
| T2 | Add the "SIGNATURES" block (list of checkers who signed + level + time) to the existing checker detail screen | `app/(main)/org/approval-detail.tsx` | M |
| T3 | New "Order status" screen for the maker: hero + chip + vertical per-level timeline | `app/(main)/org/order-status.tsx` (new) | L |
| T4 | After creating a disbursement, navigate to `order-status` with the id just created | `app/(main)/org/disburse.tsx` | S |
| T5 | Tests: signature progress format, timeline state by `current_level`/`status`, correct display when rejected/expired | `app/(main)/org/__tests__/order-status.test.tsx` (new) | M |

## 4. Schema changes

None — uses the backend DTO already available from `approval-detail-endpoint`.

## 5. API changes

No backend change — only consumes the existing `GET /v1/org/approvals/{id}`.

## 6. Test plan

| Case | Test |
|---|---|
| Timeline: a passed step shows "approved" | signature with level < current_level → dot "done" |
| Timeline: the current step shows "waiting" | level == current_level and status=pending → dot "active" |
| Timeline: a later step not yet their turn | level > current_level → dot "wait", dimmed |
| Approved status | chip "Approved", no "wait" step remains |
| Rejected/expired status | corresponding red chip, timeline stops at the rejected/expired step |

## 7. Rollout & risk

Low risk — 2 new/extended UI screens, no change to the existing money or OTP flow (approve/reject still goes through `useDecideApproval` as before). Hard dependency on the already-merged backend endpoint; if the backend is not yet deployed, the screen will only show a load error — the app does not crash.

## 8. Execution checklist

1. Manual (change the hook) → T1
2. `gen-screen`-style manual work for `order-status.tsx` → T3, T4
3. Add the UI in the existing file → T2
4. Tests + `npm run typecheck` + `npm run lint` → T5

HPay · plan generated by the `gen-feature-plan` skill · repo hpay-mobile + hpay-backend · 2026-07-24

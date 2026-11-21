[← HPay Docs](../../../../docs/index.html) · repo `hpay-admin` · plan

# Implementation Plan — My Tasks & Server-side Pagination

**Feature slug:** `a4-my-tasks-server-pagination` · **Phase:** B (post-audit) · **Source:** [doc 03 (A02, A04–A07)](../../../../docs/architecture/03-admin-solution-architecture.html) + [doc 07](../../../../docs/architecture/07-tanstack-table-query-pagination.html). Closes gap **G5** (missing My Tasks page) and **G6/A02** (pagination).

## 1. Current state as surveyed (not assumptions)

| Layer | Reality in the code | Consequence |
|---|---|---|
| Backend list APIs | **Already done**: `CommonQueryParams` (page/limit/sort/order/from/to/status/tab) + `Normalize()` + `Offset()`; 8 endpoints return `{items, total, page, limit, total_pages}` with real LIMIT/OFFSET | The server part of A02 **does not need to be redone** |
| Backend approvals | `GET /v1/admin/approvals` + `POST /v1/admin/approvals/:id/decide` (`action: approve\|reject`, `reason` ≤255) already exist | My Tasks only lacks the UI |
| FE `useTableQueryState` | Already produces `queryParams` (page 1-based, limit, search, sort, order) + auto-reset page when filters change | Infrastructure exists, **no page uses it yet** |
| FE tables | Pages only declare `getCoreRowModel()` — **no** `getPaginationRowModel`, **no** `manualPagination`/`pageCount` | ⚠️ Pagination **does not work**: the buttons are displayed but the table always renders all rows |
| FE hooks | Accept `params` and call `buildQueryString`, but **return a flat array**, discarding `total` | No `totalCount` to build `pageCount` |

**Correction relative to the first audit pass:** the initial report described admin as "client-side pagination". The reality is worse — there is no pagination model at all, so the pagination controls are **inert**. Recorded here so the plan reflects the truth.

## 2. Scope & Non-goals

### Ships

- **G5 — My Tasks / Approval Inbox:** route `/tasks`, a table of approvals awaiting review, detail view, approve/reject with a reason, RBAC via `can()`, handling of 409 `ERR_ALREADY_FINALIZED` (A01).
- **G6/A02 — Real server-side pagination:** hooks return `{rows, total}`; pages pass `queryParams`; tables enable `manualPagination: true` + `pageCount`; `DataTable` receives `totalCount`.

### Non-goals (defer)

- Migrating React Router → TanStack Router (G8) — does not block functionality.
- Async xlsx export polling (A06) and Orgs CRUD (G9) — separate plans.
- Removing the demo-data fallback in hooks — recorded as technical debt, handled separately.

## 3. Task breakdown

| # | Task | Files | Size | Depends |
|---|---|---|---|---|
| T1 | Standardize the pagination return type: `Paged<T> = {rows: T[]; total: number}` in `queries/common.ts` | `src/queries/common.ts` | S | — |
| T2 | Hook returns `Paged<T>` and keeps `params` → query key unchanged (audit first, as the template) | `src/queries/useAuditQueries.ts` | S | T1 |
| T3 | Create hook `useApprovalQueries` (list) + `useDecideApproval` (mutation, invalidates the list) | `src/queries/useApprovalQueries.ts` | M | T1 |
| T4 | **G5** My Tasks page + route `/tasks` + sidebar link; RBAC, mandatory reason, 409 → toast + refetch | `src/pages/MyTasksPage.tsx`, `src/router.tsx`, sidebar | L | T3 |
| T5 | **G6** Enable server-side for tables: `manualPagination`, `pageCount`, `totalCount`; pages pass `queryParams` into the hook | `src/pages/AuditPage.tsx` (+ the remaining queue pages) | L | T2, T4 |
| T6 | Verify in the browser: changing page calls the new API, no console errors, take screenshots | — | M | T5 |

## 4. API changes

**No backend change.** Only consumes the existing contract exactly:

```
GET /v1/admin/audit?page=1&limit=10&sort=created_at&order=desc
  → { logs: [...], total, page, limit, total_pages }

GET  /v1/admin/approvals?status=pending&page=1&limit=10
  → { approvals: [...], total, page, limit, total_pages }
POST /v1/admin/approvals/:id/decide   { action: "approve"|"reject", reason }
  → 409 ERR_ALREADY_FINALIZED when someone else finalized first (A01)
```

## 5. Test plan

| Verification | How | Code |
|---|---|---|
| Changing page calls the API with the new `page` | Browser: network request when clicking Next | A02 |
| Number of rendered rows = `limit`, not all of them | Browser: count rows in the DOM | A02 |
| Changing a filter resets to page 1 | Already present in `useTableQueryState` | A02 |
| My Tasks shows pending approvals | Browser + read_page | A04 |
| Approve when the record is already finalized → 409 handled gracefully | Toast + refetch, no crash | A01 |
| A role without permission does not see the decide buttons | `can()` guard | A05 |

## 6. Rollout & risk

- **Main risk:** changing the hooks' return type is breaking for every page that uses them. Mitigation: change **one hook at a time** (audit first as the template), run `tsc` after each step so the compiler points out the call sites.
- **Backend not committed:** the backend's pagination DTO part is in the working tree uncommitted (not this plan's job). The FE depends on it — if the backend reverts, pages will receive `total` undefined; hooks must fall back to `total = rows.length` so nothing breaks.
- **No backfill/migration** — pure frontend, rollback = revert the commit.

## 7. Execution checklist

1. `impact()` on `useTableQueryState` and `useAuditQueries` before editing (mandatory per CLAUDE.md).
1. T1 → T2 → `npx tsc --noEmit`.
1. T3 → T4 (skill **gen-admin-page** / **gen-feature-hook**).
1. T5 → `tsc` + `npm run build`.
1. T6 browser verification, take screenshots.
1. `detect_changes()` before committing; update this plan if the code deviates.

HPay · plan generated by the `gen-feature-plan` skill · repo hpay-admin · 2026-07-22

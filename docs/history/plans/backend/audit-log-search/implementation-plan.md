[← HPay Docs](../../../overview-v2.md) · repo `hpay-backend` + `hpay-admin` · plan

# Implementation Plan — Audit Log Server-Side Search

> **Feature slug:** `audit-log-search` · The missing part of "real server-side pagination, search and date filters for admin lists" (done for most admin lists, the audit log was overlooked). The frontend (`AuditPage.tsx`) already notes this explicitly: *"search is intentionally NOT sent — /v1/admin/audit has no text filter"*.

## 1. Current state (investigated)

| Component | Reality | Conclusion |
| --- | --- | --- |
| `internal/listing.Filter` | Already has `Name/Order/From/To` + helpers `Text()/FromTS()/ToTS()/SortOrder()`, shared by fraud flags, workflow instances, etc. | ✅ reuse, no new field needed |
| `internal/store/queries/audit.sql` | `ListAuditLogs`/`CountAuditLogs` have only `LIMIT/OFFSET`, accepting no filter | ❌ must be rewritten following the `fraud.sql` template |
| `internal/product/service.go: ListAuditLogs` | Signature `(ctx, limit, offset int32)` | ❌ change to `(ctx, f listing.Filter)` like `ListFraudFlags`/`ListWorkflowInstances` |
| `internal/api/admin_ops_handler.go: ListAuditLogs` | Parses limit/offset manually, does not call `query.ToFilter()` | ❌ switch to the `FraudFlags` handler pattern (which already has `ToFilter()`) |
| `hpay-admin AuditPage.tsx`/`useAuditQueries.ts` | The "Event details" column is filtered client-side via `globalFilter`, with a note explicitly saying it is not sent to the server; the input has the placeholder "Filter within the current page" | ❌ change to follow the exact pattern of `OfferFactoryPage.tsx`/`ServiceCatalogPage.tsx` (`name: globalFilter \|\| undefined` sent straight to the hook) |

## 2. Scope & Non-goals

### Ships

- Backend: `ListAuditLogs`/`CountAuditLogs` filter by `name` (matching `action` OR `resource` OR `reason`, ILIKE), a date range `from`/`to` on `created_at`, and `order` — reusing the existing `listing.Filter`/`CommonQueryParams.ToFilter()` exactly.

- Admin: remove the "filter current page only" behavior, send `name` to the server exactly as the 2 pages that did it correctly; remove the misleading placeholder.

### Non-goals (defer)

- Searching by actor name/email (needs a join of `admin_users`/`users` depending on `actor_type` — `actor_id` is currently only a UUID, with no name to search) — deferred because the actor is currently displayed as `role.short_uuid`, not a real name.

- A date range picker UI on `AuditPage` — the backend already supports `from`/`to` through `listing.Filter` (like every other list), but adding a date picker is separate UI work, not part of this "search" request.

## 3. Task breakdown

| # | Task | Files | Size |
| --- | --- | --- | --- |
| T1 | Rewrite the `ListAuditLogs`/`CountAuditLogs` SQL following the `fraud.sql` template: `sqlc.narg('name')` matching action/resource/reason, `from_at`/`to_at`, `sort_order` | `internal/store/queries/audit.sql` | M |
| T2 | `sqlc generate`; change `Service.ListAuditLogs` to accept `listing.Filter` | `internal/product/service.go` | S |
| T3 | Handler uses `query.ToFilter()`, removing the manual limit/offset parsing | `internal/api/admin_ops_handler.go` | S |
| T4 | openapi: add query params `name/from/to/order/page/limit` for `/v1/admin/audit` | `api/openapi.yaml` | S |
| T5 | Tests: search matches action/resource/reason, no match returns empty, from/to filter the dates correctly | `test/integration/*_test.go` (find where audit is already tested or add a new file) | M |
| T6 | Admin: send `name: globalFilter \|\| undefined` to `useAuditQueries`, remove the client-side filter + old placeholder | `src/admin/src/pages/AuditPage.tsx` | S |

## 4. Schema changes

No migration — only the query changes; the `audit_logs` table and its 2 existing indexes (`idx_audit_actor`, `idx_audit_action`) are enough for `ILIKE` + a `created_at` range at the audit log's scale (no full-text index is needed for the current data volume).

## 5. API changes

```text
GET /v1/admin/audit?name=P2P&from=2026-07-01&to=2026-07-24&order=asc&page=1&limit=10
```

No new `ERR_*` — unparseable dates are ignored (keeping the existing `ParseFromTime/ParseToTime` behavior, no 400).

## 6. Test plan

| Case | Test |
| --- | --- |
| Search matches action | Write 2 audit logs with different actions, search by one, only exactly 1 is returned |
| Search matches resource/reason | Search for a string present only in `resource` or `reason`, still returns results (OR, not matching only action) |
| Nothing matches | Search for a non-existent string → `total=0`, no error |
| Date range | `from`/`to` correctly exclude records outside the range |

## 7. Rollout & risk

- Low risk: only 1 admin data-reading endpoint, does not touch ledger/workflow. No feature flag needed.

- The response shape is unchanged (`logs/total/page/limit/total_pages`) — the old admin (not yet deployed) can still call it, it just lacks the filter.

## 8. Execution checklist

- Manual (no `gen-migration` needed, no schema change) → T1–T4

- `gen-api-test` for T5

- `fintech-review` before committing (even though it is only a read path, still check secrets/PII in logs)

- Manual on the `hpay-admin` side → T6

---
HPay · plan generated by the gen-feature-plan skill · repo hpay-backend + hpay-admin · 2026-07-24

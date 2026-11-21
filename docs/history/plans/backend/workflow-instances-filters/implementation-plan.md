[← HPay Docs](../../../overview-v2.md) · repo hpay-backend + hpay-admin · plan

# Implementation Plan — Workflow Instances Filters

> **Feature slug:** `workflow-instances-filters` · The same kind of technical debt as [audit-log-search](../audit-log-search/implementation-plan.md): the Workflow Monitor UI (`WorkflowPage.tsx`) already has 3 filters (status/definition/stuck>5m), but a comment states outright "API has no such filters yet" — it filters client-side on exactly 1 page of data, not across the whole table.

## 1. Current state (surveyed)

| Component | Reality | Conclusion |
| --- | --- | --- |
| `internal/product/service.go: ListWorkflowInstances` | Accepts a `listing.Filter` but only uses `Limit/Offset`, ignoring every other field | ❌ identical to the audit-log-search gap that was just fixed |
| `internal/store/queries/workflow.sql: ListWorkflowInstances/CountWorkflowInstances` | No WHERE other than `LIMIT/OFFSET` | ❌ must be rewritten |
| `hpay-admin WorkflowPage.tsx` | 3 UI controls already exist: toggle "Stuck > 5 minutes", select status (all/failed/running/waiting/completed/compensated), select definition (all/org_disbursement/p2p_transfer/kyc_verification) | ✅ UI done, only the hook needs to send filters to the server instead of filtering in `useWorkflowQueries.ts: filterInstances()` |
| Frontend status label "waiting" | The real enum in the DB is `waiting_approval` (`wf_status`) | ⚠️ must map "waiting" → "waiting_approval" at the handler layer, without changing the UI label |
| Definition dropdown | Matches by display name `definitionId.includes(def)` (already joined as `"name vVersion"`) | ✅ becomes `d.name ILIKE %def%` in SQL — same behavior, correct across the whole table |

## 2. Scope & Non-goals

### Ships

- Backend: `ListWorkflowInstances`/`CountWorkflowInstances` filter by `status` (map "waiting"→"waiting_approval"), `definition` (definition name, ILIKE), `stuck_only` (status=running AND age > 5 minutes), plus `name` (matches `ref_code`) + `from`/`to` + `order`, which are already available through `listing.Filter` (zero cost, consistent with every other list).

- Admin: `useWorkflowInstances` sends `status/definition/stuck_only` to the server, removes the client-side `filterInstances()` and the "clientFiltered" warning in the comment.

### Non-goals (defer)

- Adding a free-text search box (ref_code) to the UI — `name` is supported in the backend for consistency, but there is no requirement for a new input on this page.

- Changing the existing 10s auto-refresh mechanism.

## 3. Task breakdown

| # | Task | Files | Size |
| --- | --- | --- | --- |
| T1 | Rewrite the SQL: `status` (cast `wf_status`→text for comparison), `definition` ILIKE on `d.name`, `stuck_only` boolean (status=running AND created_at < now()-5m), `ref_code` ILIKE, `from/to`, sort order | `internal/store/queries/workflow.sql` | M |
| T2 | `sqlc generate`; change the signature to `ListWorkflowInstances(ctx, f listing.Filter, status, definition string, stuckOnly bool)` | `internal/product/service.go` | S |
| T3 | Handler additionally reads `status` (map waiting→waiting_approval), `definition`, `stuck_only` from the query string | `internal/api/admin_ops_handler.go` | S |
| T4 | Tests: each filter individually + combined, stuck_only at the correct 5-minute threshold | `test/integration/workflow_instances_filter_test.go` (new) | L |
| T5 | Admin: `useWorkflowInstances` sends the filters to the server, removes client-side filtering | `src/admin/src/queries/useWorkflowQueries.ts`, `WorkflowPage.tsx` (calls the hook, UI unchanged) | M |

## 4. Schema changes

No migration — the `workflow_instances`/`workflow_definitions` tables already have enough columns, only the query changes.

## 5. API changes

```text
GET /v1/admin/workflows?status=waiting&definition=p2p_transfer&stuck_only=true&name=MP1A2B&from=2026-07-01&page=1&limit=10
```

`status` accepts both the short label `waiting` (mapped to `waiting_approval`) and the real enum values, so compatibility is not broken if someone calls the API directly with DB values.

## 6. Test plan

| Case | Test |
| --- | --- |
| Filter by status | Create a running + a completed instance, filter `status=completed` returns only that kind |
| Filter by status "waiting" (map) | An instance in `waiting_approval`, filter `status=waiting` still matches |
| Filter by definition | Instances from `p2p_transfer` vs `org_disbursement`, the filter picks the right kind |
| stuck_only | A freshly created running instance (not stuck) is excluded; a running instance with `created_at` set back > 5 minutes appears |
| Combining multiple filters | status + definition together narrow the result set correctly |

## 7. Rollout & risk

Low risk — read-only, 1 caller (the `ListWorkflows` handler), no money/ledger change. The response shape is unchanged.

## 8. Execution checklist

- Manual (no migration) → T1–T3

- `gen-api-test` → T4

- `fintech-review` before commit

- Manual on the `hpay-admin` side → T5

---
HPay · plan generated by skill `gen-feature-plan` · repo hpay-backend + hpay-admin · 2026-07-24

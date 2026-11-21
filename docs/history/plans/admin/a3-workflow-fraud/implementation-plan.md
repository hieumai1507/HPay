# Implementation Plan — A3 Workflow Monitor & Fraud hpay-admin

Phase that implements the **Workflow Monitor** (list instances, detail timeline, retry/compensate) and the **Fraud Monitor** (list flags, AI explanation, resolve dialog with consequences). Both screens auto-refresh every 10–30s.

## 1. Screens

| Screen | Mockup | APIs |
|---|---|---|
| Workflow Monitor (A4 mockup) | Instances table with stuck/status/definition filters; "Stuck > 5 minutes" badge; 10s auto-refresh | `GET /v1/admin/approvals` + workflow data from instances |
| Workflow Instance Detail (A5 mockup) | Header card + step execution timeline (VALIDATE→FRAUD_CHECK→WAIT_APPROVAL→DEBIT→CREDIT); I/O JSON viewer; Retry/Force fail buttons | `POST /v1/admin/approvals/:id/decide` (retry, compensate) |
| Fraud Monitor (A6 mockup) | Flags table with rule, score, SLA timer; row detail with AI explanation; bulk mark safe | `GET /v1/admin/fraud-flags` |
| Fraud Resolve Dialog (A7 mockup) | Modal with 3 options: lock + cancel / lock + reversal / mark only; reason ≥10 characters | `POST /v1/admin/fraud-flags/:id/resolve` |

## 2. Tasks

| # | Task | Files | Size | Depends |
|---|---|---|---|---|
| T1 | Workflow hooks: `useWorkflowInstances(filters)`, `useWorkflowDetail(id)`; auto-refetch interval 10s | `src/features/workflow/hooks.ts` | S | A1 |
| T2 | Workflow Monitor page: table with columns Instance/Definition/Txn/Amount/Current step/Age/Status/Action; filter bar stuck/status/definition; red "Stuck > 5 minutes" badge when applicable; 10s auto-refresh | `src/pages/WorkflowPage.tsx` | M | T1 |
| T3 | Workflow Instance Detail page: header card (txn ref, amount, type, start time, context viewer), step execution timeline (dot done/fail/active + output JSON), raw I/O JSON code viewer panel, Retry / Force fail buttons + reason | `src/pages/WorkflowDetailPage.tsx` | L | T1 |
| T4 | Retry / compensate action: POST decide → toast + invalidate; require reason; confirm dialog for "force fail" | `src/pages/WorkflowDetailPage.tsx` | S | T3 |
| T5 | Fraud hooks: `useFraudFlags(status)`, `useResolveFraudFlag()`; auto-refetch 30s | `src/features/fraud/hooks.ts` | S | A1 |
| T6 | Fraud Monitor page: table with checkbox multi-select; columns Flag/Rule chip/Score/Txn/Amount/User masked/Age (SLA warning)/Status; expandable row detail with AI explanation + quick action buttons | `src/pages/FraudPage.tsx` | M | T5 |
| T7 | Fraud Resolve Dialog (modal): 3 radio options (lock+cancel / lock+reversal / mark-only); reason textarea; POST resolve → close + invalidate + toast | `src/features/fraud/ResolveDialog.tsx` | M | T5 |

## 3. Workflow Instance Detail — Timeline component

The timeline needs a shared component `src/ui/Timeline.tsx` with props:
`steps: {name, status: 'done'|'fail'|'active'|'wait', durationMs?, output?, error?}[]`
Reused from mockup A5 — colored dot (green=done, red=fail, blinking green=active), line connector, JSON output collapse/expand.

## 4. Fraud SLA timer

Each flag displays its age (created_at → now) and is highlighted red if it exceeds 30 minutes (SLA per mockup A6). Uses a 60s `setInterval` to update the UI. No server push needed.

## 5. Verification

```
npm run typecheck
npm run build
# Manual check:
# - Workflow: filter "Stuck > 5 minutes" → open detail → view timeline → press Retry
# - Fraud: view flag SLA → open dialog → choose option → confirm
```

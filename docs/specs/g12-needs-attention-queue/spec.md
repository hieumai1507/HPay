---
title: Needs-attention queue for parked money cases (G12)
type: spec
status: done
last_verified: 2026-11-24
adr: ../../adr/0013-reversal-needs-manual.md
---

# Spec: needs-attention queue (PLAN G12)

## Problem
A reversal whose receiver spent the money is parked as `approval_requests.status = needs_manual` (ADR-0013), and a workflow whose compensation failed ends as `workflow_instances.status = compensation_failed` (ADR-0009, migration 000022). Both need a person. Nothing in the admin app shows them: the approvals inbox accepts only `status=pending`, and the workflow page has no such status in its type or filter. Parked money cases are invisible.

## Goals
1. Admin API: list approvals with `status=needs_manual` (with ref code, amount, age, reason); list workflows with `status=compensation_failed` (already supported by the generic status filter, now pinned by a test); a cheap read-only count endpoint for an alert badge.
2. Admin app: a "Needs attention" page listing both kinds with ref code, amount, age, reason and a link to the detail, an empty state, and a count badge on its sidebar entry.

## Non-goals
- Any action that resolves a parked case or moves money (resolution stays manual and undefined, see `domain/reversal.md`).
- A new permission, a migration, e-mail or push alerts.
- Visual design (the owner will redesign the admin UI): existing components only.

## Assumptions (no one to ask, safe defaults)
- A1 Permission: reuse `workflow.action` (same as `GET /v1/admin/approvals` and `/workflows`); roles risk_officer, ops, superadmin.
- A2 "Reason" of a parked approval = the reason of the newest `audit_logs` row whose resource ends with `(approval_request <id>)` (written by `finalizeOrPark`); falls back to `approval_requests.reason`.
- A3 "Age" = now minus `finalized_at` (the park time) for approvals and minus `created_at` for workflows (no dedicated park timestamp exists for workflows; `updated_at` is bumped by sweeps).
- A4 Approval rows have no admin detail page; the link goes to the audit log. Workflow rows link to `/workflows/:id`.
- A5 org_transaction subjects are excluded (admins do not decide them), same as the pending inbox.
- A6 The badge is polled every 30 s.

## Acceptance criteria
- **AC-1** Given an admin request parked as needs_manual, When an authorised admin calls `GET /v1/admin/approvals?status=needs_manual`, Then it is listed with ref_code, amount, fee, reason and parked_at, and pending requests are not listed; the default listing still returns only pending.
- **AC-2** Given an unsupported status (for example `approved`), Then the endpoint answers 400.
- **AC-3** Given workflow instances in `compensation_failed`, When `GET /v1/admin/workflows?status=compensation_failed`, Then only those are returned.
- **AC-4** `GET /v1/admin/attention/summary` returns `{needs_manual_approvals, compensation_failed_workflows, total}` counting exactly those two statuses (not compensated, not failed).
- **AC-5** The summary and the needs_manual list require `workflow.action`: viewer and kyc_reviewer get 403, no token 401; the summary has no write verb.
- **AC-6** The admin page lists both kinds with ref code, amount, age, reason and a detail link; with nothing parked it shows an empty state.
- **AC-7** The sidebar shows a count badge on "Needs attention" when total > 0 and no badge when 0; the entry and route are visible only to admins holding `workflow.action`.

## Invariants touched
C-08 (permission checked on the server, UI check is UX only). No money is posted, no ledger or status write: C-01 to C-06 are untouched because every new path is a SELECT.

## API and schema delta
- `GET /v1/admin/approvals` status enum becomes `[pending, needs_manual]`; needs_manual rows add `parked_at` and put the audit reason in `reason`.
- New `GET /v1/admin/attention/summary` (permission `workflow.action`).
- sqlc queries `ListNeedsManualAdminApprovals`, `CountNeedsManualAdminApprovals`, `CountAttentionItems`. No migration.
- OpenAPI and `reference/api.md` updated.

## Edge-case ids
None of the E/M/A catalogue applies (read-only feature).

## Architecture impact: **none**
Read: `architecture/` building blocks for admin API and approval. The change adds two read queries and one read endpoint inside existing modules (`approval`, `product`, `api`) and one admin page. Not a new endpoint family (same `/v1/admin` family, same guard).

## Domain pages changed
`domain/reversal.md`, `domain/approval-maker-checker.md`, `domain/workflow-engine.md`, `reference/api.md`, `product/feature-catalog.md`.

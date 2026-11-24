---
title: Needs-attention queue - test cases
type: spec
status: approved
last_verified: 2026-11-24
---

# Test cases: needs-attention queue (G12)

Source of the workbook: `test-cases.json`. Render with `python -X utf8 .claude/skills/hpay-qa-testcases/scripts/render_feature_workbook.py g12-needs-attention-queue`.

| ID | AC | Type | Executor | Test |
|----|----|------|----------|------|
| TC-G12-001 | AC-1 | Happy Path | AUTO-API | `TestAttention_ApprovalsFilterNeedsManual` |
| TC-G12-002 | AC-3 | Happy Path | AUTO-API | `TestAttention_WorkflowsFilterCompensationFailed` |
| TC-G12-003 | AC-4 | Happy Path | AUTO-API | `TestAttention_SummaryCounts` |
| TC-G12-004 | AC-2 | Negative | AUTO-API | `TestAttention_ApprovalsUnknownStatusRefused` |
| TC-G12-005 | AC-5 | Security/AuthZ | AUTO-API | `TestAttention_SummaryPermission` |
| TC-G12-006 | AC-5 | Security/AuthZ | AUTO-API | `TestAttention_SummaryPermission` |
| TC-G12-007 | AC-5 | Security/AuthZ | AUTO-API | `TestAdminRBAC_RoleMatrix`, `TestEveryAdminRouteHasAPermission` |
| TC-G12-008 | AC-6 | Edge Case | AUTO-WEB | `NeedsAttentionPage.test.tsx` empty state |
| TC-G12-009 | AC-6 | Happy Path | AUTO-WEB | `NeedsAttentionPage.test.tsx` lists both kinds |
| TC-G12-010 | AC-6 | Boundary | AUTO-WEB | `age.test.ts` |
| TC-G12-011 | AC-7 | Happy Path | HUMAN | badge visible (Blocked: no browser run) |
| TC-G12-012 | AC-7 | Security/AuthZ | HUMAN | no entry without permission (Blocked: no browser run) |
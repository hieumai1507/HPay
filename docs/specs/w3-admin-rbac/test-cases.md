---
title: Test cases - admin RBAC
type: spec
status: proposed
spec: spec.md
---

# Test cases (designed before code)

| Test ID | AC | Type | Priority | Executor | Steps | Expected | Automated as |
|---------|----|------|----------|----------|-------|----------|--------------|
| TC-RBA-001 | AC-1 | Security | P0 | AUTO-API | log in as a viewer, call every guarded route | 403 `ERR_FORBIDDEN_PERMISSION` each | `internal/api` `TestAdminRBAC_ViewerIsDeniedEverywhere` |
| TC-RBA-002 | AC-2 | Security | P0 | AUTO-API | for each of the 5 roles call every admin route | status is 403 exactly where the role lacks the permission, never 403 where it has it | `TestAdminRBAC_RoleMatrix` |
| TC-RBA-003 | AC-3 | Security | P0 | AUTO-API | valid token, then set the admin to `locked` | next call 401 `ERR_ACCOUNT_LOCKED` | `TestAdminRBAC_LockedAdminRefusedImmediately` |
| TC-RBA-004 | AC-4 | Security | P0 | AUTO-API | token issued as `ops`, role changed to `viewer` | next call is denied by the new role | `TestAdminRBAC_RoleChangeTakesEffect` |
| TC-RBA-005 | AC-5 | Negative | P1 | AUTO-API | token for an admin id that no longer exists | 401 | `TestAdminRBAC_DeletedAdminRefused` |
| TC-RBA-006 | AC-6 | Security | P0 | AUTO-API | create/update an AI key, update AI settings | audit rows with actor, action, resource; no key material in any column | `TestAdminAI_WritesAreAudited` |
| TC-RBA-007 | AC-7 | Regression | P1 | AUTO-API | parse `ROLE_PERMISSIONS` from `src/admin/src/stores/admin.ts` and compare with the Go table | identical | `internal/auth` `TestRolePermissionsMatchFrontend` |
| TC-RBA-008 | AC-8 | Security | P0 | AUTO-API | user-audience token on an admin route | 401 | `TestAdminRBAC_UserTokenRefused` |
| TC-RBA-009 | AC-9 | Regression | P0 | AUTO-API | full suite, sync check, admin typecheck | pass | whole suite |
| TC-RBA-010 | AC-2 | Security | P1 | AUTO-API | every route registered under `/v1/admin` has a permission entry | no unguarded admin route can exist | `TestEveryAdminRouteHasAPermission` |
---
id: BUG-0013
title: Any authenticated admin, including a viewer, can call every admin route; a locked admin keeps access until the token expires
status: fixed
severity: high
area: backend/auth
tags: [authz, session, ai-key]
audit: [BE-H6]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/test/integration/admin_rbac_test.go ; src/backend/internal/auth/rbac_test.go
related: [BUG-0012]
---

# BUG-0013: No server-side authorization on admin routes

## Symptom
A `viewer` could create AI keys, override organization policies, file reversals, run reconciliation and read every list. A locked or demoted admin kept full access until the 15-minute token expired. AI key and setting changes left no audit row.

## Root cause
`RequireAdmin` validated signature, expiry and audience only and never loaded the admin; the role was returned by login but is not in the token, and no handler checked it. The frontend role table was a UX hint, with nothing on the server to match it.

## Other places with the same flaw
`GET /v1/files/*key` (signed link for KYC images and recon files) has no admin check at all: access is by the signed link alone (BE-H9, not part of this fix). Org routes use org roles, not admin roles, and were not changed.

## Fix
ADR-0015. `RequireAdmin` loads the admin from the database on every request (unknown id: 401; `status != active`: 401 `ERR_ACCOUNT_LOCKED`; role taken from the row). `RequirePermission` is attached to every `/v1/admin` route with the table in `docs/specs/w3-admin-rbac/spec.md`; a viewer holds nothing and sees nothing; denials return 403 `ERR_FORBIDDEN_PERMISSION` without naming a permission. The role table lives in `internal/auth/rbac.go` and a test compares it with `src/admin/src/stores/admin.ts`, so drift fails the build. `TestEveryAdminRouteHasAPermission` fails if a route is added without a decision. AI key create/update and AI setting update write audit rows (`ai.key.create`, `ai.key.update`, `ai.setting.update`) with actor and client IP and never any key material.

## Regression test
`TestAdminRBAC_ViewerIsDeniedEverywhere` (24 routes), `TestAdminRBAC_RoleMatrix` (5 roles x 24 routes), `TestAdminRBAC_LockedAdminRefusedImmediately`, `TestAdminRBAC_RoleChangeTakesEffect`, `TestAdminRBAC_DeletedAdminRefused`, `TestAdminRBAC_UserTokenRefused`, `TestEveryAdminRouteHasAPermission`, `TestAdminAI_WritesAreAudited`, `TestRolePermissionsMatchFrontend` (verified to fail when a frontend permission is changed; it fails, not skips, when the file is missing), `TestRoleHas`. Red first: 5 of the 7 initial tests failed before the middleware existed.

## How to recognize this again
- A viewer session loading admin data. `admin_users.status = 'locked'` rows whose last audit action is later than the lock.
- A new route under `/v1/admin` that the table test does not know.
- Audit gaps for configuration changes.

## Docs updated
`product/roles-and-permissions.md`, `domain/identity-and-auth.md`, `architecture/admin-app.md`, `constitution.md` (C-08), `adr/0015-admin-rbac.md`, audit report.

## Not done
Admin refresh tokens, a separate signing secret for admin tokens, MFA for admins, admin user management endpoints (`admins.manage` has no feature yet), audit rows for the other admin writes (KYC review and reversals already audit through their services; service status and org policy writes need a check), file route authorization (BE-H9).
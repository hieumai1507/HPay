---
title: Wave 3 slice - admin authorization (RBAC) and admin session checks
type: spec
status: proposed
last_verified: 2026-11-21
audit: [BE-H6]
related:
  - ../../product/roles-and-permissions.md
  - ../../domain/identity-and-auth.md
  - ../../adr/0015-admin-rbac.md
---

# Wave 3 slice: admin RBAC (BE-H6)

## Problem (verified in code)
- `RequireAdmin` (`api/admin_handler.go`) only validates signature, expiry and the `admin` audience and stores `admin_id`. The role is returned by login but is not in the token and is never checked, so a `viewer` can call every `/v1/admin/*` route, including AI key management, org policy overrides, reversals and the recon run.
- The token is never re-checked against the database: a locked or demoted admin keeps full access until the token expires (15 minutes), and there is no refresh.
- The same `JWT_SECRET` signs user and admin tokens (separated only by the `aud` claim).
- Destructive and configuration actions are not all audited (AI keys and settings write no audit row).
The admin frontend already maps roles to permissions (`src/stores/admin.ts ROLE_PERMISSIONS`) and says plainly that this is UX only.

## Goals
Every admin route enforces the permission it needs, on the server, using the admin's CURRENT role and status from the database; destructive or configuration actions write an audit row with the actor.

## Non-goals
Admin refresh tokens, a separate signing secret, admin user management screens (`admins.manage` has no endpoints yet), MFA for admins.

## Design
Permissions are the frontend's names so the two sides cannot drift: `kyc.review`, `fraud.resolve`, `workflow.action`, `recon.run`, `orgs.edit`, `ai.manage`, `admins.manage`. The role to permission table lives in ONE Go place (`internal/auth/rbac.go`) and a test compares it with the table in `src/admin/src/stores/admin.ts`, so a change on one side fails the build until the other follows.

| Route | Permission |
|-------|-----------|
| `GET /kyc`, `POST /kyc/:id/review` | `kyc.review` |
| `GET /fraud-flags`, `POST /fraud-flags/:id/resolve` | `fraud.resolve` |
| `POST /reversals` | `workflow.action` |
| `GET /approvals`, `POST /approvals/:id/decide` | `workflow.action` |
| `POST /recon/run`, `GET /recon/runs/:date` | `recon.run` |
| `GET /workflows`, `GET /workflows/:id` | `workflow.action` |
| `GET /orgs`, `PUT /orgs/:id/policies` | read: `orgs.edit` or `workflow.action`; write: `orgs.edit` |
| `GET /services`, `GET /services/:id`, `GET /offers`, `GET /offers/:id` | read: any admin permission |
| `PUT /services/:id` | `admins.manage` |
| `GET/POST/PUT /ai/*` | `ai.manage` |
| `GET /audit` | any admin permission (a `viewer` has none and sees nothing) |
| `GET /v1/files/*key` (KYC image) | `kyc.review` |

`RequireAdmin` loads the admin row on every request (cheap primary-key read): `status` must be `active` and the role is taken from the row, never from the token. A locked admin is refused immediately.

## Acceptance criteria
- **AC-1** A `viewer` receives 403 `ERR_FORBIDDEN_PERMISSION` on every route in the table that needs a permission and on `GET /audit`; the response names no permission list.
- **AC-2** Each role can call exactly the routes its permissions allow (test matrix over all 5 roles and all routes).
- **AC-3** An admin whose `status` becomes `locked` is refused on the next request even with a valid token (401 `ERR_ACCOUNT_LOCKED`).
- **AC-4** An admin whose role is changed in the database is authorized by the NEW role on the next request.
- **AC-5** A token for a deleted admin id is refused.
- **AC-6** AI key create, update and AI settings update write an audit row (actor admin, action, resource, no key material).
- **AC-7** The Go role table equals the TypeScript role table (automated comparison).
- **AC-8** A user token (audience `user`) is still refused on admin routes (regression).
- **AC-9** Full suite, OpenAPI sync and admin typecheck pass; `docs/product/roles-and-permissions.md` loses the "UX only" caveat for the routes that are now enforced.

## Invariants touched
C-08 (two-layer authorization with audit). Architecture impact: `amend` (admin auth middleware and the permission table). ADR-0015 (proposed).

## Constitution check
| ID | Touched | Compliant | How |
|----|---------|-----------|-----|
| C-08 | yes | yes after change | role enforced per route from the database, audit on destructive and configuration writes |
| C-11 | yes | yes | AI key audit rows hold no key material |
| C-14 | yes | yes | tests first, role table cross-checked with the frontend |

## Open questions (assumptions in bold)
1. Services and offers have no dedicated permission. **Reads: any admin permission; writes: `admins.manage` (superadmin).** Add a dedicated `catalog.edit` permission if product wants ops to edit services.
2. **`viewer` sees nothing**, as the frontend already assumes; if product wants read-only dashboards for viewers, add a `read` permission.
3. Per-request admin lookup adds one primary-key query per admin call; **no cache** (admin traffic is low).
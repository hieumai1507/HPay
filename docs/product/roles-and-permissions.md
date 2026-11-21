---
title: Roles and permissions
type: product
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/internal/api/router.go, src/backend/internal/api/middleware.go, src/backend/internal/api/admin_handler.go, src/backend/internal/org/service.go, src/backend/internal/approval/service.go, src/backend/internal/approval/admin.go, src/backend/internal/auth/admin.go, src/backend/db/migrations/000001_users_admin_auth.up.sql, src/admin/src/stores/admin.ts, src/mobile/src/stores/org.ts]
---

# Roles and permissions

Three independent role systems. Backend enforcement and frontend gating are different, and the frontend is stricter than the backend in places; the backend is the authority.

## 1. End user (personal)

Identity: `users` row, user JWT (aud `user`). No roles. Any authenticated user can call every `/v1/*` public route for their own data; ownership is checked in queries (wallet by owner, transaction history by wallet). Account state matters only at login (`ErrAccountLocked`) and at posting time (MOVE_FUNDS rechecks sender/receiver `active`, `handlers.go:95-106`); an already-issued access token is not revoked for locked users (BE-H6).

Gating that depends on state, not role: KYC status (`none`, `pending`, `manual_review`, `auto_approved`, `verified`, `rejected`) drives the legacy per-transaction cap and the mobile redirect to KYC intro; offer package drives service availability and limits (see [kyc-tiers-and-limits.md](kyc-tiers-and-limits.md)). Fee previews, lookup and QR resolve are rate limited per user.

## 2. Organization roles (business users)

`org_members(org_id, user_id, role, checker_level)` with enum `org_role` = `admin | maker | checker`, UNIQUE `(org_id, user_id)` so one role per user per org. Created by `POST /v1/org` (creator becomes admin, `org/service.go:71`) and `POST /v1/org/:id/members` (`org_handler.go:48` validates `oneof=admin maker checker`).

| Action | Endpoint | Backend check | Mobile gating |
|---|---|---|---|
| List my orgs | `GET /v1/org` | any user | account tab |
| Create org | `POST /v1/org` | any authenticated user | account tab |
| Add member | `POST /v1/org/:id/members` | org role `admin` (`requireRole`, `org/service.go:144`) | `org/admin.tsx` |
| Create approval policy | `POST /v1/org/:id/policies` | org role `admin` (`:197`), tiers validated | `org/admin.tsx` |
| Top-up org wallet | service method `Topup` requires `admin` (`:279`); no route mounted | - | - |
| Create disbursement | `POST /v1/org/:id/transactions` | org role `maker` (`:363`), active org, active policy for amount, <= 1,000,000,000, Idempotency-Key | `org/disburse.tsx`; uses `activeOrg.role` |
| Approvals inbox | `GET /v1/org/approvals` | query returns pending requests for the checker's memberships | `org/approvals.tsx` |
| Approval detail | `GET /v1/org/approvals/:id`, `/by-ref/:ref_code` | viewer must be related (maker or eligible checker) per `Detail` | `approval-detail.tsx`, `order-status.tsx` |
| Challenge (issue OTP) | `POST /v1/org/approvals/:id/challenge` | eligibility check, rate limit 10/min | `approval-detail.tsx:201` |
| Decide | `POST /v1/org/approvals/:id/decide` | OTP valid; member with role `checker`; `checker_level` equals the request's current tier; one action per checker/level (unique) ; not expired | approve/reject buttons |
| Cancel | `POST /v1/org/approvals/:id/cancel` | maker who created it (`approval/service.go:176`) | `order-status.tsx` |

Multi-tier model: an approval policy holds tiers (level N needs k signatures from checkers of level N); the policy is snapshotted into the request at creation, `Evaluate` decides pending/approved/rejected (`approval/evaluate.go`). Docs call these "L1/L2 checkers"; in the schema they are `role=checker` plus `checker_level`.

Gaps: no `checkerID != createdBy` rule in `eligibility` (the unique membership makes one user have one role per org, so maker and checker overlap requires being in two orgs or an admin-role path; the audit marks self-approval as CONFIRMED for the org flow); `admin` org role can create policies but is not a checker; org role is not checked when listing org data via the mobile store (the mobile `ActiveOrg.role` is client state from `GET /v1/org`, display gating only); balance/limit not re-checked at MOVE_FUNDS after approval apart from ledger overdraft (audit).

## 3. Admin roles (back-office)

Table `admin_users(email, password_hash, full_name, role, status)` with enum `admin_role` = `viewer | kyc_reviewer | risk_officer | ops | superadmin` (migration 000001:9,57; default `viewer`; a dev superadmin is seeded at 000001:77-81). Login: `POST /v1/admin/auth/login` (rate limit 5/min) returns an `aud=admin` JWT and the admin profile including role; locked admins get `ERR_ACCOUNT_LOCKED` (`auth/admin.go:45`).

### What the backend enforces (ADR-0015, BUG-0013; fixed in the working tree 2026-11-21, not yet committed)

`RequireAdmin` validates signature, expiry and audience, then loads the admin from `admin_users` on every request: an unknown id is refused (401), a status other than `active` is refused (401 `ERR_ACCOUNT_LOCKED`), and the role is taken from the row, never from the token, so a lock or a demotion applies on the next call. `RequirePermission` is attached to every `/v1/admin` route. The role table is `internal/auth/rbac.go` and a test compares it with `src/admin/src/stores/admin.ts`.

| Route | Permission (any one is enough) |
|---|---|
| `GET /kyc`, `POST /kyc/:id/review` | `kyc.review` |
| `GET /fraud-flags`, `POST /fraud-flags/:id/resolve` | `fraud.resolve` |
| `POST /reversals`, `GET /approvals`, `POST /approvals/:id/decide`, `GET /workflows`, `GET /workflows/:id` | `workflow.action` |
| `POST /recon/run`, `GET /recon/runs/:date` | `recon.run` |
| `GET /orgs` | `orgs.edit` or `workflow.action` |
| `PUT /orgs/:id/policies` | `orgs.edit` |
| `GET /services`, `GET /services/:id`, `GET /offers`, `GET /offers/:id`, `GET /audit` | any permission (a `viewer` holds none and sees nothing) |
| `PUT /services/:id` | `admins.manage` |
| `POST /users/:id/pin/reset` | `pin.reset` (superadmin only; removes a user's PIN after out-of-band identity verification, reason required, audited) |
| `POST /users/:id/totp/reset` | `totp.reset` (superadmin only; removes a user's authenticator and recovery codes after out-of-band identity verification, reason required, audited) |
| `GET/POST/PUT /ai/keys`, `GET/PUT /ai/settings` | `ai.manage` |

A denial is 403 `ERR_FORBIDDEN_PERMISSION` and does not name a permission. AI key create/update and AI setting update write audit rows (`ai.key.create`, `ai.key.update`, `ai.setting.update`) with the actor and client IP and no key material. Not covered: `GET /v1/files/*key` (signed link only, no identity; BE-H9), admin refresh tokens, a separate admin signing secret, MFA, and audit rows for every other admin write.
### What the admin frontend gates

`stores/admin.ts` maps roles to permissions; the backend now enforces the same table (see above) and the frontend uses it to hide what a role cannot use:

| Role | Permissions in the UI map |
|---|---|
| `viewer` | none |
| `kyc_reviewer` | `kyc.review` |
| `risk_officer` | `fraud.resolve`, `workflow.action` |
| `ops` | `workflow.action`, `recon.run` |
| `superadmin` | all: `kyc.review`, `fraud.resolve`, `workflow.action`, `recon.run`, `orgs.edit`, `ai.manage`, `admins.manage`, `pin.reset`, `totp.reset` |

Consulted by the UI (fixed in working tree, 2026-11-21, not yet committed): `kyc.review` (KYC review buttons), `workflow.action` (My Tasks decide, workflow retry/compensate), `fraud.resolve` (fraud resolve buttons), `recon.run` (run reconciliation), `orgs.edit` (org policy editor), `ai.manage` (AI keys and routing), `admins.manage` (service status change; reused as the platform-configuration permission because services and offers have no dedicated permission, **product decision pending**). A `RequirePermission` wrapper (inside `RequireAdmin`) now guards the routes below and the sidebar hides the same links. Source: `src/admin/src/lib/access.ts` `ROUTE_ACCESS`.

| Route | Visible when the role holds any of |
|---|---|
| `/fraud` | `fraud.resolve` |
| `/recon` | `recon.run` |
| `/orgs` | `orgs.edit`, `workflow.action` |
| `/ai-settings` | `ai.manage` |
| `/services`, `/offers`, `/offers/:id` | `admins.manage`, `orgs.edit`, `ai.manage` |
| `/audit` | any permission (a `viewer` sees nothing) |

**The frontend guards are a convenience; the server enforces permissions (ADR-0015).** `/dashboard`, `/kyc`, `/workflows` and `/tasks` are not route-guarded (their buttons are gated where a permission exists). `admins.manage` has no feature behind it (no admin-user management endpoint or page).

### Intended vs actual summary

| Capability | Intended role (UI map) | Backend enforces | Frontend enforces |
|---|---|---|---|
| Review KYC | kyc_reviewer, superadmin | kyc.review | buttons for kyc_reviewer, superadmin |
| Resolve fraud flag | risk_officer, superadmin | fraud.resolve | page and buttons gated by `fraud.resolve` |
| Decide admin approvals (reversal) | risk_officer, ops, superadmin | workflow.action (self-approval blocked) | buttons gated by `workflow.action` |
| Run recon | ops, superadmin | recon.run | page and run button gated by `recon.run` |
| Edit org policy | superadmin | orgs.edit | edit controls gated by `orgs.edit`, page visible to `orgs.edit` or `workflow.action` |
| AI keys/settings | superadmin | ai.manage (writes audited) | page and writes gated by `ai.manage` |
| Services/offers | none defined (`admins.manage` reused for writes, any permission for reads; decision pending) | admins.manage for status change; any permission for reads | pages and service status change gated |

## 4. System actors

Mock bank: shared-secret header; the route exists only outside production and the secret has no default outside dev and ci (ADR-0012). Workers: system actor id for audit entries (`auth/service.go:18`). Files: signed links valid 5 minutes, no identity.

## Open gaps to close

1. ~~Role claim in admin JWT plus a backend `RequirePermission` middleware (BE-H6).~~ Done in the working tree (database role on every request; ADR-0015), not yet committed.
2. ~~Gate fraud, recon, org policy, AI pages in the UI to match.~~ Done in the UI and now backed by server enforcement.
3. Explicit self-approval block for org approvals.
4. Separate admin signing secret and refresh/revocation for both token types.

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

### What the backend enforces

Only authentication: `RequireAdmin` verifies signature, expiry and audience and stores `admin_id` (`admin_handler.go:25-42`). The role is NOT in the token claims used for authorization and NO handler or service checks it. Therefore every authenticated admin, including `viewer`, can call every `/v1/admin/*` route:

KYC queue and review, fraud flags and resolve, create reversal, admin approvals and decide (`approval.AdminDecide` only checks `adminID != createdBy` style self-approval for reversals, `ErrSelfApproval` via `approval/admin.go:67`), recon run and read, service list/get/status update, offers list/get, workflows list/get, audit list, orgs list and `PUT /orgs/:id/policies`, AI keys create/update/list and AI settings update, signed file fetch (HMAC link only, no auth). This is audit BE-H6; no RBAC middleware exists.

### What the admin frontend gates

`stores/admin.ts` maps roles to permissions (UX only):

| Role | Permissions in the UI map |
|---|---|
| `viewer` | none |
| `kyc_reviewer` | `kyc.review` |
| `risk_officer` | `fraud.resolve`, `workflow.action` |
| `ops` | `workflow.action`, `recon.run` |
| `superadmin` | all: `kyc.review`, `fraud.resolve`, `workflow.action`, `recon.run`, `orgs.edit`, `ai.manage`, `admins.manage` |

Actually consulted by pages: `kyc.review` (KYCDetailPage:44, enables review buttons) and `workflow.action` (MyTasksPage:39, enables decide). The permissions `fraud.resolve`, `recon.run`, `orgs.edit`, `ai.manage` are defined but not checked by any page, and `admins.manage` has no feature behind it (no admin-user management endpoint or page exists). Routes and the sidebar are identical for all roles. `RequireAdmin` only checks for a token.

### Intended vs actual summary

| Capability | Intended role (UI map) | Backend enforces | Frontend enforces |
|---|---|---|---|
| Review KYC | kyc_reviewer, superadmin | any admin | buttons for kyc_reviewer, superadmin |
| Resolve fraud flag | risk_officer, superadmin | any admin | not gated |
| Decide admin approvals (reversal) | risk_officer, ops, superadmin | any admin (self-approval blocked) | buttons gated by `workflow.action` |
| Run recon | ops, superadmin | any admin | not gated |
| Edit org policy | superadmin | any admin | not gated |
| AI keys/settings | superadmin | any admin | not gated |
| Services/offers | none defined | any admin (offers read-only on backend) | not gated |

## 4. System actors

Mock bank: shared-secret header (default secret in every env, BE-H7). Workers: system actor id for audit entries (`auth/service.go:18`). Files: signed links valid 5 minutes, no identity.

## Open gaps to close

1. Role claim in admin JWT plus a backend `RequirePermission` middleware matching the UI map (BE-H6).
2. Gate fraud, recon, org policy, AI pages in the UI to match.
3. Explicit self-approval block for org approvals.
4. Separate admin signing secret and refresh/revocation for both token types.

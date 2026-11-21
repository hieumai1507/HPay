---
id: ADR-0015
title: Enforce admin permissions on the server from the database role, one table shared with the frontend
status: proposed
date: 2026-11-21
supersedes: []
superseded_by: []
spec: specs/w3-admin-rbac/spec.md
affects:
  - domain/identity-and-auth.md
  - product/roles-and-permissions.md
  - architecture/admin-app.md
  - architecture/crosscutting-concepts.md
constitution: [C-08, C-11]
---

# ADR-0015: Enforce admin permissions on the server from the database role

Status: **proposed**. Written under the lead's instruction to complete the plan before review; the user approves it with the final change set.

## Context
Admin routes check only that a token is valid (audit BE-H6). The frontend already hides what a role cannot use and says so plainly (`roles-and-permissions.md`). The role is not in the token, the token is never re-checked, and a locked admin keeps access for the token's lifetime.

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| **A. Per-route permission middleware; the role and status are read from `admin_users` on every request; one Go permission table, cross-checked by a test against the frontend table** | Immediate effect of lock and role change; no token format change; no drift between client and server; tiny code | One primary-key query per admin request | Low |
| B. Put the role in the JWT and trust it | No query | A locked or demoted admin keeps access until expiry; role changes need token revocation | Medium; repeats the BE-H6 root cause |
| C. A full policy engine or external authorization service | Flexible | Over-built for five roles and about 25 routes | High |
| D. Keep UX-only guards | No change | Not acceptable for a finance back office | Rejected |

## Decision
Adopt **A**. Permissions reuse the frontend's names. `RequireAdmin` becomes a loader (admin must exist and be active; role from the database) and a second middleware `RequirePermission(...)` is attached per route. Reads that any admin may perform require at least one permission, so a viewer sees nothing. Destructive and configuration writes write an audit row (actor, action, resource), never key material.

## Constitution check
C-08: both layers present (role from the database, audit with actor). C-11: audit rows carry no secrets. No deviation.

## Consequences
- Easier: a lock or demotion is effective at once; adding a route without a permission fails a test.
- Harder: every new admin route needs a permission entry; one extra read per admin request.
- Risk: the permission names are a contract with the admin app; the cross-check test fails the build if they drift. Follow-ups: admin refresh and MFA, a separate signing secret for admin tokens, `admins.manage` endpoints.

## Documentation to update
`domain/identity-and-auth.md`, `product/roles-and-permissions.md`, `architecture/admin-app.md`, `architecture/crosscutting-concepts.md`, `constitution.md` (C-08), `adr/README.md`.
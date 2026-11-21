[← HPay Docs](../../../overview-v2.md) · src/backend/docs/plans/f6-org-disbursement-endpoint

# Implementation Plan — Org Disbursement HTTP Endpoint (F6 gap-fill)

> F6 (Approval Engine) shipped `org.Service.CreateDisbursement` fully implemented and covered by integration tests, but no HTTP route ever called it — the mobile maker screen (`app/(main)/org/disburse.tsx`) has been calling a route that 404s. This plan wires the existing service to HTTP; it does not change approval/policy/workflow logic.

## 1. Scope & non-goals

**In scope:** one new handler method on `OrgHandler`, one new route `POST /v1/org/:id/transactions`, `RequireIdempotencyKey()` middleware on it, an `openapi.yaml` entry, and an HTTP-layer integration test.

**Non-goals:** no change to `org.Service.CreateDisbursement`, no schema/migration, no change to the approval engine, no change to the mobile client (its `useDisburseOrgMoney` hook / `OrgDisbursementResult` type already matches `org.DisbursementResult` field-for-field — verified by reading `src/mobile/src/queries/useOrgQueries.ts`, so the "expects `transaction_id`" concern in the original ask does not hold against current code).

## 2. Task breakdown

| # | Task | Files touched | Size | Depends on |
| --- | --- | --- | --- | --- |
| 1 | Add `OrgHandler.Disburse` handler: bind `{phone, amount, note}`, call `svc.CreateDisbursement` with `c.GetString("idempotency_key")` (same convention as `TopupHandler`/`TransferHandler`), return `201` with the raw `org.DisbursementResult` (ref_code/status/amount/fee/total/created_at/duplicate — already the correct wire shape) | `internal/api/org_handler.go` | S | — |
| 2 | Register route `og.POST("/:id/transactions", RequireIdempotencyKey(), orgH.Disburse)` in the org group, next to members/policies | `internal/api/router.go` | S | 1 |
| 3 | Add `/v1/org/{id}/transactions` POST operation to the spec: request schema, 201 body ref to a new `DisbursementResult` schema, error responses for every `CreateDisbursement` failure mode | `api/openapi.yaml` | S | 1 |
| 4 | HTTP-layer integration test: build a minimal gin router (pattern from `m3_support_test.go`'s `userRouter`) mounting only this route with idempotency middleware, drive it with `httptest`, assert 201 body shape, 400 missing-header, and one domain-error mapping (e.g. wrong role → 403 `ERR_FORBIDDEN_ROLE`) | `test/integration/org_disbursement_http_test.go` (new) | M | 1,2 |
| 5 | `scripts/check-openapi-sync.py` must pass (router↔spec parity gate) | none (verification) | S | 2,3 |
| 6 | Run `fintech-review` skill before commit (money-moving endpoint, idempotency + error-contract review) | none (verification) | S | 1–4 |

## 3. Schema changes

None. `CreateDisbursement`, its tables (`transactions`, `approval_requests`, `workflow_instances`) and its unique idempotency constraint already exist and are exercised by `test/integration/approval_test.go`.

## 4. API changes

| Method & path | Auth | Success | ERR_* codes (existing, from `errmap.go`) |
| --- | --- | --- | --- |
| `POST /v1/org/{id}/transactions` | Bearer user JWT + `Idempotency-Key` header (required, ≤64 chars) | `201` DisbursementResult | `ERR_IDEMPOTENCY_KEY_REQUIRED` (400, middleware) · `ERR_NOT_ORG_MEMBER` / `ERR_FORBIDDEN_ROLE` (403, non-maker) · `ERR_AMOUNT_INVALID` (400) · `ERR_LIMIT_EXCEEDED` (400) · `ERR_ORG_INACTIVE` (403) · `ERR_RECEIVER_NOT_FOUND` (404) · `ERR_WALLET_NOT_FOUND` (404) · `ERR_NO_POLICY` (400) · `ERR_INSUFFICIENT_BALANCE` (400) |

Request body: `{ phone: string, amount: int64, note?: string }` — matches what `hpay-mobile`'s `useDisburseOrgMoney` already sends. Response body is the literal JSON tags on `org.DisbursementResult`: `ref_code, status, amount, fee, total, created_at, duplicate?`. `openapi.yaml` diff: one new path item + one new reusable schema (`DisbursementResult`), following the sibling `/v1/org/{id}/policies` entry's structure.

## 5. Test plan

| ID / name | Covers | Status |
| --- | --- | --- |
| `TestOrgDisbursementTwoTierEndToEnd`, `TestOrgDisbursementRejectFailsClean`, `TestEdge_E08_CheckerRaceSingleFinalize`, `TestEdge_E09_PolicySnapshotSurvivesPolicyChange`, `TestEdge_E10_ExpiredApprovalFailsClean`, `TestApprovalEligibility` | Service-layer behavior of `CreateDisbursement` (E08/E09/E10) | Existing — unchanged, still call `e.osvc.CreateDisbursement` directly |
| `TestOrgDisbursementHTTP_Success` | New: full HTTP round-trip, 201 + correct JSON field names | New |
| `TestOrgDisbursementHTTP_MissingIdempotencyKey` | New: middleware 400 `ERR_IDEMPOTENCY_KEY_REQUIRED` when header absent | New |
| `TestOrgDisbursementHTTP_ForbiddenRole` | New: checker/admin (non-maker) hitting the route gets 403 `ERR_FORBIDDEN_ROLE` over HTTP, proving `respondErr` mapping works end-to-end for this route | New |

## 6. Rollout & risk

**Feature flag:** none needed — this only adds a route; nothing upstream stops calling it, and the endpoint is additive.

**Backfill:** none.

**Half-done merge risk:** low. If tasks 1–2 land without 3–4, the endpoint works but `check-openapi-sync.py` fails CI (by design) and there's no dedicated HTTP test — acceptable mid-PR state, not mid-*merge* state.

**Rollback:** revert the single commit; the service method and its existing tests are untouched, so rollback has zero blast radius on F6.

> **Known follow-up, not in this plan's scope:** the mobile screen currently has no loading/duplicate-specific UI for the `duplicate: true` replay case — it just navigates to order-status either way, which is correct behavior but worth a UX pass later.

## 7. Execution checklist

- impact({target:"CreateDisbursement", direction:"upstream"}) — confirmed 0 callers today (LOW risk); re-run after wiring to confirm exactly 1 new caller (the handler).

- Write `OrgHandler.Disburse` + route (no scaffold-module needed — extends an existing module).

- gen-migration — not invoked (no schema change).

- gen-api-test — used as a style reference for the new HTTP test file; test written by hand since it also needs to assert the openapi/router parity contract this repo enforces.

- Update `api/openapi.yaml`; run `python scripts/check-openapi-sync.py`.

- detect_changes() before commit; confirm only `OrgHandler`, router registration, and the new test file are affected — no drift into unrelated processes.

- fintech-review skill — mandatory final gate (money-moving endpoint).

---
src/backend/docs/plans/f6-org-disbursement-endpoint · generated via gen-feature-plan

# 📋 Implementation Plan — M3 API Support `backend`

A small backend-side phase serving mobile M3 (Core Screens). Source: a review of the plan `src/mobile/docs/plans/m3-core-screens` found 4 places where the client has to "make do"/hardcode because a contract is missing — violating the two-way contract-first rule. This phase adds all 4 APIs so mobile has no workaround left.

## 1. Scope & non-goals

**Ships** — 4 API changes (details in §3):

- **A1 — Receiver lookup:** `GET /v1/users/lookup?phone=` returns a masked name so the Transfer screen can confirm the recipient before transferring.
- **A2 — Counterparty in history/detail:** add `direction` + `counterparty_name` (masked) + `counterparty_phone` (masked) to `GET /v1/transactions` and `GET /v1/transactions/{refCode}` — TxnListItem needs the "counterparty name", and the recent-contacts list needs the phone number.
- **A3 — Expose KYC limit:** `GET /v1/users/me` returns `{user, txn_limit}` — the client drops the hardcoded limit map (the map in the current mobile plan already diverges from `wallet.TxnLimit()`).
- **A4 — Failure code on the timeline:** `GET /v1/transactions/{refCode}/timeline` adds `failure_code` (sanitized ERR_*) when the instance is failed/compensated — the client maps it to a Vietnamese message (M03).

**Non-goals:** no DB schema change (all data already exists, only join/expose); no full-text search of contacts; no engine/workflow change.

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Masking helpers `MaskName`/`MaskPhone` (pure, table-driven test) | `internal/user/mask.go` (+test) | S | — |
| T2 | A1 lookup: query `GetUserByPhone` (already exists? — reuse), service + handler + rate-limit 20/min/user against phone-number probing; 404 when not found (no distinction between "does not exist" vs "locked") | `internal/api/user_handler.go`, `internal/store/queries/users.sql` | M | T1 |
| T3 | A2: change the SQL `ListTransactionsByWallet` + `GetTransactionByRefCode` to join both directions wallets→users, service computes `direction` from the caller's wallet, new DTO (does not leak the sqlc row) | `internal/store/queries/transactions.sql`, `internal/wallet/service.go`, `internal/api/wallet_handler.go` | L | T1 |
| T4 | A3: `GET /v1/users/me` — current user + `txn_limit` from `wallet.TxnLimit(kyc_status)` | `internal/api/user_handler.go` | S | — |
| T5 | A4: `TimelineFor` reads `step_executions.error` of the last failed step → maps to the `failure_code` ERR_* via a fixed table in `errmap.go`; unrecognized error → `ERR_TRANSFER_FAILED` (generic, no internal leak) | `internal/transfer/service.go`, `internal/api/errmap.go` | M | — |
| T6 | openapi.yaml: 2 new endpoints + 3 extended responses — SAME COMMIT | `api/openapi.yaml` | S | T2–T5 |
| T7 | Integration tests (testcontainers) — see §5 | `test/integration/m3_support_test.go` | L | T2–T5 |

## 3. API changes (openapi.yaml diff)

| Endpoint | Change | Response | Errors |
| --- | --- | --- | --- |
| `GET /v1/users/lookup?phone=` **(new)** | JWT + rate-limit 20/min/user | `{found:true, masked_name}` | 404 ERR_USER_NOT_FOUND · 400 ERR_VALIDATION · 429 ERR_RATE_LIMITED |
| `GET /v1/users/me` **(new)** | JWT | `{user:{id,phone,full_name,kyc_status}, txn_limit}` | 401 |
| `GET /v1/transactions` | extends each item | + `direction:"in"\|"out"`, `counterparty_name`, `counterparty_phone` (all masked; null for topup/system) | — |
| `GET /v1/transactions/{refCode}` | extended | + the same 3 fields as above | — |
| `GET /v1/transactions/{refCode}/timeline` | extended | + `failure_code?` (only when failed/compensated) | — |

> **Masking (decision in the plan, locked tightly by tests):** `MaskPhone("0912345678") = "091****678"` (keep the first 3 and last 3). `MaskName("Nguyen Van An") = "Nguyen V** A*"` — keep the first word (family name) intact; the following words keep the first letter + `*` for the remaining length. The server only returns already-masked data — the client never sees another person's full name/phone number.

## 4. Schema changes

**None.** Only joins/derives from existing data (`users`, `wallets`, `step_executions.error`).

## 5. Test plan

| Test | Content |
| --- | --- |
| `TestLookup_MaskedAndRateLimited` | Found → correct masked format; user does not exist → 404; calling >20/min → 429 (anti-enumeration) |
| `TestHistory_CounterpartyAndDirection` | A transfers to B: A's history → direction=out + B's masked name; B's → in + A's masked name; topup → counterparty null |
| `TestUsersMe_TxnLimitMatchesTier` | txn_limit matches `wallet.TxnLimit` for all 6 kyc_status values (matches the server truth, not the old FE map) |
| `TestTimeline_FailureCode` | Transfer fails due to insufficient balance at the wallet step → `failure_code=ERR_INSUFFICIENT_BALANCE`; unknown error → `ERR_TRANSFER_FAILED`; completed instance → no field |
| Unit `mask_test.go` | Table-driven: names with 1/2/3 words, unicode; short/long/invalid phone |

## 6. Rollout & risk

- Everything is additive (new fields + new endpoints) — not breaking for existing clients; mobile regenerates `schema.d.ts` via `npm run gen:api`.
- Main risk: **phone enumeration** via lookup → per-user rate-limit + a response that does not distinguish the reason for 404. An audit_log entry is written for every lookup.
- `failure_code` is only mapped from an allowlist of sentinels — never returns the raw internal error (no stack/SQL leak).
- Rollback = revert the commit (no migration).

## 7. Execution checklist

1. This plan + solution-architecture.md → **awaiting approval**
2. T1 mask → T2 lookup → T3 history enrich → T4 /me → T5 failure_code → T6 openapi → T7 tests
3. `sqlc generate` clean → `go build ./...` → `go test ./...`
4. fintech-review → commit (English, `feat:`) → mobile runs `npm run gen:api` and updates the M3 plan (fixed in the same round)

---

hpay-backend · docs/plans/m3-api-support · 18/07/2026 · source: gap analysis of the m3-core-screens plan (mobile)

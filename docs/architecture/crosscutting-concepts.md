---
title: Cross-cutting concepts
type: architecture
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/internal/ledger/ledger.go, src/backend/internal/fee/fee.go, src/backend/internal/api/middleware.go, src/backend/internal/api/errmap.go, src/backend/internal/api/router.go, src/backend/internal/outbox/relay.go, src/backend/internal/auth, src/backend/internal/platform/logger/logger.go, src/backend/internal/offerguard/offerguard.go, src/backend/internal/store/queries]
---

# Cross-cutting concepts

Audit IDs refer to [../audit/AUDIT-REPORT.md](../audit/AUDIT-REPORT.md). Its "Verification appendix" overrides contradictions in that report.

## 1. Money is int64 minor units

All amounts are `int64` VND (no decimals; VND has no minor unit in practice, so 1 unit = 1 dong). Columns are `BIGINT` (`primary_pocket_max`, `daily_limit`, `balance`, `amount`, `fixed_fee`). Fee math: `fixed + amount*percent_bp/10_000`, integer division rounds down (`fee/fee.go:25-26`); no overflow protection (audit "Ledger Post / Fee"). Mobile mirrors this with `bigint` and emits JSON integers without passing through `Number` (`mobile/src/api/client.ts:76-86`), but the engine request still does `Number(amount)` (`mobile/app/(main)/transfer/confirm.tsx:46`), and `schema.d.ts` types amounts as `number`.

## 2. Idempotency

- Scope and binding (ADR-0011, migration 000015): `transactions` has `UNIQUE (initiated_by, idempotency_key)` and a nullable `request_hash` (SHA-256 of a canonical length-prefixed encoding of operation, receiver/target ids, amount, service code, note, extra params as applicable). Helpers in `wallet/idempotency.go`: `RequestHash`, `FindReplay` (user-scoped lookup, hash compare), `IsIdempotencyConflict`. Flow: look up `(user, key)`; none -> insert; same hash (or NULL hash from a pre-000015 row) -> return the original result; different hash -> 409 `ERR_IDEMPOTENCY_KEY_REUSED`. A concurrent insert that loses the race hits the unique index, re-reads and returns the winner. Used by `transfer` (P2P, engine), `wallet.Topup` and `moneyrequest.Pay`.
- Money requests: `money_requests.idempotency_key/request_hash`, per-requester partial unique index for single requests; a split (many rows per key) is serialized with `pg_advisory_xact_lock` and replays all its rows in order.
- The HTTP header is enforced by `RequireIdempotencyKey()` (`api/middleware.go`, 1-64 chars) on `POST /v1/transactions/request`, `POST /v1/money-requests`, `POST /v1/money-requests/split` and `POST /v1/org/:id/transactions`. Not on reads, auth, confirm/verify (state only), decline/cancel/remind, vault create and round-up target (metadata only; vault money moves through the engine). The services reject an empty key as well, so there is no server-generated fallback.
- `internal/org` (`Topup`, `CreateDisbursement`) uses the same `wallet.FindReplay` contract. The legacy query `GetTransactionByIdempotencyKey` is no longer called. Keys never expire.
- Workflow step idempotency: `step_executions` unique `(instance, step, attempt)`; replays re-emit the recorded result (`workflow/consumer.go:109-116`). `MOVE_FUNDS` still has the double-post window because the ledger tx and the step record are separate transactions (BE-C1, `handlers.go:82-122`, `consumer.go:164`).
- Bank callback idempotency: by `ref_code` under `FOR UPDATE` (`wallet/callback.go:69`).

## 3. Locking and concurrency

- Ledger: `GetWalletsForUpdate` selects `... ORDER BY id FOR UPDATE` so all multi-wallet postings lock in id order (`store/queries/wallets.sql:6-9`, `ledger.go:82`); balance changes happen only through `ApplyWalletDelta` (`wallets.sql:33`). Non-system wallets cannot go negative; system wallets can (`ledger.go:98,111`).
- Append-only ledger: DB triggers reject UPDATE/DELETE on `ledger_entries` (migration 000007; verified in CI). Double-entry balance (debit = credit) is validated in Go only (`ledger.ValidateLegs`; audit migration table).
- Workflow: `GetInstanceForUpdate` row lock, status/step/attempt guards drop stale events (`engine.go:90-139`). Recovery leases stuck instances by an UPDATE to `recovering` (`workflow/recovery.go:22-25`).
- Approvals: `GetApprovalRequestForUpdate` plus unique approval action per checker/level; `FinalizeApprovalRequest` is the race arbiter (`approval/service.go:417,497`). KYC review uses a guarded UPDATE (`kyc/admin.go:104-136`).
- Outbox: `FOR UPDATE SKIP LOCKED` (`store/queries/outbox.sql:11`). Claim query orders by `id` while the index is by `available_at` (audit outbox note).
- OfferGuard daily/monthly/interval checks are re-run at confirm under a per-user `pg_advisory_xact_lock` (BUG-0017, `transfer/service.go` `ExecuteEngineConfirm`, `offerguard.CheckWindows`). Requests that need no confirmation are checked only at request time.

## 4. Transactional outbox

Business rows and `outbox` events commit in one tx (`engine.Start(ctx, tx, ...)`, `workflow/engine.go:47-77`; approval finalize `approval/service.go:566`). `outbox.Relay` publishes to Redis Streams and marks rows published in the same tx as the claim (`outbox/relay.go:47-83`); when Redis is down it keeps rows (E05/E17). Delivery is at-least-once; there is no attempts/dead-letter column (poison event can block the head, audit outbox note). Consumers are Redis consumer groups (`wf:steps:<handler>` group `workers`, `wf:events` group `engine`); crashed consumers are reclaimed with XAUTOCLAIM every 30 s.

## 5. Error contract (errmap)

One wire format: `{code, message, request_id}` (`api/errmap.go:27-31,124-132`). `respondErr` walks `errTable` (about 80 domain error to code/HTTP mappings) with `errors.Is`; unmatched errors become `500 ERR_INTERNAL` without internals. `respondValidation` gives `400 ERR_VALIDATION`. Middleware errors use the same shape without `request_id` (`middleware.go:21,48`). Gaps: engine confirm/verify return a plain `errors.New("không tìm thấy giao dịch")` and the "service disabled/under maintenance" error, neither in `errTable`, so they surface as 500 (`transfer/service.go:297,518`). Frontends: admin maps codes to Vietnamese text (`admin/src/api/errors.ts`), mobile uses `src/lib/errors.ts`.

## 6. Auth layers

| Layer | Mechanism | Where |
|---|---|---|
| User access token | HS256 JWT, aud `user`, TTL 15 min, carries user id + device id | `auth/jwt.go:11-51`, `RequireAuth` `middleware.go:16` |
| Refresh token | opaque, 30 days, rotating; reuse of a used token revokes the whole family and is audited (`ERR_TOKEN_REUSED`) | `auth/refresh.go`, `auth/service.go:96-156` |
| Passwords | argon2id (64 MiB, t=3, p=2) | `auth/password.go:14-20` |
| Admin token | same issuer class with aud `admin`, signed with the same `JWT_SECRET`, no refresh, `admin_users` table | `router.go:166`, `auth/admin.go`, `RequireAdmin` `admin_handler.go:25` |
| Org role | `requireRole(admin/maker)` in org service; checker level in approval eligibility | `org/service.go:449`, `approval/service.go:221-250` |
| OTP | 6-digit code, TTL 120 s, max 3 attempts, 30 s resend cooldown; used only for approval decisions | `auth/otp.go:15-17`, `auth/service.go:172-228` |
| Mock bank | shared secret header | `mockbank_handler.go:31` |
| Signed file links | HMAC with `JWT_SECRET`, 5 min expiry; QR payload signed, 15 min; receiver token 10 min | `storage/storage.go:96`, `qr/qr.go:61,107` |

Gaps: access token not revocable and user status not checked per request (BE-H6); no admin RBAC on the backend (every admin token can call every `/v1/admin` route, only frontend hides things; see [../product/roles-and-permissions.md](../product/roles-and-permissions.md)); no server-side PIN (BE-C2); `confirm`/`verify` have an IDOR pattern (BE-C2, only the transaction id is needed).

### One-time codes and security notices (ADR-0016, ADR-0017)
Codes (OTP) and notices to the account holder leave the system only through the `auth.Sender` port. HPay always generates, hashes, counts and expires the codes itself; a provider only delivers them. Selection (`auth.NewSenderFromConfig`): `SENDER_PROVIDER=twilio` uses the Twilio adapter in any environment and refuses, never falls back to log-only, when its settings are incomplete; with no provider, `production` and `demo` use the refusing sender (a flow that depends on delivery fails closed with 503 instead of appearing to succeed) and the other environments use a log-only sender that records that a message was sent and never the code. The Twilio adapter calls the Programmable Messaging API through the guarded gateway client (https only, public addresses only, no redirects, C-10) and never logs the code or the API secret. An OTP value appears in an HTTP response only when `APP_ENV=dev`.

### Second factor: TOTP (ADR-0017)
The 2fa band of the offer auth matrix needs the PIN and a TOTP code. TOTP (RFC 6238) is built in `internal/auth`: the secret is AES-256-GCM encrypted at rest with `TOTP_ENCRYPTION_KEY`, shown once at enrolment, and a code is accepted once (the time step is claimed with a guarded UPDATE). The confirm step asks the `transfer.Authenticator` for the factor, so a further factor (a device-bound biometric, which the mobile app would supply) is a new implementation, not a change to the flow.
## 7. Logging

`log/slog` JSON to stdout (`platform/logger/logger.go`); level Debug with source in `dev`, Info otherwise. Access log per request (`router.go:235-247`): request_id, method, route template, status, duration; `X-Request-Id` is accepted from the client or generated and echoed. Policy (comment in logger.go): never log passwords, OTP, tokens, full phones, balances at info or above. Gap: no metrics or tracing; `/healthz` reports Postgres and Redis (503 when degraded).

## 8. Rate limiting

Redis fixed window `INCR` + `EXPIRE`, key by client IP + route (`RateLimit`) or per user (`RateLimitUser`); fail-open if Redis errors (`middleware.go:39-79`). Applied: register and login 5/min, refresh 30/min, QR resolve 30/min/user, user lookup 20/min/user, approval challenge 10/min, admin login 5/min (`router.go:99-109,160,187`). Not applied: logout, mock-bank callback, `/v1/files`, other admin routes (BE-H8). No `SetTrustedProxies` configured, so `X-Forwarded-For` handling is Gin's default.

## 9. Audit log

Table `audit_logs(actor_id, actor_type, action, resource, reason, ip, created_at)` (migration 000001). Written via `InsertAudit` from: refresh-token reuse (`auth/service.go:114`), KYC review (`kyc/admin.go:146`), approval decisions and expiry (`approval/service.go:481,614`, `approval/admin.go:121`), org create/member/policy (`org/service.go:96,180,261`), fraud resolve (`fraud/admin.go:126`), reversal (`reversal/reversal.go:110`), product service/offer changes (`product/service.go:284`). Read via `GET /v1/admin/audit` (free-text search over action/resource/reason). Gaps: no audit when AI keys or settings change, when signed file links are issued, or on admin login (audit notes BE-Medium).

## 10. Timezone handling

There is no explicit timezone anywhere in the backend: no `time.LoadLocation`, no `Asia/Ho_Chi_Minh`. Day and month boundaries for OfferGuard limits use `time.Now().Location()` (`offerguard.go:92-110`), the recon cron uses server-local 00:15 and "yesterday" (`recon/cron.go:24-29`), fraud rule ODD_HOURS uses an hour-of-day field commented "local" (`fraud/rules.go:30`). In the distroless image the process zone is UTC, so Vietnamese users see limit windows and recon runs offset by 7 hours (audit BE-H2). Fix pending: pin a fixed zone constant. Database timestamps are `TIMESTAMPTZ`.

### Replay safety of ledger postings (ADR-0010)

A posting is identified by (transaction, kind). ledger.Post claims a row in ledger_postings after the ordered wallet locks and before writing entries; a second claim returns ErrAlreadyPosted (same total) or ErrPostingConflict (different total) and changes nothing. Workflow handlers treat ErrAlreadyPosted as success, which makes a redelivered or re-run money step harmless even though the step record and the money movement still commit in different transactions. Code: internal/ledger/ledger.go; table: reference/data-model.md.

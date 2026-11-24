# HPay – Full Project Audit Report

Date: 2026-11-21. Scope: `hpay-backend`, `hpay-admin`, `hpay-mobile`, `hpay-docs`, `ref-docs`.
Method: 4 agents read code in parallel, read-only, no files modified.

> **Limitations.** This report is based on reading code and grep; no tests were run and the application was not run. Any item marked **[unverified]** is a suspicion that needs more reading. Not yet checked:
> - `workflow/engine.go` (only grepped), `gateway.go`, `round_up.go`, `fraud/rules.go`.
> - Endpoint-by-endpoint and field-by-field comparison between `openapi.yaml` and the frontend.
> - The two largest docs files (`00-backend-design-v2-base.html`, `technical-overview.*`), the xlsx files and `ref-docs/*.docx`.
> - Line numbers in the docs section may be inaccurate because grep output was truncated.

---

## Verification appendix (2026-11-21, code re-read)

Results of verifying the **[unverified]** items below. This section **overrides** every contradiction in the report.

| Item | Conclusion | Evidence |
|---|---|---|
| MOVE_FUNDS double-post (BE-C1) | **CONFIRMED**, looked for a guard that would refute it but found none | `handlers.go` ~82-122: separate tx `Begin→ledger.Post→Commit`; `consumer.go:164-212 recordAndEmit` is another tx. `ledger_entries` has only `idx_ledger_txn`, not UNIQUE (000002:83). `PostWithQueries` does not check whether the txn already has entries |
| Compensation fail | **PARTIAL** | `engine.go:301-310`: only logs Error, then sets `failed`. The `wf_status` enum has no `compensation_failed`. This branch does not call `FailTransactionGuarded`, so the transaction may be stuck in `processing`. `ev.Retryable` is ignored. `ClaimStuckInstances` only claims `running`, so an instance stuck in `compensating` is not recovered |
| `dispatchCompensate` Attempt=101 | **CONFIRMED** (cannot be retried) | `engine.go:337`; `consumer.go:111-114` when it sees an existing record it only re-emits the old result |
| SSRF gateway (FIXED in the working tree 2026-11-21, not yet committed: BUG-0014) | **CONFIRMED** | `action_pipeline.go:81-107`: URL comes from `srv.Actions` in the DB, only the `http` prefix is checked; `gateway.go:59` calls it directly. No allowlist, no blocking of private/metadata IPs, no redirect limit. There is a 10s timeout, body max 64KB. Real HTTP is enabled only when `Env=="production"` |
| Org self-approval | **CONFIRMED** | `org_members` has `UNIQUE(org_id,user_id)` (000005:36). `eligibility()` (`approval/service.go:221-250`) has no `checkerID != CreatedBy`. Only the admin flow has it (`admin.go:67`) |
| Idempotency middleware | **PARTIAL** | `RequireIdempotencyKey` is attached only at `router.go:118` and `:156`. It is **not** attached to `/money-requests`, `/money-requests/split`, `POST /vaults`, `PATCH /vaults/:id/round-up-target` |
| `WalletHandler.Topup`, `TransferToUser` | **Dead code** (not mounted) | Downgrade BE-M5 (legacy bypasses offerguard) to Low because it is not reachable |
| asynq / golang-migrate / testcontainers | **Docs wrong** | `go.mod` has only gin, jwt, uuid, pgx, godotenv, go-redis, excelize, x/crypto. The real cron/recovery are in-process goroutines (`main.go:146-151`). Multiple replicas will run duplicates (the recon cron has no lease check). Migrations are run by the external `migrate` CLI (Makefile, CI), not in the server or the Dockerfile |
| Config prod guard | **CONFIRMED** (weak) | `config.go:72` `APP_ENV` defaults to `dev`, not validated. Code compares against `"production"` (`main.go:117`, `seed/main.go:43`) but that value is not in the valid list (dev\|ci\|demo) |
| Mojibake | **REFUTED** | `transfer/service.go` and `router.tsx` are valid UTF-8, no BOM, 0 U+FFFD. It is just the console displaying the wrong codepage. **Remove mojibake from the bug list.** Even so, the error `errors.New("không tìm thấy giao dịch")` is still missing from `errTable` (see trailing note) |
| OpenAPI vs frontend | **PARTIAL**, real mismatch (details below) | `api/openapi.yaml` has 63 paths |

**Endpoints the frontend calls but backend/openapi does not have:**
- Mobile: `/v1/auth/verify-otp`, `/v1/engine/status`, `/v1/money-requests/{id}/pay`, `/v1/wallets/topup`, `/v1/transactions/transfer`.
- Admin: `/v1/admin/fraud/flags` (+`/{id}/resolve`) whereas the backend is `/fraud-flags`; `/v1/admin/kyc/{id}/decision` (backend is `/kyc/:id/review`); `/v1/admin/services/{id}/status` (backend `PUT /services/:id`); `POST /v1/admin/services`; `POST/PUT /v1/admin/offers/{id}` (backend has only GET); `/v1/p2p-notify`.
  - **Status (fixed in working tree, 2026-11-21, not yet committed):** hooks now use the real routes (`/fraud-flags`, `/kyc/:id/review`, `PUT /services/:id`); the dead hooks were removed. `POST /v1/admin/services` and `PUT/POST /v1/admin/offers/:id` have no backend: the UI actions are disabled with a visible reason (backend gaps). `/v1/p2p-notify` is only a sample string in `ServicePipelineModal`, not a call.
- The workflow instance status enum in openapi (~line 1726) lacks `waiting_approval`, `compensating`, `recovering`. Mobile uses a string for amount in `useFeePreview` but the response type is number.
- **Not done:** comparing the transaction/money-request/KYC status enums with the DB, and comparing each response field.

---

## 0. Do immediately (top priorities)

| # | Task | Level |
|---|------|-----|
| 1 | Rotate all secrets in `hpay-backend/.env`: Neon DB password, Cloudinary API secret, JWT secret. The contents appeared in the output of this audit. | Critical |
| 2 | MOVE_FUNDS is not idempotent: redelivery or a crash between 2 txs can debit twice (BE-C1) | Critical |
| 3 | `/transactions/confirm` and `/verify` do not check PIN/OTP and have an IDOR; the request already moves money (BE-C2) | Critical |
| 4 | Idempotency-Key is globally UNIQUE, not per user, and the payload is not matched (BE-H1) | High |
| 5 | Mobile hard-codes PIN `123456` for every transaction and for the app lock | Critical |
| 6 | Admin has no role-based authorization (both backend and frontend routes) | High |
| 7 | `APP_ENV` defaults to `dev`: leaks `otp_dev`, weak secrets are accepted. The mock-bank callback uses a default secret and the route is mounted in every env | High |
| 8 | Missing DB constraints: unique reversal, ledger idempotency, per-user unique for the idempotency key (grouped into migration 000014) | High |

---

## 1. Logic and security vulnerabilities – Backend

### Critical

**BE-C1. MOVE_FUNDS can debit twice** *(FIXED in the working tree 2026-11-21, not yet committed: BUG-0001, ADR-0010)* (`internal/workflow/handlers/handlers.go:82-122`, `workflow/consumer.go:102-161`, migration 000002:70-84)
- `Wallet.Execute` commits `ledger.Post` in one tx, while `step_executions` is written in another tx (`recordAndEmit`).
- If a crash happens between the two txs, or the outbox redelivers (at-least-once), the step runs again and posts to the ledger a second time.
- `ledger.Post` does not check whether `txnID` was already posted. `ledger_entries` has no UNIQUE preventing this. Compensate has the same flaw.
- Fix: write the step claim and the ledger in the same tx, or INSERT a `step_execution` with status `running` first. Add UNIQUE `(transaction_id, posting_kind)` on the ledger.

**BE-C2. Engine confirm/verify is unauthenticated and has an IDOR** *(FIXED in the working tree 2026-11-21, not yet committed: BUG-0012, ADR-0008, ADR-0014)* (`internal/transfer/service.go:515-540`, `internal/api/transaction_engine_handler.go:66-127`)
- `ExecuteEngineConfirm` and `ExecuteEngineVerify` accept PIN, OTP, `SenderUserID` but do not use them. They only look up `GetTransactionByWorkflow` and then return `ref_code`, `amount`, `fee`, `status` of any transaction if the UUID is known.
- `ExecuteEngineRequest` calls `engine.Start` immediately (line ~390), so money moves without PIN/OTP. `AuthRequired` is only a returned label, the server does not enforce it, contradicting the `offer_auth_matrix` spec. The backend has no PIN route at all.
- DB errors are swallowed and converted to "transaction not found". This error string is not in `errTable`, so it returns 500.
- Fix: the workflow stops at the AUTH step waiting for authentication. Confirm verifies PIN/OTP by hash, with rate limiting and lockout after N wrong attempts. Check `initiated_by == user` (404 otherwise).

### High

**BE-H1. Global Idempotency-Key** *(FIXED in the working tree 2026-11-21, not yet committed: BUG-0002, BUG-0003, ADR-0011, migration 000015; residual: `internal/org` still replays by key only, see `docs/specs/w1-idempotency/spec.md`)* (migration 000002:57; `transfer/service.go:141-149,369-386`; `wallet/service.go:121-128`; `moneyrequest/service.go:233-241`)
- If user B reuses A's key, B receives A's transaction result. The same user changing the payload still receives the old result, with no 409.
- When the client sends no key, `ExecuteEngineRequest` generates a UUID itself, so it is not idempotent. A network retry will create a second transaction.
- Fix: UNIQUE `(initiated_by, idempotency_key)` plus a request hash, return 409 if different. Require a key on every money-writing endpoint.
- Additionally: `/money-requests`, `/money-requests/split`, `/vaults` do not appear to have `RequireIdempotencyKey` (`router.go:128-129`). **[verified, then fixed]**: money requests had no idempotency at all (create and split now require the key and replay per requester); `/vaults` and the round-up PATCH only change metadata and were deliberately left without a key.

**BE-H2. Offer limit has a TOCTOU race** **[fixed 2026-11-24, BUG-0017]** (`internal/offerguard/offerguard.go:92-121`, called from `transfer/service.go:315`)
- SUM the volume, compare against the limit, then INSERT in another tx. N parallel requests with different keys all pass. The `IntervalSeconds` throttle can also be bypassed.
- Day/month boundaries use the server timezone, not a fixed Vietnam time.
- Fix: `pg_advisory_xact_lock(hash(user,type))` or lock a counter row in the same tx as the INSERT.

**BE-H3. The double-reversal constraint exists only in a comment** *(FIXED in the working tree 2026-11-21, not yet committed: BUG-0011, ADR-0013)* (`reversal/reversal.go:106,126-172`, `queries/transactions.sql:63-66`)
- There is no `one_reversal_per_txn` or `UNIQUE(subject_type, subject_id)` in the migrations.
- It is currently still blocked thanks to `idempotency_key = "rev-<id>"`, but the error surfaces as a raw 23505 and the approval can get stuck.
- The executor does not check `orig.Status == completed` and ignores the row count of `MarkTransactionReversed`.
- Fix: partial UNIQUE `transactions(reversal_of)`, partial UNIQUE on pending approvals, `FOR UPDATE` on the original transaction and check `n == 1`.

**BE-H4. Reversal/compensate when the receiver has already spent the money** *(reversal part FIXED in the working tree 2026-11-21, not yet committed: BUG-0011, ADR-0013; the workflow compensation part is still open)* (`reversal.go:153-166`, `handlers.go:113-116`)
- `ErrInsufficientBalance` rolls back the whole finalize, the approval is stuck in `pending`. Compensate treats this as a terminal error, the workflow is failed while the money remains with the receiver.
- Fix: add a `reversal_failed` or `needs_manual` status with an audit record. Consider a debt account (receivable) or a hold.

**BE-H5. Money request is marked `paid` too early** (`moneyrequest/service.go:244-256`)
- `paid` is set in the tx that creates the transaction, before MOVE_FUNDS runs asynchronously. If the workflow fails, the request stays `paid`.
- Fix: keep `pending` until the transaction is `completed`, or add a `paying` status and revert to `pending` on failure.

**BE-H6. Access token is not revocable and status is not checked; admin has no RBAC** *(admin part FIXED in the working tree 2026-11-21, not yet committed: BUG-0013, ADR-0015; user access-token revocation and a separate admin signing secret are still open)* (`api/middleware.go:16-34`, `auth/jwt.go`, `api/admin_handler.go:25-42`, `auth/admin.go`)
- A locked user can still use the access token for up to 15 minutes and can still refresh. Logout only revokes the refresh family.
- `RequireAdmin` grants every admin every permission, including `UpdateAISetting`, creating AI keys, reversal. Admin has no refresh. It shares `JWT_SECRET` with users, separated only by `aud`.
- Fix: add jti or a token version and check status (Redis cache). Per-route RBAC by role. Use a separate secret for admin.

**BE-H7. Weak default configuration** *(FIXED in the working tree 2026-11-21, not yet committed: BUG-0010, ADR-0012)* (`platform/config/config.go:64,81,95`, `cmd/server/main.go:117`, `api/router.go:45`, `approval_handler.go:43-45`)
- `APP_ENV` defaults to `dev`. In dev, `otp_dev` is returned in the `/approvals/:id/challenge` response, and a short JWT secret is still accepted.
- `MOCK_BANK_SECRET` defaults to `dev-mock-bank-secret`.
- The valid env list (comment) is `dev|ci|demo`, but `main.go` compares `!= "production"`. Consequence: every env currently uses the Mock gateway, and there is no deliberate way to enable the real gateway.
- `godotenv.Load()` runs in every env.
- `/internal/mock-bank/callback` is mounted in every env, with no rate limit and no IP allowlist. Anyone who knows the default secret can top up fake money.
- Fix: `APP_ENV` is mandatory and whitelisted. Forbid default secrets outside dev. Mount mock-bank only when not production. Load `.env` only in dev.

**BE-H8. Weak brute-force protection and rate limiting** (per-account lockout and timing fixed, BUG-0016; the rest below still open) (`api/middleware.go:39-54`, `router.go:99-102`, `auth/service.go:73-92`)
- Rate limit is per IP + route, 5 per minute. No per-account lockout, no counting of wrong attempts.
- No `SetTrustedProxies` seen, so X-Forwarded-For can be spoofed. The rate limiter fails open when Redis fails.
- `/auth/logout`, mock-bank, `/v1/files`, `/v1/admin/*` have no rate limit.
- Login with a non-existent phone returns early (no hashing), so timing leaks user enumeration, including admin login. Register exposes `ErrPhoneTaken`.

**BE-H9. KYC upload and national ID (CCCD) images** (`kyc/service.go:41-53`, `kyc_handler.go:58`, `storage/cloudinary.go`, `storage/storage.go:119-141`, `api/admin_handler.go:141-154`)
- Only the first 2-4 magic bytes are checked, the image is not decoded. The key is always `id_card.jpg` even for PNG. No `MaxBytesReader`, only `LimitReader`.
- Cloudinary upload defaults to `type=upload`, which is public by URL. Anyone who knows the URL can download the CCCD image without the system's HMAC signature.
- The signed URL is signed with `JWT_SECRET` itself, is not tied to any admin, and there is no audit when a link is issued. `ServeFile` lacks `X-Content-Type-Options: nosniff`.
- Orphan files are not cleaned up (the stored requestID differs from the DB row ID).
- Fix: upload as `authenticated/private`, use a separate signing key, add nosniff and audit, `image.DecodeConfig` with a pixel limit, `MaxBytesReader`.

### Medium

- **Self-approval is not explicitly blocked** (`approval/service.go:221-251`). `eligibility()` only checks the checker role and level. It depends only on UNIQUE `(org_id, user_id)` (migration 000005 needs verification). Should add `checkerID != req.CreatedBy`.
- **Balance/limit is not re-checked at MOVE_FUNDS** after approval. `orgTxnLimit` is only checked at creation. **[unverified]**
- **`human_approval` is attached only to org wallets.** A user-wallet service that has this step will fail (`approval/service.go:94-99`).
- **AdminDecide** (`approval/admin.go`): no OTP, no rate limit, no RBAC, no IP recorded. KYC review needs only 1 admin, the reject reason is optional.
- **AI key stored in plaintext** in `ai_keys.api_key`. No audit when creating/changing a key or setting.
  - `admin_ai.go:127` compares the string `"no rows in result set"` instead of `errors.Is(pgx.ErrNoRows)`.
  - `UpdateAIKey` missing the `is_active` field silently deactivates the key.
  - `advisor.New("")` silently returns a mock.
  - Model `claude-sonnet-5` is hardcoded, ignoring `model_name` in the DB.
  - Sending CCCD images to a third-party LLM needs legal consideration.
- **Ledger `Post`** does not check wallet `status` and `currency` (`ledger.go:83-99`). No int64 overflow protection (`ledger.go:39-59`, `fee.go:26`). `ExecuteEngineRequest` does not apply `wallet.TxnLimit`.
- **Fee**:
  - Rounds down, not settled in the spec.
  - `fee_policies` lacks CHECK and UNIQUE `(txn_type, kyc_status, version)`.
  - Does not block `fee > amount`.
  - `Calc` swallows negative values into 0.
- **Outbox** (`outbox/relay.go:63-83`): at-least-once, no dead-letter or attempts. A poison event blocks the head of the queue. The index is ordered by `available_at` but the query is `ORDER BY id`. `PumpLocal` marks published before processing (dev/test only, should be blocked in production).
- **`TransferToUser` (legacy) bypasses offerguard** (`transfer/service.go:88-160`). If the endpoint is still open, limits can be bypassed.
- **Workflow**:
  - Need to confirm the `compensation_failed` status and alerting (`engine.go:301-310`).
  - `dispatchCompensate` uses a fixed Attempt of 101. **[unverified]**
  - `consumer.go:77` uses `time.Sleep(1s)` instead of `select ctx.Done()`.
  - A network error when downloading the KYC image is treated as terminal, with no retry.
- **Fraud** (`fraud/handler.go`): `HourOfDay` uses server time. The advisory goroutine overwrites the `reasons` column, which can lose data.
- **HTTP server and headers**:
  - Missing Read/Write/IdleTimeout (`main.go:77-81`), CORS, HSTS, nosniff, global body limit.
  - `healthz` returns the Postgres/Redis `err.Error()` to strangers.
- **Refresh token**: does not check `users.status`. `X-Device-Id` is sent by the client and not bound. No limit on the number of families/devices. Audit is swallowed with `_ =`.
- **OTP**: hash is bound to `userID:purpose` instead of the challenge id. No total limit per user. `Decide` burns the OTP before checking eligibility.
- **User lookup** allows enumerating phone numbers (20/minute/user, registration is free).

### Low

- **Seed has no environment guard.** `.env` currently points to the real Neon DB. Seed creates a fixed password `Demo@123` for demo users and `E2ETest@123` for 2 e2e admins, so running it by mistake would create admins with known passwords (`cmd/seed/main.go`).
- `docker-compose.yml`: Redis has no password, port exposed to the host, `JWT_SECRET` missing for the app profile.
- `.gitignore` has no `data/` (local KYC images could be committed later).
- `NewRefCode` is 48 bits, ignores the `rand.Read` error, no retry on collision.
- `vault.go:74-76` truncates the label by byte, corrupting Vietnamese UTF-8. No validation of `target < 0`.
- `qr` receiver_token can be reused for 10 minutes. `hmac.Equal` not confirmed.
- `recon.go`: `buildReport` and the LLM run before `ErrAlreadyRan` is received. `day` is not normalized to a timezone.
- Passwords: no complexity requirement. Login does not cap length (hash DoS risk).
- Errors swallowed here and there: `uuid.Parse` errors ignored in handlers, `json.Unmarshal` error ignored (`approval/service.go:126`).
- `otp_challenges` has no cleanup job.
- There is a `WalletHandler.Topup` function but no mounted route is seen (orphan function or removed route). **[unverified]**

### Migration / DB – missing constraints (grouped into migration 000014)

| Issue | Location |
|---|---|
| UNIQUE `(initiated_by, idempotency_key)` in place of the global UNIQUE | 000002:57 |
| `ledger_entries`: missing UNIQUE against duplicate posting and CHECK `wallet_id OR account_id NOT NULL` | 000002:67-80 |
| Debit = credit balance is checked only in Go. Needs a DEFERRABLE constraint trigger | 000002 |
| `transactions`: missing CHECK `sender <> receiver` | 000002:51-64 |
| `transactions.reversal_of`: missing partial UNIQUE | 000002:60 |
| `approval_requests`: missing UNIQUE per subject (pending) | 000005:65-85 |
| `fee_policies`: missing CHECK and UNIQUE | 000004:47-57 |
| `wallets.currency` has no CHECK, `ledger_entries` does not store currency | 000002 |
| Missing index `(initiated_by, type, created_at)` for offerguard/fraud | `offer_guard.sql:22-35` |
| `outbox`: missing `attempts/last_error`, index does not match `ORDER BY id` | 000003:69-78 |

Points confirmed to be correct:
- Wallet locking in id order.
- CHECK `balance >= 0` for non-system wallets.
- Append-only trigger (000007).
- State transition guard.
- `GetTopupByRefCodeForUpdate`.
- No floats used for money.
- Approve/cancel/double-approve races handled well (`FOR UPDATE` + UNIQUE action).
- Refresh rotation has reuse detection.

---

## 2. Vulnerabilities – Admin (`hpay-admin`) and Mobile (`hpay-mobile`)

### Critical / High

1. **PIN `'123456'` hard-coded for every transaction** (CRITICAL):
   - `app/(main)/transfer/confirm.tsx:53`.
   - `src/queries/useTransactionQueries.ts:155,166,179`.
   - `src/queries/useVaultQueries.ts:81`.
   - The app sends a fixed PIN by itself. Combined with BE-C2, transaction authentication is currently void at both ends.
   - _Fixed in working tree, 2026-11-21, not yet committed (client side only):_ literals removed, user is asked via `usePin()`; server verification (ADR-0008, BE-C2) is still missing, so this is NOT closed end to end.
2. **App lock compares a hard-coded PIN on the client** (HIGH): `SecurityWrapper.tsx:111-112` (`pinInput === '123456'`, comment "Mock"). No lockout. Biometrics only call `setLocked(false)`.
   - _Fixed in working tree, 2026-11-21, not yet committed:_ no client PIN; OS biometric/device-credential unlock only; sign-out after 5 failed attempts; server verification is a TODO (ADR-0008).
3. **Transfer double-submit** (HIGH):
   - The guard `transfer.isPending` uses the `useTransfer` mutation, while the actual submit uses `engine.requestTransaction/confirmTransaction`, so `isPending` is always false.
   - The `useMemo` key is regenerated on remount. The confirm step has no idempotency key.
   - Fix: lock the button with `isRequesting || isConfirming` plus a `useRef` flag. Store the key per transaction id.
   - _Fixed in working tree, 2026-11-21, not yet committed:_ `useSubmitGuard` (ref flag) on every money action and a persisted key per intent (`lib/intentKey.ts`).
4. **Admin does not block routes by role** (`router.tsx:36-56`, `RequireAdmin.tsx`):
   - Only checks for a token. A viewer can reach `/orgs`, `/ai-settings`, `/services`, `/offers`, `/recon`, `/audit`.
   - `can()` is used only in `KYCDetailPage` and `MyTasksPage`. The permissions `recon.run`, `orgs.edit`, `ai.manage`, `fraud.resolve`, `admins.manage` are declared but unused. Combined with BE-H6.
   - **Status (fixed in working tree, 2026-11-21, not yet committed):** a `RequirePermission` wrapper guards `/orgs`, `/ai-settings`, `/services`, `/offers`, `/recon`, `/audit`, `/fraud`, and action buttons use `can()` (`fraud.resolve`, `recon.run`, `orgs.edit`, `ai.manage`). UX only: the backend still has no role enforcement (BE-H6), so this is not a security control.
5. **Mock data hides real errors** (HIGH):
   - `useReconQueries.ts:107-112,140-143` (fallback `?? 2173`, `duration_ms || 4200`).
   - `useAIQueries.ts:20-79` (fake strings that look like API keys).
   - `useOrgsQueries.ts`, `useAuditQueries.ts:54`.
   - `DashboardPage.tsx:7-17` (KPIs entirely mock).
   - `catch {}` swallows even 401/500 and returns fake numbers. The operator thinks they are looking at real numbers. Enable mock only when `import.meta.env.DEV`.
   - **Status (fixed in working tree, 2026-11-21, not yet committed):** mock data now requires `import.meta.env.DEV` and `VITE_USE_MOCKS=true` (loaded from `src/mocks/*`, absent from the production bundle); empty `catch {}` fallbacks removed from recon, AI, orgs, audit, fraud, workflows, services; missing numbers show an em dash; failed writes are failures. Dashboard KPIs show an em dash in production.
6. **Admin JWT** in `sessionStorage`, no refresh (MEDIUM). Consider an httpOnly cookie and CSP.
   - **Status (fixed in working tree, 2026-11-21, not yet committed):** a Content-Security-Policy meta tag is injected into the production build (script-src self). Token storage and refresh are unchanged.

### Medium

- **`Number()/parseInt()` on amounts** violates convention M06 (`money.ts` uses BigInt): `confirm.tsx:45`, `vaults/[id].tsx:60`, `vaults/index.tsx:43`, `org/admin.tsx:145`, `org/disburse.tsx:43`.
  - **Status (fixed in working tree, 2026-11-21, not yet committed):** admin part fixed (`OfferDetailPage`, `OrgsPage`): integer parsing with validation and upper bound via `src/admin/src/lib/money.ts`. The mobile locations listed here are NOT fixed by this change.
  - _Fixed in working tree, 2026-11-21, not yet committed:_ bigint parsing with an upper bound (`parseAmountStrict`), bigint to the API, lint rule and jest guard.
- **Admin does not use Idempotency-Key** for write operations (approving workflows, resolving fraud, running recon).
  - **Status (fixed in working tree, 2026-11-21, not yet committed):** admin mutations now send one Idempotency-Key per user action and disable while pending. The backend ignores the header on admin routes, so this is client readiness only.
- **Mobile refresh clears the session on a transient error** (`refresh.ts:31-40`): every `!res.ok` including 5xx/429 clears the refresh token. It should clear only on 401/403.
  - _Fixed in working tree, 2026-11-21, not yet committed:_ only 401/403 clear; network, 5xx and 429 keep the session.
- `OfferDetailPage.tsx:181-214`: `Number(e.target.value)` with no validation of negative/NaN. `OrgsPage.tsx:146` divides `/1000000` as a float.
  - **Status (fixed in working tree, 2026-11-21, not yet committed):** fixed (validated integer inputs; save disabled because the backend has no offer write route). `OrgsPage` float division replaced by integer `formatMillions`.
- `kyc-draft.ts` stores the draft in SecureStore (size limit; need to check whether it stores images). **[unverified]**
  - _Checked 2026-11-21:_ it stores only short file URIs, not image bytes; no change needed (see `docs/architecture/mobile-app.md`).

### Low / infrastructure

- `hpay-mobile/app.config.ts:10`: env `dev` points to the real server `https://hpay.demaciahieu.store`. Staging/demo are the placeholder `HPay.example.com`. Missing `ios.bundleIdentifier` and `android.package`.
  - _Fixed in working tree, 2026-11-21, not yet committed:_ dev uses `EXPO_PUBLIC_API_URL` (default `http://localhost:8080`); identifiers `com.hieumai1507.hpay` added.
- `hpay-admin/e2e/.auth/sessions.json` contains real JWTs of the e2e accounts (already in `.gitignore`), should be deleted after running. The e2e password is hard-coded in `e2e/fixtures/auth.ts`.
  - **Status (fixed in working tree, 2026-11-21, not yet committed):** the e2e password is now read from `E2E_ADMIN_PASSWORD` (default is the backend seed value, e2e folder only). `sessions.json` still has to be deleted manually after a run.
- Stray files to clean up: `hpay-backend/hpay-server-local.exe`, `integration_test.exe`, `hpay-backend/tmp/go-build*`, `hpay-docs/push-to-github.ps1`.
- `ServicePipelineModal.tsx:66`: sample URL `https://api.bankpartner.vn/...`. Check whether it is a default sent to the backend.
  - **Status (fixed in working tree, 2026-11-21, not yet committed):** it is display-only sample data inside the pipeline preview and is never sent; there is no `/v1/p2p-notify` call. The modal save button was a fake success and is now disabled (no backend endpoint).
- Admin `api/client.ts`: non-JSON (HTML 502) still returns `res.text()` cast to `T`.
  - **Status (fixed in working tree, 2026-11-21, not yet committed):** fixed: a non-JSON response is an `ApiError` (`ERR_NON_JSON_RESPONSE`); tests in `src/admin/src/api/client.test.ts`.
- `pages/Placeholder.tsx` is suspected dead code.
  - **Status (fixed in working tree, 2026-11-21, not yet committed):** removed (grep showed no importer).
- **Mojibake (UTF-8 read incorrectly)** seen in many files via PowerShell: `router.tsx`, `LoginPage.tsx`, `transfer/service.go:426-540`, README. **[unverified]** It may only be a terminal display issue. It needs to be opened in a UTF-8 editor. If the files really are wrong, Vietnamese error messages would show corrupted text.

Good points: the mobile access token lives only in memory, refresh is in SecureStore, refresh single-flight has a test, `money.ts` uses BigInt, mobile has an Idempotency-Key for transfer/request/split/topup/vault/disburse.

---

## 3. Information from other projects / traces of reference sources

| Location | Content | Proposal |
|---|---|---|
| `hpay-docs/README.md:16,28`; `architecture/06-service-engine-api-consolidation.html:27` | "reference wallets / MoMo", "HPay Service Engine". Used as HPay's own standard name | Rename to "HPay Service Engine". Move the reference part into a "Reference sources" section |
| `hpay-docs/index.html:287` | Footer: "a corporate authorization matrix, an external retail wallet flow overview" | Separate from the HPay spec |
| `architecture/00-backend-design-v2-base.html:27-30,57-62` | "learned from a reference approval engine"; `engineVerifyTransaction`, `VERIFY_TRANSACTION` are names from another system | Rename or state clearly that it is reference |
| `architecture/04-feature-extensions.html:2,27,90,95` | "a reference retail wallet… production in Thailand", PromptPay, Node-RED, DipChip, Sails.js/MongoDB/AngularJS | Move into a comparison appendix |
| `planning/master-plan.html:52,103,122`; `master-plan.txt:18,62,76` | "source a reference wallet backend (real code read)", "claim-lease learned from the reference wallet" | Remove the internal repo name if it is not yours |
| `hpay-backend/internal/approval/service.go:150` | Comment "the reference engine's dry-run idea" | Fix the comment |
| `README.md:21`; `04-feature-extensions.html:25` | "US wallet" (Vietnamese phrase in the docs) but the currency is VND | State explicitly "Venmo/Cash App-style UI" |
| `guide/functional-overview.en.html:285-286` | Tier 1/2 say "**$200**", the same threshold in both tiers. The `.vi` version says 5 million | Change to 5,000,000 VND |
| `hpay-docs/push-to-github.ps1` | Commit message says "34 HTML mockups" (README says 42). It says itself "delete this file after running" | Delete the file |
| `ref-docs/<external wallet flow overview>.html` | Document of another system (correctly placed in ref-docs) | Make sure it is not put on public docs |

Conclusion: no foreign stack (Laravel/Django/Stripe/MySQL) was found outside the reference-wallet comparison. Module path `github.com/hieumai1507/...` matches the repo owner.

---

## 4. Contradictions in the docs and between docs and code

| # | Contradiction | Level |
|---|---|---|
| 1 | README says "5 Core Migrations 000001-000005"; `session-prompt.md:109` says 11 migrations; `:116` mentions "000015". The code has 13 migrations (`000001`–`000013`), no `000015` | High |
| 2 | KYC limits differ in every place: mockup `01-auth-kyc.html:118` says 100 million/day; seed `primary_pocket_max` L1/L2 is 50 million / 200 million; `offer_services` defaults to 500 million/day, 2 billion/month; doc 05 says 100M/txn, 500M/day, 2 billion/month | High |
| 3 | Doc 00 `txn_type`/`txn_status` lacks `processing_async` (the code has it). Seed fee uses `'disbursement'`, the doc says `org_disbursement` | Med-High |
| 4 | `approval_subject` in doc 04: `org_transaction\|admin_reversal\|account_lock\|policy_change`; the code has only `('org_transaction','reversal')`. `approval_status` lacks `cancelled` (migration 000011) | Med |
| 5 | Docs 00/01/04 and `index.html` say **asynq** for cron/recover_stuck, but `go.mod` (the direct require section that was read) shows no `asynq`, `golang-migrate`, `testcontainers`. **[unverified]** Need `Select-String go.mod -Pattern "asynq\|migrate\|testcontainers"` | Med |
| 6 | Edge cases: "E01–E18" (`README:13`, `index.html:272`) and "E01–E20" (many places). "37 cases" whereas E(20)+M(12)+A(7) = 39 | Low-Med |
| 7 | Mockup count: README 42 screens, push script 34. `07-admin-offer-service.html` is not in the README and is missing the corresponding zip | Low |
| 8 | Endpoints: doc 00 says `/auth/register`, `/auth/login` without `/v1`. The relationship between `verify` and `confirm` is unclear | Med |
| 9 | Storage: `01:128` says local/MinIO, `.env.example` has `STORAGE_PROVIDER=cloudinary` | Med |
| 10 | Roles: docs use "L1/L2" but `org_role` has only admin/maker/checker. Tier is stored in two places (`approval_policies.tiers` JSONB and `approval_policy_tiers.tier_level`) and can drift | Med |
| 11 | Phase: `session-prompt.md:120` mentions F11, `master-plan` has only F1–F10 | Low |
| 12 | `push-to-github.ps1`: "same as the other 3 repos" but docs is the 4th repo | Low |

---

## 5. Logic gaps in the design (docs have not defined them)

1. **Idempotency**: scope (per user or global), what to do with the same key and a different payload, how long a key lives.
2. **request / confirm / verify**: missing state model.
   - How long until `request` expires.
   - `confirm` called twice, `confirm` after expiry.
   - Whether the fee is locked at `request` or recomputed at `confirm`.
   - Where `verify` sits for large limits.
3. **Reversal**:
   - The receiver does not have enough balance to refund.
   - Whether the fee is refunded, partial or full refund, refunding twice.
   - Maximum time limit for a refund.
   - The docs say reversal goes through maker-checker but F9 is not done.
4. **Maker-checker**:
   - Whether the Maker ≠ Checker rule is enforced by the DB or only in code. Tier 2 needs two different people.
   - Whether money is held while awaiting 24h approval.
   - A checker deleted or locked midway, an approval expiring exactly while being decided.
   - "5 challenge fails → lock approve for 15 minutes": locked per user or per approval (per approval allows DoS of the checker).
5. **Ledger**:
   - Fixed lock order to avoid cross deadlocks (E02).
   - The `system` wallet is allowed to go negative, doc §1 says "balance ≥ 0" generally.
   - TRUNCATE and a separate DB privilege for the app role (the append-only trigger does not block it yet).
   - No job yet to reconcile `wallets.balance = SUM(ledger_entries)` and alert.
6. **Fee**: rounding, who pays the fee, whether the fee is refunded on fail/reversal, which takes priority when multiple rows match (`kyc_status` NULL).
7. **KYC**:
   - Difference between `auto_approved` and `verified` and the AI score threshold.
   - Downgrading KYC while there are transactions exceeding the new limit.
   - How the day/month window is computed (which timezone, whether pending/failed count).
8. **Money request / split**: recipient locked, expiring exactly while being paid (E19), rate limit on `remind`.
9. **Fraud**: the gray zone 40–70 calls the LLM but the behavior on LLM error/timeout (fail-open or fail-closed) is not recorded, and whether a flag blocks the transaction or only marks it.

---

## 6. Improvement suggestions

**Missing tests (priority)**
- Post to the ledger twice with the same `txnID`.
- Crash-recovery of MOVE_FUNDS (failure between Commit and `recordAndEmit`, then replay).
- Offerguard race (N goroutines, same user).
- Idempotency across users and the same key with a different payload.
- Concurrent reversal, and reversal when the receiver is out of funds.
- IDOR on confirm/verify, wrong PIN.
- Expired/forged signed URL, path traversal in `Local.Get`.
- Property-based tests for `fee.Calc`.

**Architecture**
- Split `ExecuteEngineRequest` (~140 lines). Extract the duplicated idempotency logic in transfer/wallet/moneyrequest into a helper.
- Replace `strings.Contains` for recognizing errors with error codes or `errors.Is` (`transfer/service.go:197-203`).
- Move the hardcoded `TxnLimit` (`wallet/service.go:33-42`) into a policy table.
- Standardize the Asia/Ho_Chi_Minh timezone for every day/month computation.
- Move fire-and-forget goroutines (`fraud/handler.go:97`) into a worker with graceful shutdown.
- Map `pgx.ErrNoRows` to 404 and extend `errTable`.

**Frontend**
- A shared `useSubmitGuard` hook for every money action.
- Pass bigint instead of `Number()`. Add a lint rule forbidding `Number(`/`parseInt(` on amount variables.
- Use `openapi-fetch` with `schema.d.ts` so the compiler catches contract mismatches.
- Separate mock from the production build. Add CSP for admin.

**Operations**
- Run `govulncheck`, `gosec`. Run `gitnexus analyze --pdg` then `explain` to review taint.
- Add `data/` to `.gitignore`. Clean up `.exe` files, `tmp/`, `sessions.json`.
- The project is not a git repo (`D:\hpay` has no `.git`), so `.gitignore` currently protects nothing. Should `git init` and check before pushing.

---

## 7. Proposed order of handling

1. **Today**: rotate secrets (item 0.1). Delete `sessions.json`, `.exe`.
2. **Before going live**: BE-C1, BE-C2, BE-H1, BE-H2, BE-H3. Migration 000014 groups the missing constraints. Remove the mock PIN in mobile.
3. **Next**: admin RBAC (backend + frontend routes), constrain `APP_ENV` and secrets, patch Cloudinary/KYC, explicit self-approval, BE-H5.
4. **After that**: fix the docs (sections 3, 4), add the "invariants and exceptions" section (section 5), add tests (section 6).
5. **Needs further verification**: the **[unverified]** items, compare `openapi.yaml` with the frontend, read `engine.go`/`gateway.go` (SSRF via service URLs configured in the DB), check file encoding.

---

Note on retained Vietnamese text: the code string `errors.New("không tìm thấy giao dịch")` (Vietnamese for "transaction not found") is a genuine code literal and is kept as-is in the verification appendix; it is the only Vietnamese text remaining in this file.

[← F10](../f10-hardening/implementation-plan.md) · m4-api-support · [solution-architecture →](solution-architecture.md)

# M4 API Support — Implementation Plan

## 1. Scope & non-goals

> **Contract-first for mobile M4 (Request/Split + QR):** openapi.yaml changes first, and mobile generates its client from it. QR principle per doc 02 §5: **the QR carries a payload signed with HMAC server-side, the client does not trust a raw QR** — reusing exactly the `storage.Signer` mechanism (HMAC-SHA256 + TTL, verify-signature-before-expiry) that has been running since F8. The money-requests APIs have been sufficient since F7 — the M4 Request/Split part uses them as-is, nothing changes.

| Ships | Not done in M4 (backlog) |
| --- | --- |
| **Receive-money QR** (mandatory, M4-T8): `GET /v1/users/me/qr` → {payload, signature, expires_at} — payload is base64url JSON {user_id, name_masked, phone_masked, exp}, signed with an HMAC server secret, TTL 15 minutes; `POST /v1/qr/resolve` {payload, signature} → verify sig BEFORE expiry (does not leak information about a fake QR) → {name_masked, phone_masked, receiver_token, expires_at} — `receiver_token` is a short-lived HMAC token (10 minutes, purpose `qr-receiver`) representing the recipient's user_id; **transfer accepts receiver_token**: the transfer body adds `receiver_token` (replacing `receiver_phone`, exactly 1 of the 2) — the handler verifies the token → resolves user_id server-side → existing transfer flow. Errors: `ERR_QR_INVALID` / `ERR_QR_EXPIRED` / `ERR_VALIDATION` (missing/extra receiver). **Payment reminder** (nice-to-have, mockup 29): `POST /v1/money-requests/{id}/remind` — requester only, the request is still `pending`, 24h guard via a guarded UPDATE on the new column `last_reminded_at` (migration 000019); inbox/sent/detail additionally return `last_reminded_at` so mobile can display "Reminded at…". Errors: `ERR_REMIND_TOO_SOON` (429). | **Notifications infra** (table + push): remind only bumps `last_reminded_at`, the payer sees it when opening the inbox (polling per doc 02 §6 — no websocket). **Pay-by-counterparty-token** from history (M3 divergence backlog): the `receiver_token` mechanism built in this phase is exactly the foundation — later, history only needs to return an additional token with the same purpose, and transfer does not have to change again; recorded in the backlog, not done. **Vaults / round-up** (mockups 30–31): the master plan does not assign them to any M — a pure backlog, untouched. **Dynamic QR with an amount** (amount in the QR): the mockup only needs a static receive-money QR. |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | openapi.yaml: 3 new endpoints (`/v1/users/me/qr`, `/v1/qr/resolve`, `/v1/money-requests/{id}/remind`) + transfer body adds `receiver_token` (oneOf with receiver_phone) + money-request responses add `last_reminded_at` + 3 new error codes | api/openapi.yaml | M | — |
| T2 | gen-migration 000019: `ALTER TABLE money_requests ADD COLUMN last_reminded_at TIMESTAMPTZ` (nullable, no backfill) | db/migrations/000019_* | S | — |
| T3 | `internal/qr`: Service{signer} — `Mint(user)` (payload+sig, TTL 15'), `Resolve(payload, sig)` (verify sig→exp→parse, returns masked + receiver_token purpose `qr-receiver` TTL 10'), `VerifyReceiverToken(token)` → user_id. Reuses `storage.Signer.mac` via a small interface (additionally export generic `Sign(msg string, ttl)`/`VerifyMsg` on Signer — no change to existing SignURL/Verify behavior). Unit tests: forged sig, expired, tampered payload, token with wrong purpose | internal/qr/*, internal/storage/storage.go | M | — |
| T4 | API handlers + router: `GET /v1/users/me/qr` (RequireAuth), `POST /v1/qr/resolve` (RequireAuth + RateLimitUser 30/min against probing), errmap adds `ERR_QR_INVALID`/`ERR_QR_EXPIRED` | internal/api/qr_handler.go, router.go, errmap.go | M | T3 |
| T5 | Transfer accepts a token: `transferReq` adds `receiver_token` (validate exactly 1 of 2); the handler verifies the token → user_id → `svc.TransferToUser(ctx, senderID, receiverID, ...)` — small refactor: split the part of `Transfer` after `GetUserByPhone` into `TransferToUser`, and `Transfer` (phone) calls it. **Run impact on `Transfer` before editing** | internal/transfer/service.go, internal/api/transfer_handler.go | M | T3 |
| T6 | Remind: query `RemindMoneyRequestGuarded` (UPDATE ... SET last_reminded_at=now() WHERE id=$1 AND requester_id=$2 AND status='pending' AND (last_reminded_at IS NULL OR last_reminded_at < now()-interval '24 hours') RETURNING — atomic, no race); the service distinguishes not-found/forbidden/not-pending/too-soon; handler + route; inbox/sent/detail queries return the extra column | internal/store/queries/money_request.sql, internal/moneyrequest/service.go, internal/api/request_handler.go | M | T2 |
| T7 | Integration tests: QR mint→resolve→transfer-by-token end-to-end (money arrives in the right wallet); QR forged/expired; remind the 1st time OK + 2nd time within 24h → ERR_REMIND_TOO_SOON + 2 concurrent reminds, only 1 succeeds; remind by a non-requester → 403 | test/integration/qr_test.go, moneyrequest_test.go | M | T4,T5,T6 |
| T8 | Docs: update the plan if it drifts; note the backlog (counterparty-token, vaults) in the README backlog section | docs/plans/m4-api-support/*, README.md | S | T7 |

## 3. Schema changes

Migration 000019 (gen-migration): `ALTER TABLE money_requests ADD COLUMN last_reminded_at TIMESTAMPTZ;` — nullable, down = DROP COLUMN. No enum change, no new table. Note the sqlc lessons: a nullable timestamp column → `pgtype.Timestamptz`, no COALESCE needed (only a nullable uuid has to be wrapped).

## 4. API changes

| Endpoint | Change | Errors |
| --- | --- | --- |
| `GET /v1/users/me/qr` | New — {payload, signature, expires_at}; TTL 15 minutes | — |
| `POST /v1/qr/resolve` | New — verify + return {name_masked, phone_masked, receiver_token, expires_at}; RateLimitUser 30/min | `ERR_QR_INVALID` 400 · `ERR_QR_EXPIRED` 410 |
| `POST /v1/transactions/transfer` | Body adds `receiver_token` (oneOf with `receiver_phone`) | `ERR_QR_INVALID`/`ERR_QR_EXPIRED` when the token is corrupt/expired; `ERR_VALIDATION` when both or neither |
| `POST /v1/money-requests/{id}/remind` | New — the requester reminds the payer, once per 24h per request | `ERR_REMIND_TOO_SOON` 429 · `ERR_REQUEST_NOT_FOUND` · `ERR_FORBIDDEN` · `ERR_REQUEST_NOT_PAYABLE` |
| money-requests inbox/sent/detail | Response adds `last_reminded_at` (nullable) | — |

## 5. Test plan

| Test | Content |
| --- | --- |
| `TestQR_MintResolveTransferEndToEnd` | u2 mints QR → u1 resolves (receives masked + token) → transfers by token → drain → money arrives in u2's wallet, txn completed |
| `TestQR_ForgedAndExpired` | modify 1 byte of the payload → ERR_QR_INVALID (does not leak expiry); QR past TTL → ERR_QR_EXPIRED; a token used as a file-sig and vice versa → invalid (purposes are separated) |
| `TestRemind_RateLimit24h` | remind OK → remind again → ERR_REMIND_TOO_SOON; 2 goroutines reminding concurrently → exactly 1 succeeds (guarded UPDATE) |
| `TestRemind_AuthZ` | payer (not the requester) reminds → ERR_FORBIDDEN; request already paid → ERR_REQUEST_NOT_PAYABLE |
| Unit `internal/qr` | sig/exp/parse boundaries as in T3 |

There are no new E codes (E01–E20 were closed in F10; M-cases belong to the mobile repo). Tests are placed by the usual convention, integration in `test/integration/`.

## 6. Rollout & risk

- **Separated token purpose:** receiver_token and file-sig share the same Signer but the message has a purpose prefix (`qr-receiver:` vs the file key) — a token of this kind cannot open a file and vice versa; covered by tests.
- **Privacy:** resolve only returns masked data (reusing m3's `user.MaskName/MaskPhone`); the QR payload also only contains masked data + user_id — user_id appearing in the QR is acceptable because the QR is shown by the owner, and no API accepts a raw user_id to transfer money (a signed token is required).
- **Entirely additive:** the transfer body is extended with oneOf — an old M3 client sending receiver_phone still works unchanged; the migration is nullable with no backfill; a half-way merge breaks nothing.
- **Remind spam:** the 24h guard sits in SQL (atomic), plus RateLimitUser on the route to prevent id probing.

## 7. Execution checklist

1. gen-feature-plan ✔ (this document) → **awaiting approval**
2. T1 openapi → T2 gen-migration 000019 → sqlc generate clean → build
3. T3 internal/qr (+ generic Signer, run impact on Signer) → T4 handlers → T5 transfer (impact on Transfer) → T6 remind
4. T7 tests (go test -p 1) → fintech-review → verify on your machine → commit

---

m4-api-support · implementation plan v1 · 18/07/2026

[← F7](../f7-request-money/implementation-plan.md) · f8-kyc · [solution-architecture →](solution-architecture.md)

# F8 KYC + AI — Implementation Plan

## 1. Scope & non-goals

> **Key point:** KYC runs as a **workflow definition** (`kyc_verification`) on the very same F4 engine — the first time an instance is used that is NOT attached to a transaction (the engine has supported `transaction_id = NULL` since F4). The `kyc_requests` schema exists since F1. What is genuinely new: a storage interface for images, a Claude vision scorer + mock fallback, and **minimal admin auth** (login + aud=admin middleware) because master plan F8 requires an "admin review API" yet admin_users has no login path.

| Ships | Not done in F8 |
| --- | --- |
| Storage interface + local-dir driver (images stored in `KYC_STORAGE_DIR`, key = `{request_id}/{id_card\|selfie}.jpg`); HMAC signed URL with a 5-minute TTL for admins to view images (doc 01 §3.4); Scorer interface: `claudeScorer` (Anthropic API over net/http, vision with 2 images, returns JSON score 0–100 + reasons) + `mockScorer` (deterministic by image hash — runs in CI/offline without a key); workflow `kyc_verification` v1: SCORE (auto, retry 2, timeout 30s) → CLASSIFY (auto: ≥85 auto_approved · 50–84 manual_review · <50 rejected, updates `kyc_requests.status` + `users.kyc_status` guarded); user API: POST /v1/kyc/submit (multipart, blocks double-submit), GET /v1/kyc/status; Admin: POST /v1/admin/auth/login (admin_users, argon2id, JWT aud=admin, access-only), GET /v1/admin/kyc?tab=, POST /v1/admin/kyc/:id/review (guarded WHERE status='manual_review' — if 2 reviewers race, the later one gets 409, the foundation for A01) + audit_logs; migration 000014: seed definition + seed 1 dev admin user; openapi in the same commit; tests covering all 3 verdict branches + double-submit + review race + signed URL + mock determinism | OCR extraction of the ID number + AES-GCM PII encryption (F8 extracts no text from images — the scorer returns only score/reasons; encrypted columns are added when a feature needs to read the document number); MinIO/S3 driver (interface ready, local driver is enough for the demo); refresh token for admin (15' access is enough for the dev SPA); notification to the user when the verdict is ready; other admin APIs (fraud/recon/My Tasks — F9) |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Migration 000014: seed definition `kyc_verification` v1 (SCORE→CLASSIFY) + seed dev admin user (`admin@HPay.dev`, precomputed argon2id hash, role superadmin, comment to change in prod). Down removes both | db/migrations/000014_* | S | — |
| T2 | Queries: kyc.sql (InsertKYCRequest, GetKYCRequest, GetLatestKYCByUser, SetKYCScore, ClassifyKYCGuarded WHERE status='pending', ReviewKYCGuarded WHERE status='manual_review', ListKYCQueue by tab, GetAdminByEmail) + users.sql adds SetUserKycStatusGuarded (transition map: none/rejected→pending, pending→auto_approved\|manual_review\|rejected, manual_review→verified\|rejected) | internal/store/queries/kyc.sql, auth.sql | M | T1 |
| T3 | `internal/storage`: Store/Load interface + localDriver; SignURL/VerifyURL (HMAC-SHA256 key+exp, secret = JWT_SECRET); unit tests for sign/verify/expired. Config adds `KYC_STORAGE_DIR` (default ./data/kyc), `ANTHROPIC_API_KEY` (optional) | internal/storage/*, internal/platform/config | M | — |
| T4 | `internal/kyc`: Scorer interface + claudeScorer (net/http, messages API, 2 base64 images, strict JSON parsing, network error/5xx → RetryableError) + mockScorer (score = deterministic hash(image)); pure Classify() (thresholds) + unit tests | internal/kyc/scorer*.go, classify.go | M | — |
| T5 | Workflow handlers: `kyc_scorer` (load images from storage, call Scorer, SetKYCScore + output context) + `kyc_classifier` (ClassifyKYCGuarded + SetUserKycStatusGuarded in 1 tx — idempotent thanks to the guard); register in a separate registry `kyc.RegisterHandlers(reg, pool, scorer, store)` in main.go | internal/kyc/handlers.go, cmd/server | L | T2,T3,T4 |
| T6 | `internal/kyc` service + user API: Submit (multipart jpeg/png ≤5MB; blocked when the user is pending/manual_review or already verified; write images to storage FIRST, then tx: InsertKYCRequest + users→pending guarded + engine.Start(kyc_verification, txn=Nil, context={kyc_request_id}) — an orphan file if the tx fails is acceptable, noted); Status (latest request + users.kyc_status) | internal/kyc/service.go, internal/api/kyc_handler.go | L | T5 |
| T7 | Admin: auth login (GetAdminByEmail + VerifyPassword + JWT aud=admin), RequireAdmin middleware, GET /v1/admin/kyc?tab=manual\|auto\|rejected (with signed image URLs), POST /v1/admin/kyc/:id/review {action, reason} (tx: ReviewKYCGuarded → 0 rows = 409 ERR_ALREADY_REVIEWED; SetUserKycStatusGuarded; audit_logs actor_type=admin); route file /v1/files/kyc/*key authenticated by the HMAC signature itself (no JWT needed); errmap + openapi + router | internal/api/admin_*.go, router.go, errmap.go | L | T3,T6 |
| T8 | Tests: E2E for 3 verdict branches (stub scorer 90/60/30 → auto_approved/manual_review/rejected, users.kyc_status matches); double-submit is blocked; admin review approve→verified + reject; review race (2 admins in parallel → 1 wins, 1 gets ERR_ALREADY_REVIEWED); TxnLimit changes by tier after approve (hooks into F3); expired signed URL; mock determinism; admin login with wrong password | test/integration/kyc_test.go, unit tests | L | T7 |

## 3. Schema changes

No table structure changes — 000014 is only **seed data** (definition + dev admin user). `kyc_requests`/`admin_users`/`audit_logs` have enough columns from F1/F2. If coding reveals a gap → stop and propose a docs fix first (docs-first).

## 4. API changes (into openapi.yaml in the same commit)

| Endpoint | Description | Errors |
| --- | --- | --- |
| `POST /v1/kyc/submit` | multipart `id_card`, `selfie` (jpeg/png ≤5MB) → 202 {request_id, status:pending} — verdict is async, poll /status | ERR_VALIDATION (missing file/wrong format/too large), ERR_KYC_ALREADY_PENDING, ERR_KYC_ALREADY_VERIFIED |
| `GET /v1/kyc/status` | {kyc_status, latest_request: {id, status, ai_score?, ai_reasons?, created_at}} | — |
| `POST /v1/admin/auth/login` | {email, password} → JWT aud=admin (access 15') | ERR_INVALID_CREDENTIALS, ERR_ACCOUNT_LOCKED |
| `GET /v1/admin/kyc?tab=manual\|auto\|rejected` | Review queue (default manual); each item carries `id_card_url`/`selfie_url` signed with HMAC, TTL 5' | 401 when the admin token is missing/invalid |
| `POST /v1/admin/kyc/{id}/review` | {action: approve\|reject, reason} → manual_review → verified\|rejected + users.kyc_status + audit | ERR_KYC_NOT_FOUND, ERR_ALREADY_REVIEWED (409 — 2-reviewer race, foundation for A01), ERR_VALIDATION |
| `GET /v1/files/kyc/{key}` | Streams the image; auth = HMAC signature in the query (exp 5') — no header needed | ERR_INVALID_SIGNATURE (403), ERR_LINK_EXPIRED (410) |

## 5. Test plan

| Group | Test |
| --- | --- |
| 3-branch workflow | Stub scorer returns 90/60/30 → kyc_requests.status = auto_approved / manual_review / rejected; users.kyc_status matches each branch; instance completed; step_executions contains SCORE + CLASSIFY |
| Guards | Submit while pending → ERR_KYC_ALREADY_PENDING; submit while verified → ERR_KYC_ALREADY_VERIFIED; submit again after rejected → OK (re-KYC) |
| Admin review | approve → verified (both tables) + audit row; reject → rejected; 2 admins review the same request in parallel → exactly 1 wins, the later one gets ERR_ALREADY_REVIEWED (guarded UPDATE, same technique as E08) |
| Tier hooks into F3 | A user with none has a 2M limit → after auto_approved the limit is 100M (wallet.TxnLimit) — a 10M transfer fails before and passes after |
| Unit | Classify thresholds (boundaries 85/50); mockScorer deterministic (same image same score); SignURL/VerifyURL correct + expired + tampered key → fail; claudeScorer parse of bad JSON → RetryableError (mock HTTP server) |

F8 has no mandatory E code in the master plan — no REQUIRED_EDGE_IDS added.

## 6. Rollout & risk

- **Images written before the DB tx:** if the tx fails after the file is written → an orphan file in the storage dir. Accepted for local storage (clean up by hand/cron later); reversing the order would be worse (a row pointing to a non-existent image).

- **claudeScorer does not run in CI** (no key) — mockScorer is selected when `ANTHROPIC_API_KEY` is missing, with a clear "using mock scorer" log at boot. Tests inject a stub directly, depending on neither.

- **Admin seed in a migration** is a dev convenience — hard-coded argon2id hash, fixed email; the README states to change it in prod. The password is not logged anywhere.

- Score is `int` 0–100 end-to-end (no float — avoids the grey zone with fintech-review #1 even though score is not money).

- Route `/v1/files/kyc/*key` is mounted at role admin|all (only the admin SPA needs to view images).

- The KYC workflow is the first instance without a transaction — the engine has supported this since F4 (NULLIF insert + the `inst.TransactionID != uuid.Nil` guard in failInstance); low risk, but the 3-branch tests will reveal any dead corner.

## 7. Execution checklist

- [x] gen-feature-plan · [ ] T1 (gen-migration) → T2 (sqlc) → T3→T8 (gen-workflow-step for the 2 handlers, gen-api-test) · [ ] fintech-review · [ ] commit `feat: F8 kyc - vision scorer with mock fallback, kyc workflow, admin review api`

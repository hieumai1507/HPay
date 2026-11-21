[← F1](../f1-foundation/implementation-plan.md) · f2-auth · [solution-architecture →](solution-architecture.md)

# F2 Auth & Users — Implementation Plan

## 1. Scope & non-goals

| Ships | Not in F2 |
| --- | --- |
| User register/login (argon2id), JWT access 15' + refresh 30d **rotation by family + theft detection**, device binding in the token, OTP service (SHA-256 hash, TTL 120s, max 3 attempts, cooldown 30s), rate-limit middleware (Redis fixed window), audit log, admin_users table (admin login for later phases to use), error contract `ERR_*` | Wallet creation on register (F3, with backfill), KYC (F8), admin login handler (opened only when the A-phase needs it), biometric (client-side), admin 2FA |

## 2. Task breakdown

| # | Task | Files | Size |
| --- | --- | --- | --- |
| T1 | Migration 000009: refresh_tokens (family_id, token_hash, used/revoked), admin_users + admin_role, audit_logs (actor_id NOT NULL + fixed SYSTEM_ACTOR uuid) | db/migrations/000009_auth.* | M |
| T2 | sqlc queries with explicit columns (avoid nullable-uuid): users, refresh_tokens, otp, audit | internal/store/queries/auth.sql | M |
| T3 | `internal/auth`: password (argon2id), jwt (HS256), opaque refresh token + SHA-256, pure OTP core (unit-testable), service (Register/Login/Refresh/Logout/OTP) | internal/auth/*.go | L |
| T4 | API: standard errmap, RequireAuth + RateLimit middleware, handlers /v1/auth/* | internal/api/* | M |
| T5 | Unit tests: password, jwt, OTP (E11), rotation decision | internal/auth/*_test.go | M |
| T6 | CI: sqlc generate before test/build; REQUIRED_EDGE_IDS="E11"; config JWT_SECRET | ci.yml, config.go, .env.example | S |

## 3. Schema changes

000009_auth (up/down): 3 tables + 1 enum. `refresh_tokens.family_id` is the rotation chain — 1 login = 1 family; reusing an already-used refresh token ⇒ revoke the whole family (theft detection). `audit_logs` is append-only by convention (no trigger — write volume is high, enforced via permissions + review).

## 4. API changes

| Endpoint | Body → Response | Errors |
| --- | --- | --- |
| `POST /v1/auth/register` | phone, password, full_name (+ X-Device-Id) → user + token pair | ERR_VALIDATION, ERR_PHONE_TAKEN |
| `POST /v1/auth/login` | phone, password (+ X-Device-Id) → token pair | ERR_INVALID_CREDENTIALS (does not distinguish wrong user vs. wrong password), ERR_ACCOUNT_LOCKED, ERR_RATE_LIMITED (5/min/IP) |
| `POST /v1/auth/refresh` | refresh_token → new token pair (rotation) | ERR_INVALID_TOKEN, ERR_TOKEN_EXPIRED, ERR_TOKEN_REUSED (theft → revoke family) |
| `POST /v1/auth/logout` | refresh_token → revoke family | — |

openapi.yaml is updated with all 4 endpoints + schemas.

## 5. Test plan

| ID | Test |
| --- | --- |
| E11 | `TestEdge_E11_OTPBruteForce`: 3 wrong attempts → the challenge is permanently invalidated, even if the correct code is entered on the 4th attempt |
| — | password: roundtrip + wrong password + hashes differ between two runs (salt) |
| — | jwt: issue/parse roundtrip, expired is rejected, wrong signing method is rejected |
| — | rotation: used token → REUSE decision (pure func), expired → EXPIRED |
| — | Full login/refresh integration flow: deferred to F3, when the testutil harness (testcontainers) is set up together with the ledger |

## 6. Rollout & risk

- JWT_SECRET is mandatory in env — fail-fast when missing; changing the secret kills all old access tokens within ≤15' (accepted).

- Code now depends on sqlc generation: devs must run `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` then `make sqlc` before the first build — documented in the README. CI already generates automatically.

- OTP dev-mode: log the code to the console when APP_ENV=dev (never in any other env).

## 7. Execution checklist

- [x] gen-feature-plan · [ ] T1→T6 · [ ] fintech-review · [ ] commit `feat: F2 auth - argon2id, JWT refresh rotation with theft detection, OTP service`

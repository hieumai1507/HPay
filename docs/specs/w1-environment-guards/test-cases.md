---
title: Test cases - environment guards
type: spec
status: proposed
spec: spec.md
---

# Test cases (designed before code)

| Test ID | AC | Type | Priority | Executor | Steps | Expected | Automated as |
|---------|----|------|----------|----------|-------|----------|--------------|
| TC-ENV-001 | AC-1 | Negative | P0 | AUTO-API | `config.Load` with APP_ENV unset, empty, `prod`, `Production`, `production ` | error naming `dev, ci, demo, production` | `internal/platform/config` `TestLoad_RejectsMissingOrUnknownEnv` |
| TC-ENV-002 | AC-2 | Security | P0 | AUTO-API | `APP_ENV=production` with a 20-byte JWT secret; with the `.env.example` placeholder; with no `MOCK_BANK_SECRET`; with the dev default | each fails with a specific message | `TestLoad_ProductionRejectsWeakOrDefaultSecrets` |
| TC-ENV-003 | AC-2 | Security | P0 | AUTO-API | build the router with a production config and POST `/internal/mock-bank/callback` | 404 | `internal/api` `TestMockBankRouteNotMountedInProduction` |
| TC-ENV-004 | AC-3 | Happy | P0 | AUTO-API | `APP_ENV=dev|ci|demo` with short secrets (as the test suite and CI use) | loads successfully | `TestLoad_NonProductionKeepsDevBehavior` |
| TC-ENV-005 | AC-4 | Security | P0 | AUTO-API | approval challenge with env `dev` vs `demo` vs `production` | `otp_dev` present only for `dev` | `internal/api` `TestChallengeOTPOnlyInDev` |
| TC-ENV-006 | AC-2 | Security | P1 | AUTO-API | `cfg.IsProduction()` drives gateway selection | real HTTP gateway only for production | `TestGatewaySelection` (new helper in `cmd/server` or config) |
| TC-ENV-007 | AC-5 | Security | P0 | AUTO-API | seeder guard function with envs and hosts: production, unknown env, `db.example.com`, `localhost`, `postgres`, flag `--allow-remote-db` | refuse, refuse, refuse, allow, allow, allow-with-flag | `cmd/seed` `TestSeedGuard` |
| TC-ENV-008 | AC-6 | Security | P1 | AUTO-API | GET `/healthz` with Postgres or Redis failing | body has no error text, status 503 | `internal/api` `TestHealthzHidesErrorText` |
| TC-ENV-009 | AC-6 | Security | P1 | AUTO-API | any response | `X-Content-Type-Options`, `Referrer-Policy`, `X-Frame-Options`; HSTS only in production | `TestSecurityHeaders` |
| TC-ENV-010 | AC-7 | Security | P1 | AUTO-API | request with forged `X-Forwarded-For` and no trusted proxy; then with the proxy trusted | client IP is the socket address, then the forwarded one | `TestTrustedProxies` |
| TC-ENV-011 | AC-8 | Regression | P0 | AUTO-API | full backend suite with `APP_ENV=ci` set by the runner | 170+ tests pass | whole suite |
| TC-ENV-012 | AC-6 | Boundary | P2 | HUMAN | start the server and inspect `http.Server` fields | timeouts and MaxHeaderBytes set | code review in `hpay-review` |
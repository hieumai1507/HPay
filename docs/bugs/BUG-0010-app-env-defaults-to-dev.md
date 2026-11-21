---
id: BUG-0010
title: A missing APP_ENV silently runs the process as dev (OTP in responses, mock bank route, weak secrets)
status: fixed
severity: high
area: backend/config
tags: [config, env-guard, authz]
audit: [BE-H7]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/internal/platform/config/config_test.go:TestLoad_RejectsMissingOrUnknownEnv ; src/backend/internal/api/env_guards_test.go ; src/backend/cmd/seed/guard_test.go:TestSeedGuard
related: []
---

# BUG-0010: A missing APP_ENV silently runs the process as dev

(The number is deliberately out of sequence: BUG-0002 to BUG-0009 are reserved for work running in parallel; renumber when merging if the user prefers a dense sequence.)

## Symptom
Starting the server without `APP_ENV` behaved as `dev`: approval challenge responses contained the OTP code (`otp_dev`), Gin ran in debug mode, a `JWT_SECRET` of any length was accepted, `MOCK_BANK_SECRET` defaulted to a public string, and `/internal/mock-bank/callback` was mounted in every environment, so anyone who knew the default could credit a wallet. The real outbound gateway could not be enabled by any documented value.

## Root cause
`platform/config/config.go` used `getenv("APP_ENV", "dev")` and never validated it. The documented values were `dev|ci|demo`, but `cmd/server/main.go:117` and `cmd/seed/main.go:43` compared with the literal `production`, and `router.go:81-82` mounted the mock bank route unconditionally. Several packages compared raw strings, so no single place defined what each environment means. `godotenv.Load()` ran in every environment, so a stray `.env` could even supply the environment.

## Other places with the same flaw
Every comparison of the environment string was found with GitNexus and grep: `api/router.go` (Gin mode), `api/approval_handler.go` (otp_dev), `auth/service.go:199` (OTP debug log, takes the env string and was left on `dev` only), `platform/logger/logger.go` (debug level on `dev` only, unchanged), `cmd/server/main.go` (gateway), `cmd/seed/main.go` (seed guard). Also fixed in the same slice: `/healthz` returned raw driver errors to anonymous callers, the HTTP server had only `ReadHeaderTimeout`, Gin trusted every proxy (so a forged `X-Forwarded-For` defeated the rate limiter), no security headers were set, and the seeder could run against any database host.

## Fix
ADR-0012. `APP_ENV` is required and must be `dev`, `ci`, `demo` or `production`; the helpers `Config.IsDev()`, `IsProduction()` and `AllowsMockBank()` answer every environment question. Outside `dev` and `ci` `JWT_SECRET` must be at least 32 bytes and not the `.env.example` placeholder, and `MOCK_BANK_SECRET` must be set and non-default; the mock bank route is not mounted in production; `.env` is loaded only when `APP_ENV=dev` comes from the real environment; the seeder runs only for dev, ci and demo and refuses a non-local host without `--allow-remote-db`; `/healthz` reports `ok` or `down` per dependency without error text; server read, write and idle timeouts and `MaxHeaderBytes`; baseline security headers (HSTS only in production); `TRUSTED_PROXIES`; compose binds ports to loopback; every launcher, CI job and the test runner sets `APP_ENV` explicitly.

## Regression test
- `TestLoad_RejectsMissingOrUnknownEnv` (6 inputs), `TestLoad_ProductionRejectsWeakOrDefaultSecrets` (4), `TestLoad_ProductionAcceptsStrongSecrets`, `TestLoad_NonProductionKeepsDevBehavior`, `TestIsDevOnlyForDev`, `TestTrustedProxiesParsing`.
- `TestMockBankRouteMountedOnlyWhereAllowed`, `TestOTPExposedOnlyInDev`, `TestHealthzHidesErrorText`, `TestSecurityHeaders`, `TestTrustedProxiesControlClientIP`.
- `TestSeedGuard` (16 cases) and `TestSeedGuardNamesTheHostItRefuses`.
- Mutation check: re-introducing the old default, skipping the secret strength check, allowing the mock bank in production, or making `IsDev` true for `ci` each made the config tests fail.
- Full backend suite after the change: 219 tests, 0 failures, with `-race`.

## How to recognize this again
- A deployment that starts without setting `APP_ENV` and behaves like dev (OTP codes in responses).
- `/internal/mock-bank/callback` answering outside dev, ci or demo.
- Rate-limit counters keyed on addresses that vary with a request header.
- A seed run that created accounts in a database that is not local.

## Docs updated
`reference/configuration.md`, `architecture/deployment.md`, `guides/setup-local-environment.md`, `quality/security-model.md`, `domain/identity-and-auth.md` (BR-AUTH-23), `constitution.md` (C-09), `adr/0012-explicit-app-env.md`.

## Not done / still open from the original finding
`/internal/*` is still not blocked at the proxy (infrastructure task), the AI key and Cloudinary rotation (user task), CORS and request body size limits, and an explicit `staging` environment if one is introduced.
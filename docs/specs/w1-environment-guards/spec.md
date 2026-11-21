---
title: Wave 1 slice 3 - explicit, validated environment and safe defaults
type: spec
status: proposed
last_verified: 2026-11-21
audit: [BE-H7]
related:
  - ../../domain/identity-and-auth.md
  - ../../reference/configuration.md
  - ../../constitution.md
  - ../../adr/0012-explicit-app-env.md
---

# Wave 1 slice 3: explicit, validated environment and safe defaults (BE-H7)

## Problem (verified in code)
- `APP_ENV` defaults to `dev` and is not validated (`platform/config/config.go:72`). Forgetting it enables dev behavior: `otp_dev` is returned in approval challenge responses (`api/approval_handler.go:48`), Gin runs in debug mode, a `JWT_SECRET` of any length is accepted (`config.go:92-96`), and `godotenv.Load()` reads a stray `.env` in every environment.
- `MOCK_BANK_SECRET` defaults to `dev-mock-bank-secret` and the route `/internal/mock-bank/callback` is mounted in every environment (`api/router.go:81-82`): anyone who knows the default can credit a wallet.
- The documented environments are `dev|ci|demo`, but the code compares with the literal `production` (`cmd/server/main.go:117`, `cmd/seed/main.go:43`), so the real outbound HTTP gateway is unreachable by any documented value and the seed guard only blocks one spelling.
- The seeder can run against any database, and it creates users and admins with known passwords.
- `/healthz` returns raw Postgres/Redis error strings to anonymous callers; the HTTP server has only `ReadHeaderTimeout`; Gin trusts every proxy (client IP, and so rate limiting, can be spoofed with `X-Forwarded-For`); there are no baseline security headers.
- `docker-compose.yml` publishes Redis without a password on all interfaces.

## Goals
Behavior that is dangerous outside development is impossible outside development, and a missing or misspelled `APP_ENV` makes the process refuse to start instead of silently running as dev.

## Non-goals
Per-user idempotency (slice 2), RBAC and PIN (Wave 3), the real gateway SSRF allowlist (Wave 3), key rotation (user).

## Decisions
1. `APP_ENV` is **required** and must be one of `dev`, `ci`, `demo`, `production`. Anything else, or empty, fails startup with a clear error. (`staging` is not introduced; the product has no staging environment today. Add it by amending the whitelist and the ADR when it exists.)
2. Secrets: outside `dev` and `ci`, `JWT_SECRET` must be at least 32 bytes and must not be a known placeholder (the value in `.env.example`); `MOCK_BANK_SECRET` must be set explicitly and must not equal the dev default. In `production`, neither may be a placeholder.
3. Mock bank: the callback route is mounted only when `APP_ENV` is `dev`, `ci` or `demo`; never in `production`.
4. Outbound gateway: real HTTP gateway only in `production`; mock in `dev`, `ci`, `demo` (this is today's intent, now reachable and spelled consistently through one helper `cfg.IsProduction()`).
5. `otp_dev` is returned only when `APP_ENV` is `dev` (unchanged), and now `dev` can only be selected explicitly.
6. `godotenv.Load()` only when `APP_ENV` is `dev` and the variable is set in the real environment (read the variable first, then decide whether to load `.env`; a `.env` file may not supply `APP_ENV` for a non-dev process).
7. Seeder: refuses to run unless `APP_ENV` is `dev`, `ci` or `demo`, and refuses a `DATABASE_URL` whose host is not `localhost`, `127.0.0.1`, `::1`, `postgres` or `host.docker.internal` unless `--allow-remote-db` is passed (printing the host it is about to touch).
8. HTTP hardening: `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`; `SetTrustedProxies` from a new `TRUSTED_PROXIES` variable (default: none, i.e. use the connection address); security headers (`X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`, and `Strict-Transport-Security` only when `production`); `/healthz` returns only `ok` or `degraded` per dependency without error text; error text is logged server-side.
9. `docker-compose.yml`: Redis requires a password (`REDIS_PASSWORD`, default a dev value in `.env.example`) and ports bind to `127.0.0.1`; `REDIS_ADDR` consumers accept a password. Postgres port binds to `127.0.0.1`.

## Acceptance criteria
- **AC-1** Starting with `APP_ENV` unset, empty, or `prod`/`production ` (typo) fails with an error naming the allowed values.
- **AC-2** With `APP_ENV=production`: a short or placeholder `JWT_SECRET` fails; an unset or default `MOCK_BANK_SECRET` fails; the mock-bank route returns 404; the outbound gateway is the real HTTP one.
- **AC-3** With `APP_ENV=dev|ci|demo` the previous behavior remains (tests and local runs keep working), except that `dev` must now be explicit.
- **AC-4** `otp_dev` appears in a challenge response only when `APP_ENV=dev`.
- **AC-5** The seeder refuses to run for `production` or for an unknown env, and refuses a non-local database host without `--allow-remote-db`.
- **AC-6** `/healthz` never contains error text; the server sets the timeouts and `MaxHeaderBytes`; the response carries the security headers; `HSTS` only in `production`.
- **AC-7** With `TRUSTED_PROXIES` unset, a forged `X-Forwarded-For` does not change the client IP used by the rate limiter; with it set to the proxy address it does.
- **AC-8** CI (`backend-ci.yml`, `admin-ci.yml`) and `scripts/test-backend.ps1`, `docker-compose.yml`, `.env.example`, `scripts/dev-local.bat` set `APP_ENV` explicitly and still pass.

## Invariants touched
C-09 (environment behavior explicit), C-11 (no default secrets).

## Architecture impact: `amend`
Pages: `architecture/deployment.md`, `architecture/crosscutting-concepts.md`, `reference/configuration.md`, `domain/identity-and-auth.md`, `guides/setup-local-environment.md`, `quality/security-model.md`. ADR: `adr/0012-explicit-app-env.md` (proposed; an unset `APP_ENV` becomes a startup error, a breaking change for any existing deployment that relied on the default).

## Constitution check
| ID | Touched | Compliant | How |
|----|---------|-----------|-----|
| C-09 | yes | yes after change | explicit validated env; dev-only behavior gated on it |
| C-11 | yes | improves | no placeholder or default secret outside dev and ci |
| C-14 | yes | yes | tests first |
Others: n/a.

## Open questions
1. Breaking change: any deployment that does not set `APP_ENV` will stop starting. The only deployments known to the repo are local scripts, CI and docker compose, all updated here. Confirm there is no hosted deployment that relies on the default (the mobile config points at `hpay.demaciahieu.store`; whether that host runs this backend is unknown).
2. `demo` and `ci` are treated as non-production for secrets; revisit if `demo` ever holds real data.
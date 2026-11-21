---
title: Deployment, CI and developer environment
type: architecture
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/Dockerfile, src/backend/docker-compose.yml, src/backend/Makefile, src/backend/.env.example, .github/workflows/backend-ci.yml, .github/workflows/admin-ci.yml, scripts/test-backend.ps1, scripts/test-backend.sh, src/backend/internal/platform/config/config.go, src/mobile/app.config.ts]
---

# Deployment, CI and developer environment

## Container image (`src/backend/Dockerfile`)

Multi-stage: `golang:1.25` build stage installs `sqlc@v1.30.0`, runs `sqlc generate`, then `CGO_ENABLED=0 go build -o /out/HPay ./cmd/server`; runtime is `gcr.io/distroless/static-debian12` with `ENTRYPOINT ["/HPay"]`, `CMD ["--role=all"]`. Consequences: the generated `internal/store/gen` is not committed-dependent (it is regenerated at build), there is no shell, no `migrate` binary, no timezone database override (default UTC), and migrations are NOT run by the image. CI pins sqlc 1.27.0 while the Dockerfile pins 1.30.0 (version skew).

## docker-compose (`src/backend/docker-compose.yml`)

Services: `postgres` (postgres:16-alpine, user/password/db `HPay`, port 5432, healthcheck), `redis` (redis:7-alpine, 6379, healthcheck), and `app` behind the compose profile `app` (build `.`, env `DATABASE_URL`, `REDIS_ADDR`, port 8080). Day-to-day development runs `go run ./cmd/server --role=all` against compose Postgres/Redis (`Makefile: dev`). No volumes are declared, so `docker compose down` loses data.

## Configuration (12-factor env vars, `platform/config/config.go:61-104`)

| Variable | Default | Notes |
|---|---|---|
| `APP_ENV` | none, **required** | one of `dev`, `ci`, `demo`, `production`; missing or unknown stops startup (ADR-0012). `Config.IsDev()` gates `otp_dev`, `.env` loading, debug logging and Gin debug mode; `Config.IsProduction()` selects the real outbound gateway, HSTS and the strictest secret checks; `AllowsMockBank()` decides whether the mock bank route exists |
| `HTTP_PORT` | 8080 | |
| `DATABASE_URL` | none, required | |
| `REDIS_ADDR` | `localhost:6379` | |
| `TRUSTED_PROXIES` | empty | proxy addresses or CIDRs trusted for `X-Forwarded-For`; empty trusts none |
| `JWT_SECRET` | none, required | at least 32 bytes and not the `.env.example` placeholder outside `dev` and `ci`; also used to HMAC-sign file links and QR codes (audit BE-H6/H9) |
| `KYC_STORAGE_DIR` | `./data/kyc` | |
| `STORAGE_PROVIDER` | `local` | `cloudinary` requires `CLOUDINARY_CLOUD_NAME/API_KEY/API_SECRET` |
| `MOCK_BANK_SECRET` | `dev-mock-bank-secret` in `dev` and `ci` only | required and non-default in `demo` and `production` (the route is not mounted in `production`) |
| `DEFAULT_OFFER_CODE` | `OFFER_CUST_L0` | read in `offerguard.go:55`; offer used when a user has no `user_offers` row |
| round-up unit env | see `workflow/handlers/round_up.go:22` | |

`godotenv.Load()` reads `./.env` in every environment (existing env wins). `src/backend/.env` exists in the working tree; the audit flagged real secrets in it (rotate; do not copy values into docs).

Mobile API base URL comes from `src/mobile/app.config.ts` `extra.apiBaseUrl` selected by `APP_ENV`: `dev` currently points at a remote host (`https://hpay.demaciahieu.store`, the localhost line is commented out), `staging`/`demo` are placeholder `example.com` URLs. Admin uses `VITE_API_URL` (empty = same origin, i.e. Vite dev proxy or a reverse proxy).

## Migrations

Files `src/backend/db/migrations/000001..000013_*.{up,down}.sql` (13 pairs). They are applied only by the external `migrate` CLI (golang-migrate v4.17.1 in CI, `migrate/migrate:v4.17.1` image in `scripts/test-backend.ps1`, `make migrate-up/-down/-verify`). The server never migrates on start; there is no migration step in the Dockerfile or compose. A deploy pipeline must run `migrate up` before starting the new binary. CI proves up -> down -> up works and that `ledger_entries` rejects UPDATE/DELETE (append-only trigger, migration 000007).

## CI workflows

`.github/workflows/backend-ci.yml` (push to main and PRs touching `src/backend/**`; working dir `src/backend`):

| Job | What it does |
|---|---|
| `lint` | sqlc generate, `go mod tidy`, gofmt check, `go vet`, `scripts/check-openapi-sync.py` (router vs `api/openapi.yaml`); golangci-lint is a TODO |
| `sqlc` | `sqlc vet \|\| true`, `sqlc generate` |
| `migrate` | Postgres 16 service; up, down -all, up; asserts append-only trigger on `ledger_entries` |
| `test` | Postgres on 5433 + Redis on 6380; migrate; `go test -p 1 ./... -race -count=1`; then `scripts/check-edge-tests.sh` (E01-E20 coverage gate). `-p 1` because packages that pump the shared outbox must not run concurrently |
| `build` | `CGO_ENABLED=0 go build ./cmd/server` (needs lint, test) |
| `vuln` | `govulncheck ./...` (installed `@latest`, unpinned) |
| `image-scan` | docker build + Trivy HIGH/CRITICAL fails (ignore-unfixed) |

`.github/workflows/admin-ci.yml` (paths `src/admin/**`, `src/backend/**`): single `e2e` job; Postgres + Redis services, sqlc generate, migrate up, build server, `go run ./cmd/seed --e2e-admins`, start server and wait on `/healthz`, `npm ci && npm run build` in `src/admin`, serve `vite preview` on 4173, run `npx playwright test --project=chromium`; uploads the report on failure. There is no mobile CI workflow and no admin unit-test job (only Playwright and `tsc` inside `npm run build`).

No CD workflow exists: nothing builds/pushes the image or deploys.

## Developer machine requirements

- Go 1.25 toolchain (go.mod), `sqlc` (generated code `internal/store/gen` is needed to compile), `migrate` CLI, Docker for Postgres/Redis, Node 20 for admin and mobile, Python 3 for the openapi sync script.
- Windows without a local Go/Postgres: use the Docker runner `scripts/test-backend.ps1`. It requires Docker Desktop, then: `docker compose -p hpay up -d postgres redis` from `src/backend`; runs `migrate/migrate:v4.17.1 ... up` on network `hpay_default`; runs `golang:1.25` with the repo mounted at `/src`, cached volumes `hpay-gomod` and `hpay-gobuild`, env `TEST_DATABASE_URL`, `REDIS_ADDR=redis:6379`, `GOFLAGS=-buildvcs=false`, executing `scripts/test-backend.sh` with the args of `-GoArgs` (default `./... -count=1`; example `.\scripts\test-backend.ps1 -GoArgs "./internal/ledger -race -count=1"`). Its header comment mentions a parameter called `-Args`; the real parameter is `-GoArgs`.
- Integration tests skip when `TEST_DATABASE_URL` is unset (`internal/testutil/testutil.go`).
- Stray build artifacts in `src/backend` (`hpay-server-local.exe`, `integration_test.exe`, `tmp/`, `.gotmp`) are local leftovers, not part of the build.

## Not present

Reverse proxy config (the `/internal/` prefix must be blocked at the proxy, comment in `router.go:79-80`), TLS, secrets management, horizontal-scaling guidance, backups, monitoring/alerting.

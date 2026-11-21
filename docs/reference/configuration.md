---
title: Configuration and tooling
type: reference
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/internal/platform/config/config.go, src/backend/cmd/server/main.go, src/backend/cmd/seed/main.go, src/backend/docker-compose.yml, src/backend/Makefile, src/backend/Dockerfile, .github/workflows/backend-ci.yml, .github/workflows/admin-ci.yml]
---

# Configuration and tooling

## Process role

`cmd/server --role=<api|worker|admin|all>` (default CMD in the Dockerfile is `--role=all`). Invalid role aborts startup.

| Role | HTTP | Public `/v1` + mock bank | `/v1/org`, `/v1/admin`, `/v1/files` | Background workers |
|---|---|---|---|---|
| api | yes | yes | no | no |
| admin | yes | no | yes | no |
| worker | no | no | no | yes |
| all | yes | yes | yes | yes |

Workers (role worker/all): outbox relay, workflow worker, recovery, approval sweeper, money-request sweeper, nightly recon cron (00:15).

## Environment variables

`.env` in the working directory is loaded with godotenv in every environment; real environment variables win.

| Name | Default | Required | Secret | Meaning |
|---|---|---|---|---|
| APP_ENV | `dev` | no | no | Environment: documented values `dev`, `ci`, `demo`; see below |
| HTTP_PORT | 8080 | no | no | Listen port (invalid int silently falls back to default) |
| DATABASE_URL | none | yes | yes (password) | Postgres connection string |
| REDIS_ADDR | localhost:6379 | no | no | Redis address (no password support) |
| JWT_SECRET | none | yes; at least 32 bytes unless APP_ENV=dev | yes | Signs user and admin JWTs and signed file URLs |
| KYC_STORAGE_DIR | ./data/kyc | no | no | Local storage root (provider `local`) |
| STORAGE_PROVIDER | local | no | no | `local` or `cloudinary` |
| CLOUDINARY_CLOUD_NAME | none | when provider=cloudinary | no | |
| CLOUDINARY_API_KEY | none | when provider=cloudinary | yes | |
| CLOUDINARY_API_SECRET | none | when provider=cloudinary | yes | |
| MOCK_BANK_SECRET | dev-mock-bank-secret | no (but change outside dev) | yes | Shared secret for `/internal/mock-bank/callback` |
| ENGINE_DYNAMIC_DISPATCH | enabled | no | no | Read in `transfer/service.go`; only the literal `false` disables service-driven workflow dispatch |
| OFFERGUARD_ENABLED | enabled | no | no | Only literal `false` disables OfferGuard limits |
| DEFAULT_OFFER_CODE | none | no | no | `offerguard` fallback offer code for users without `user_offers` |
| ROUND_UP_UNIT_VND | handler default | no | no | Round-up step rounding unit |
| TEST_DATABASE_URL | none | tests only | yes | Integration tests; skipped if unset |
| TEST_REDIS_ADDR | none | tests only | no | Used by CI test job |
| DB_URL (Makefile only) | postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable | no | yes | Makefile/migrate and seed target |

`seed` (cmd/seed) also reads `APP_ENV` (refuses to run when `production`) and `DATABASE_URL`; flag `--e2e-admins` creates e2e admin accounts.

## APP_ENV effects

Only `dev` is special-cased in most places. Values other than `dev` behave as production-like except as noted.

| Place | Rule |
|---|---|
| config.Load | JWT_SECRET shorter than 32 bytes rejected unless `dev` |
| api/router.go | Gin release mode unless `dev` |
| platform/logger | `dev` uses the dev logger format, others the production format |
| auth/service.go:199 and api/approval_handler.go:48 | `dev` only: OTP value returned in the challenge response (`otp_dev`) |
| cmd/server/main.go:117 | Action pipeline outbound gateway is the Mock gateway unless `APP_ENV == "production"`; the real HTTP gateway is only used for the literal `production`, which config comments do not list (`dev|ci|demo`) |
| cmd/seed | Refuses `production` |
| Not env-gated | `/internal/mock-bank/callback` mounted in every env; `godotenv.Load()` in every env |

Default `APP_ENV` is `dev`, so an unset variable means weak-secret acceptance and OTP leakage (audit BE-H7).

## docker-compose (src/backend/docker-compose.yml)

| Service | Image | Port | Notes |
|---|---|---|---|
| postgres | postgres:16-alpine | 5432 | user/password/db all `HPay`; healthcheck pg_isready |
| redis | redis:7-alpine | 6379 | no password; healthcheck redis-cli ping |
| app | build . (profile `app`) | 8080 | Only with `docker compose --profile app up`; sets DATABASE_URL and REDIS_ADDR but not JWT_SECRET (startup fails) |

## Makefile targets (src/backend)

| Target | Action |
|---|---|
| dev | `go run ./cmd/server --role=all` |
| build | `go build -o bin/hpay ./cmd/server` |
| test | `go test ./... -count=1` |
| test-integration | tests with TEST_DATABASE_URL=$(DB_URL) |
| lint | `go vet ./...` and `golangci-lint run` |
| sqlc | `sqlc generate` |
| migrate-up / migrate-down | golang-migrate up / `down -all` on `db/migrations` |
| migrate-verify | up, down -all, up |
| seed | build and run `cmd/seed` with DATABASE_URL=$(DB_URL) |
| loadtest | `k6 run scripts/loadtest/k6-transfer.js` |
| up / down | `docker compose up -d` / `down -v` |

Note: `.PHONY` omits `test-integration`.

## Dockerfile

Multi-stage: golang:1.25 builds (installs sqlc v1.30.0, runs `sqlc generate`, `CGO_ENABLED=0`), runs on `gcr.io/distroless/static-debian12`, `ENTRYPOINT /HPay`, `CMD --role=all`.

## CI workflows

### backend-ci.yml (paths: src/backend/**; push to main and PRs; working dir src/backend)

| Job | What it does |
|---|---|
| lint | sqlc generate, go mod tidy, gofmt check on cmd and internal, go vet, `python3 scripts/check-openapi-sync.py` |
| sqlc | `sqlc vet` (tolerated failure), `sqlc generate`, checks `internal/store/gen` exists |
| migrate | Postgres 16 service; migrate up, down -all, up; then psql proves the ledger_entries UPDATE and DELETE are blocked by the append-only trigger |
| test | Postgres (port 5433) and Redis (6380) services; migrate up; `go test -p 1 ./... -race -count=1`; `scripts/check-edge-tests.sh` (E01 to E20 gate) |
| build | needs lint and test; `CGO_ENABLED=0 go build ./cmd/server` |
| vuln | `govulncheck ./...` (installed at @latest) |
| image-scan | docker build then Trivy (HIGH,CRITICAL, ignore-unfixed, exit-code 1) |

### admin-ci.yml (paths: src/admin/**, src/backend/**)

Single job `e2e`: Postgres and Redis services, JWT_SECRET set to a CI value, APP_ENV unset (defaults to dev); sqlc generate, migrate up, build server, `go run ./cmd/seed --e2e-admins`, start server and wait on /healthz, Node 20, `npm ci`, `npm run build`, Playwright chromium install, `vite preview --port 4173`, `npx playwright test --project=chromium`, uploads `playwright-report` on failure. There is no mobile CI workflow.

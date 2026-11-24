---
title: Set up the local environment
type: guide
status: verified-against-code
last_verified: 2026-11-21
---

# Set up the local environment

Goal: run Postgres, Redis, the backend, the admin console and the mobile app on one machine, with demo data.

Sources: `src/backend/Makefile`, `docker-compose.yml`, `.env.example`, `internal/platform/config/config.go`, `cmd/seed/main.go`, `scripts/dev-local.bat`, `.github/workflows/*.yml`.

## Prerequisites

| Tool | Why | Notes |
|------|-----|-------|
| Docker Desktop (running) | Postgres 16, Redis 7, and the Docker-based test runner | Required for tests on this dev machine, see [run-tests.md](run-tests.md) |
| Go 1.25 (`go.mod` says `go 1.25.12`) | build/run backend | |
| `sqlc` 1.27+ | generates `internal/store/gen` (git-ignored, so it is missing on a fresh checkout) | CI pins 1.27.0, the Dockerfile uses 1.30.0 |
| `migrate` (golang-migrate v4.17.1) CLI | applies `db/migrations` | The server does NOT run migrations itself |
| Node 20+ and npm | admin and mobile | CI uses Node 20 |
| `make` | optional shortcuts | On Windows you can run the underlying commands directly |
| `k6` | optional, only for `make loadtest` | not installed on the verified machine |

Windows note: Windows Application Control on the dev machine blocks Go-built executables (`go run`, `go test` binaries, `bin/seed`). Where a step below runs a Go binary, the Docker alternative is given.

## 1. Environment files

1. Copy the template (the real `.env` is git-ignored and must never be committed):
   ```powershell
   Copy-Item src\backend\.env.example src\backend\.env
   ```
2. Keys (from `.env.example` and `config.Load`):

| Key | Default / example | Required | Meaning |
|-----|-------------------|----------|---------|
| `APP_ENV` | `dev` (in `.env.example`) | **yes**, no code default | `dev`, `ci`, `demo`, `production`. Missing or unknown stops the server. `dev` returns OTP codes in responses and loads `.env`; outside `dev` and `ci` a `JWT_SECRET` under 32 bytes or the example placeholder is refused |
| `HTTP_PORT` | `8080` | no | |
| `DATABASE_URL` | `postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable` | yes | matches `docker-compose.yml` |
| `REDIS_ADDR` | `localhost:6379` | no | |
| `JWT_SECRET` | placeholder in `.env.example` | yes | signs user and admin tokens (audience differs) and KYC signed URLs |
| `KYC_STORAGE_DIR` | `./data/kyc` | no | local KYC image storage (PII, git-ignored) |
| `STORAGE_PROVIDER` | `local` | no | `local` or `cloudinary`; cloudinary needs `CLOUDINARY_CLOUD_NAME/API_KEY/API_SECRET` |
| `MOCK_BANK_SECRET` | `dev-mock-bank-secret` | no | shared secret for `/internal/mock-bank/callback` |

3. Safety: `godotenv` loads `./.env` in every environment. Existing process env vars win. If your `.env` points at a cloud database, override `DATABASE_URL` in the shell before running anything (`scripts/dev-local.bat` does exactly that) so you never seed or test against a remote database. Never paste `.env` contents into docs, logs or chats.

## 2. Start Postgres and Redis

```powershell
cd src\backend
docker compose up -d        # same as: make up
docker compose ps
```
Expected: `postgres` (5432) and `redis` (6379) services running and healthy. `make down` runs `docker compose down -v` and DELETES the database volume.

## 3. Generate the sqlc code

```powershell
cd src\backend
sqlc generate               # same as: make sqlc
```
Expected: no output, and `internal/store/gen/` exists. Run it again after every change to `db/migrations` or `internal/store/queries`.

## 4. Apply migrations

```powershell
cd src\backend
migrate -path db/migrations -database "postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable" up
# or: make migrate-up
```
Expected: it applies `1/u ... 13/u`. Check: `docker exec hpay-postgres-1 psql -U HPay -d HPay -c "SELECT version FROM schema_migrations;"` returns `13` (verified on this machine). The container name depends on the compose project name (`hpay-postgres-1` when the project is `hpay`).

Reversibility check (E18): `make migrate-verify` (up, down -all, up). It destroys data on that database.

## 5. Seed demo data

The seeder (`cmd/seed`) is idempotent: users upsert by phone, every transaction has a fixed idempotency key. It runs only for `APP_ENV` `dev`, `ci` or `demo`, refuses a non-local database host unless `--allow-remote-db` is passed, and requires `DATABASE_URL`. At the end it asserts `wallets.balance == SUM(ledger)` for every wallet it touched and exits non-zero on drift. All money moves through `ledger.Post`.

The seeded KYC request points at two demo images (`seed/kyc/id-card.png`, `seed/kyc/selfie.png`). The seeder draws two plain placeholder PNGs (no personal data) into `KYC_STORAGE_DIR` (default `./data/kyc`), so the admin KYC screen shows pictures. The server must read the same directory: set `KYC_STORAGE_DIR` to the same value for both, and make sure `STORAGE_PROVIDER` is `local` (a `.env` with `STORAGE_PROVIDER=cloudinary` makes the server look for the images in Cloudinary, which does not have them, and every image answers 404).

```powershell
cd src\backend
$env:DATABASE_URL = "postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable"
go run ./cmd/seed                  # demo data
go run ./cmd/seed --e2e-admins     # demo data PLUS two fixed-credential admins for Playwright
```
Makefile shortcut: `make seed` (builds `bin/seed` first). If Application Control blocks the binary, run it in Docker instead (not executed during this verification, adapt the network name to your compose project):
```powershell
docker run --rm --network hpay_default -v "${PWD}:/src" -w /src -e DATABASE_URL="postgres://HPay:HPay@postgres:5432/HPay?sslmode=disable" -e GOFLAGS=-buildvcs=false golang:1.25 go run ./cmd/seed
```

Flags: the only flag is `--e2e-admins`.

### Demo accounts created by the seeder

All demo users share the demo password defined as the constant `demoPassword` in `cmd/seed/main.go`: `Demo@123`. This is a demo-only value; it must never exist in a production database (the seeder refuses `APP_ENV=production`, but see AUDIT BE-H7: `APP_ENV` is not validated, so rely on never pointing the seeder at a real database).

| Phone | Name | State |
|-------|------|-------|
| 0900000001 | Hieu Mai | KYC verified, balance 12,450,000 VND, vault "Du lich Da Lat", about 10 mixed transactions |
| 0900000002 | Minh Anh | verified, P2P counterpart, one pending money request |
| 0900000003 | Tran Van Ba | KYC `manual_review` (AI score 64), sits in the admin KYC queue |
| 0900000004 | Le Thi Tu | account `locked`, confirmed fraud flag, reversal approval pending in admin My Tasks |
| 0900000011 to 0900000014 | Acme Owner, Maker, Checker L1, Checker L2 | organization "Acme Corp", policy "Chi lon" v3, org wallet, one 45M disbursement at 1/2 signatures |

Also seeded: workflow instances in `failed`, `compensated` and `processing_async`-like states (for the admin monitor), fraud flags, a reconciliation day with 3 `missing_external` rows around 23:58 to 00:02, a split money request.

Admin console logins:
- `admin@HPay.dev` (superadmin) is inserted by migration `000001`. Its password is not documented in code comments; reset it by upserting a new argon2id hash if you do not know it.
- With `--e2e-admins`: `e2e-maker@HPay.dev` (role `ops`) and `e2e-checker@HPay.dev` (role `superadmin`), password `E2ETest@123` (constant `e2eAdminPassword`). Known credentials: never create these outside a local or CI database.

## 6. Run the backend

```powershell
cd src\backend
go run ./cmd/server --role=all     # same as: make dev
```
Roles (`--role`): `api` (public `/v1`), `admin` (`/v1/admin` and `/v1/org`), `worker` (outbox relay, workflow consumers, recovery, sweepers, recon cron), `all` (everything; use this locally).

Expected log lines: `starting HPay`, `http listening addr=:8080`, `workflow worker running handlers=[...]`. Health check:
```powershell
curl http://localhost:8080/healthz
```
`scripts/dev-local.bat` does the same with the local Docker database forced via env vars. Build a binary with `make build` (`bin/hpay`).

Do not run two `--role=all`/`worker` replicas against one database: the schedulers are in-process goroutines without leader election (see ADR 0005).

## 7. Run the admin console

```powershell
cd src\admin
npm ci
npm run dev                         # http://localhost:5173
```
Vite proxies `/v1` and `/healthz` to `http://localhost:8080`; `VITE_API_URL` overrides the base URL. Other scripts: `npm run build` (typecheck + build), `npm run typecheck`, `npm run lint`, `npm run gen:api`, `npm run test:e2e`.

## 8. Run the mobile app

```powershell
cd src\mobile
npm ci
npx expo start                      # or: npm start / npm run android / npm run ios
```
Known trap: in `app.config.ts` the `dev` API base URL currently points at a remote host (`https://hpay.demaciahieu.store`); the localhost line is commented out. For local work edit that map locally (do not commit the change unless intended) so `dev` points at `http://localhost:8080`, or the emulator host alias (`http://10.0.2.2:8080` on Android emulators).

## Troubleshooting

| Symptom | Cause / fix |
|---------|-------------|
| `store.X undefined` compile wall | `sqlc generate` was not run or failed; fix it first |
| `DATABASE_URL is required` / `JWT_SECRET is required` | `.env` missing or not loaded (working directory must be `src/backend`) |
| `APP_ENV "" is not valid: it is required and must be one of dev, ci, demo, production` | export `APP_ENV=dev` for local runs (the Makefile and `scripts/dev-local.bat` already do) |
| `JWT_SECRET must be at least 32 bytes when APP_ENV=...` | use `openssl rand -base64 48`, or `APP_ENV=dev`/`ci` for local runs |
| `refusing to seed a non-local database host` | you pointed `DATABASE_URL` at a remote database; the seeder creates accounts with known passwords. Use the local compose database, or pass `--allow-remote-db` only if you are sure |
| Go binaries blocked by Windows | use Docker (tests: `scripts/test-backend.ps1`) |
| Seed prints `LEDGER DRIFT` | a wallet balance differs from its ledger sum; do not ignore, investigate |

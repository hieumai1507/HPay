---
title: Building blocks
type: architecture
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/cmd/server/main.go, src/backend/internal, src/backend/internal/platform/config/config.go, src/admin/package.json, src/mobile/package.json]
---

# Building blocks

## Repository layout

```
D:\hpay
  src/backend   Go module github.com/hieumai1507/hpay-backend (go 1.25)
    cmd/server  the single server binary (main.go, 146 lines)
    cmd/seed    idempotent demo-data seeder (also --e2e-admins for CI)
    internal/   all packages (below)
    db/migrations  000001..000013 .up/.down SQL (golang-migrate file naming, applied by the external CLI)
    internal/store/queries  *.sql inputs; internal/store/gen  sqlc output (generated, built in Docker/CI by `sqlc generate`)
    api/openapi.yaml  63 paths, kept in sync with router.go by scripts/check-openapi-sync.py (CI lint job)
    test/integration/<area>, test/testkit, scripts/ (loadtest k6, check-edge-tests.sh, dev bat files)
  src/admin     React 18 + Vite back-office
  src/mobile    Expo 54 / React Native 0.81 app
  scripts/      test-backend.ps1 / .sh (Docker test runner)
  .github/workflows  backend-ci.yml, admin-ci.yml
  docs/
```

Direct dependencies (go.mod): gin 1.10, golang-jwt v5, google/uuid, pgx v5, godotenv, go-redis v9, excelize v2 (recon xlsx), x/crypto (argon2id). There is no asynq, golang-migrate library, testify or testcontainers in go.mod (contradicts `docs/history/design-v2/00-backend-design-v2-base.md:71-76` and `docs/overview.md:44,49,63`).

## Backend packages by layer

Dependencies below were derived from `import` statements in non-test files (2026-11-21). Everything depends on `store/gen` (sqlc) unless noted.

| Layer | Package | Responsibility | Depends on (internal) |
|---|---|---|---|
| Entry | `cmd/server` | wire config, DB, Redis, HTTP, workers | api, approval, auth, fee, fraud, gateway, kyc, moneyrequest, outbox, platform/*, recon, storage, workflow, workflow/handlers |
| HTTP | `api` | Gin router, middleware (auth, rate limit, idempotency header), handlers, `errmap.go` error contract | approval, auth, fee, fraud, kyc, ledger, listing, moneyrequest, offerguard, org, platform/config, product, qr, recon, reversal, storage, transfer, user, vault, wallet, workflow |
| Orchestration | `workflow` | engine (state machine), worker (Redis stream consumer), recovery, registry of step handlers | gateway, ledger |
| Orchestration | `workflow/handlers` | step handlers: validator, wallet (MOVE_FUNDS), confirmer, action_pipeline, round_up | workflow, ledger, gateway |
| Orchestration | `outbox` | relay outbox rows to Redis streams | workflow |
| Domain | `transfer` | P2P and generic engine request (`ExecuteEngineRequest`), timeline, fee preview | fee, ledger, offerguard, vault, wallet, workflow |
| Domain | `wallet` | wallets, history, top-up, bank callback | ledger, user |
| Domain | `moneyrequest` | money requests, split, expiry sweeper | fee, ledger, wallet, workflow |
| Domain | `org` | organizations, members, policies, disbursement | approval, fee, ledger, wallet, workflow |
| Domain | `approval` | maker-checker engine: tiers, OTP challenge, decide, expiry sweeper, admin approvals | auth, listing, workflow |
| Domain | `kyc` | submission, AI scorer, classifier, admin review | listing, storage, workflow |
| Domain | `fraud` | rule engine step + admin flags; LLM advisor | ai, listing, workflow |
| Domain | `reversal` | admin-initiated reversal (executed on admin approval) | approval, ledger, wallet |
| Domain | `recon` | nightly top-up reconciliation, xlsx report, cron | ai, storage |
| Domain | `vault` | vault wallets metadata | - |
| Domain | `product` | admin service catalog and offers | listing |
| Domain | `fee` | fee policy lookup and calculation | - |
| Domain | `offerguard` | per-offer service limits and auth method | - |
| Domain | `auth` | register/login, argon2id, JWT issuer, refresh rotation, OTP, admin login | - |
| Core | `ledger` | atomic double-entry posting with lock ordering | - |
| Support | `ai` | LLM advisor clients (Claude, OpenAI) | - |
| Support | `gateway` | outbound HTTP gateway and mock | - |
| Support | `storage` | Store interface, local disk, Cloudinary, HMAC signer | - |
| Support | `qr`, `listing`, `user` (masking), `testutil` | helpers | - |
| Platform | `platform/config`, `db`, `logger`, `redisclient` | config loading, pgx pool, slog JSON logger, Redis | - |

Dependency direction is strictly downward: `api` -> domain -> `ledger`/`workflow` -> `store/gen`. No package imports `api`. `approval` imports `workflow` (pause hook) and `workflow` does not import `approval`; the link is the `PauseHook` interface set via `engine.SetPauseHook` (`main.go:131`).

## Process roles (`--role`)

`flag.String("role","all")` at `cmd/server/main.go:36`; semantics in `platform/config/config.go:32-41`:

| Role | HTTP server | `/v1/*` public routes | `/v1/org/*`, `/v1/admin/*`, `/v1/files/*` | Background loops |
|---|---|---|---|---|
| `api` | yes | yes | no | no |
| `admin` | yes | no (but `/healthz` yes) | yes | no |
| `worker` | no | no | no | yes |
| `all` (default, Docker CMD) | yes | yes | yes | yes |

Note `/v1/org/*` (business-user endpoints used by the mobile app) is mounted with the *admin* role (`router.go:142-162`), so a deployment with only `api` + `worker` roles would break the mobile business screens.

Background loops started when `RunsWorkers()` (`main.go:146-151`), all plain goroutines tied to one cancel context:

| Goroutine | Interval | File |
|---|---|---|
| `outbox.Relay.Run` | 200 ms tick, batch 100 | `outbox/relay.go:30` |
| `workflow.Worker.Run` | Redis stream consumers (XREADGROUP) | `workflow/consumer.go:45` |
| `workflow.Recovery.Run` | 30 s: XAUTOCLAIM + sweep of instances idle > 2 min | `workflow/recovery.go:15-18` |
| `approval.Sweeper.Run` | 30 s, expires overdue approval requests | `approval/sweeper.go` |
| `moneyrequest.Sweeper.Run` | 30 s, expires unpaid money requests | `moneyrequest/sweeper.go` |
| `recon.Cron.Run` | daily at 00:15 server-local time, reconciles "yesterday" | `recon/cron.go:22-50` |

There is no leader election or lease: running two `worker`/`all` replicas runs every loop twice (outbox claim uses `FOR UPDATE SKIP LOCKED` and the recon run is protected by a unique `run_date`, so these two are safe; the sweepers are guarded by SQL state transitions; see audit section "asynq/golang-migrate" note on duplicate cron).

Startup (`main.go:45-153`): load config (fails fast on missing `DATABASE_URL`/`JWT_SECRET`), build JSON logger, connect Postgres (10 s ping) and Redis, optionally start HTTP, optionally register handlers (validator, wallet, confirmer; `kyc.RegisterHandlers`; `fraud.RegisterHandler`; `RegisterActionPipeline`; `RegisterRoundUp`), then wait for SIGINT/SIGTERM. Shutdown only drains the HTTP server (10 s); worker goroutines stop when the process exits.

Workflow definitions are DB rows (`workflow_definitions.steps` JSONB): `p2p_transfer` v3 (VALIDATE, FRAUD_CHECK, ACTIONS_BEFORE_GL, MOVE_FUNDS, ACTIONS_AFTER_GL, CONFIRM, ROUND_UP; migration 000010), `org_disbursement` v2 (VALIDATE, FRAUD_CHECK, ACTIONS_BEFORE_GL, WAIT_APPROVAL (`human_approval`), MOVE_FUNDS, CONFIRM), `kyc_verification` v1 (SCORE, CLASSIFY). Which definition a service uses comes from `services.workflow_name` (migration 000006).

## Admin app module

`src/admin` (see [admin-app.md](admin-app.md)): React 18, Vite 5, react-router-dom 7, TanStack Query and Table, zustand, Tailwind 3; typed API via `openapi-typescript`; Playwright e2e in `e2e/`. It talks only to `/v1/admin/*` and `/v1/files/*`.

## Mobile app module

`src/mobile` (see [mobile-app.md](mobile-app.md)): Expo 54, React Native 0.81, expo-router 6, NativeWind, TanStack Query, zustand, react-hook-form + zod, expo-secure-store, expo-local-authentication. Talks to `/v1/*` and `/v1/org/*`.

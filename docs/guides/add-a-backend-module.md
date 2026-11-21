---
title: Add a backend module
type: guide
status: verified-against-code
last_verified: 2026-11-21
---

# Add a backend module

Sources: `src/backend/internal/*`, `cmd/server/main.go`, `internal/api/router.go`, `sqlc.yaml`, `internal/vault` and `internal/moneyrequest` as small examples, recipe `references/module.md`.

A "module" is a package under `src/backend/internal/<name>/`. Existing ones: `ai api approval auth fee fraud gateway kyc ledger listing moneyrequest offerguard org outbox platform product qr recon reversal storage store testutil transfer user vault wallet workflow`.

## Stack facts

Go 1.25, Gin, pgx v5 + sqlc (queries in `internal/store/queries/*.sql`, generated code in `internal/store/gen`, package `store`), Redis Streams through the outbox relay, in-process goroutine schedulers. There is no asynq, no testcontainers, no golang-migrate library in `go.mod` (see ADR 0005).

## Steps

1. Read the relevant `docs/domain/*.md`. If the feature needs schema changes, do [add-a-migration.md](add-a-migration.md) first.
2. Create the package. Real modules are small and flat. Typical files (take only what you need):
   ```
   internal/<module>/
     service.go        # type Service { pool *pgxpool.Pool; q *store.Queries }, func NewService(pool, ...) *Service
     errors / var block # sentinel errors (var ErrX = errors.New("..."))
     <module>_test.go  # unit tests, same package
   ```
   `vault` is the minimal example: `New(pool)`, sentinel errors at the top, owner-scoped queries.
3. SQL: add `internal/store/queries/<module>.sql` with sqlc annotations (`-- name: GetX :one`). No raw SQL strings in Go business code (a few modules still do, avoid adding more). Run `sqlc generate`.
4. Business rules in the service. Conventions that the code enforces or relies on:
   - Money is `int64` minor units (VND has no minor unit, 1 = 1 dong). Never floats.
   - Money moves only through the engine (workflow with the `wallet` handler) or `ledger.Post`; never UPDATE balances directly. `ledger_entries` is append-only (DB trigger).
   - Status changes go through guarded queries (`WHERE status IN (...)`, check the affected row count), see `FailTransactionGuarded`.
   - Row-level ownership in queries, not only in handlers.
   - Events: write to the outbox in the same transaction (`InsertOutbox`); never `XADD` yourself. Only `internal/outbox` relays to Redis.
   - If the module starts a workflow: `engine.Start(ctx, tx, definitionName, txnID, initCtx)` inside the same tx as your business rows (`kyc.Service` and `transfer.Service` do this).
5. HTTP: follow [add-an-endpoint.md](add-an-endpoint.md): handler in `internal/api/<module>_handler.go`, sentinel errors in `errmap.go`, routes in `router.go`, `api/openapi.yaml`.
6. Wiring in `cmd/server/main.go` (workers) and `internal/api/router.go` (HTTP):
   - HTTP services are constructed inside `NewRouter` under `MountsPublicAPI()` or `MountsAdminAPI()`.
   - Workflow handlers: register in `main.go` inside `if cfg.Role.RunsWorkers()`.
   - Background jobs (sweepers, crons): create `NewSweeper/NewCron` with a `Run(ctx)` loop and start it with `go x.Run(workerCtx)` in `main.go` (examples: `approval.NewSweeper`, `moneyrequest.NewSweeper`, `recon.NewCron`). They are plain goroutines: with more than one replica they run twice (the recon cron has no lease).
7. Tests: unit tests colocated; cross-module scenarios in `test/integration/`. Run `go vet ./...`, `python scripts/check-openapi-sync.py`, `.\scripts\test-backend.ps1`.
8. Docs: add or update `docs/domain/<context>.md` (template: `docs/_templates/domain-doc.md`), add edge cases to the QA catalog.

## Checklist

- [ ] No sqlc struct in an API response
- [ ] Ownership enforced in SQL
- [ ] New error codes in `errmap.go` and `openapi.yaml`
- [ ] Every new or changed endpoint in `openapi.yaml` (CI enforces)
- [ ] Money endpoints use `RequireIdempotencyKey()`
- [ ] Registered for the right `--role`
- [ ] Domain doc updated

## Recipe check (module.md) and corrections

| Recipe claim | Finding |
|--------------|---------|
| "jobs via **asynq**" | WRONG. No asynq in `go.mod`; jobs are in-process goroutines started in `cmd/server/main.go` |
| "Go 1.22+" | `go.mod` requires 1.25.12 |
| Layout `service.go`, `models.go`, `errors.go`, `service_test.go` | Aspirational. Real modules vary (often `service.go` plus domain files); `models.go`/`errors.go` are not conventions |
| "All state transitions go through `transition.Apply()`-style function with an allowed-transitions map" | Not generally true. Transitions are guarded SQL updates and per-module logic (e.g. `FailTransactionGuarded`, workflow `OnResult` guard); there is no shared `transition` package |
| "errors mapped in `internal/api/errmap.go`" | Correct (`errTable`) |
| "`outbox.Enqueue(tx, event)`" | There is no such function. Use the generated query `InsertOutbox` (`store.InsertOutboxParams`) in the business transaction |
| "Routes under `/v1/*` public (JWT), `/v1/org/*`, `/v1/admin/*`" | Correct, mounted by role (`MountsPublicAPI`, `MountsAdminAPI`) |
| "Open `docs/index.html` section for this feature and the schema in doc 00" | Those are history now: `docs/history/design-v2/`. Use `docs/domain/*` |
| "`golangci-lint run`" | Not part of CI (TODO in the workflow); `go vet` and gofmt are the enforced gates |
| "Module registered in `cmd/server/main.go` wiring" | Only worker-side pieces; HTTP services are wired in `router.go` |

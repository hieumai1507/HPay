# Scaffold a backend module

> Trigger: Scaffold a new backend module for hpay-backend following project conventions. Trigger when the user asks to "create module X", "add a new domain/feature module", or when implementing a feature that has no existing folder under internal/.

# scaffold-module — Generate a complete module skeleton

## Project conventions (MUST follow)

- Language: Go (version pinned in `src/backend/go.mod`, currently 1.25.x), HTTP via **Gin**, DB via **sqlc + pgx/v5**, events via the **outbox + Redis Streams**. Background jobs are plain goroutines started in `cmd/server/main.go` (there is NO asynq; see `docs/adr/0005-*`).
- Layout: one folder per domain under `internal/<module>/` (e.g. `internal/approval/`).
- SQL lives in `internal/store/queries/<module>.sql`; NEVER write raw SQL strings inside Go files.
- Money is always `int64` (minor units). NEVER float. NEVER string arithmetic.
- State transitions go through the owning package's function, which guards on the expected current state in SQL (`UPDATE ... WHERE status = ...`) and checks the row count. There is no shared `transition` package; follow the pattern in `internal/approval` or `internal/moneyrequest`.
- Errors: return typed sentinel errors (`ErrInsufficientBalance`) mapped to API codes (`ERR_INSUFFICIENT_BALANCE`) in one place: `internal/api/errmap.go`.

## Steps

1. **Read context first.** Read `docs/domain/<context>.md`, `docs/architecture/building-blocks.md` (dependency direction) and `docs/reference/data-model.md`. Confirm which tables/queries the module needs. If schema changes are required, STOP and follow `references/migration.md` first. A new package under `internal/` is an architecture change: classify it in the spec (`hpay-feature` stage 2).
2. Create the skeleton:
   ```
   internal/<module>/
   ├── service.go        # business logic; constructor NewService(store, bus, cfg)
   ├── service_test.go   # table-driven unit tests for pure logic
   ├── models.go         # domain structs (NOT sqlc structs — map explicitly)
   └── errors.go         # sentinel errors for this module
   internal/api/<module>_handler.go   # Gin handlers, DTO in/out with validator tags
   internal/store/queries/<module>.sql
   ```
3. Write the `.sql` queries with sqlc annotations (`-- name: GetX :one`). Run `make sqlc`. Fix compile errors.
4. Wire routes in `internal/api/router.go` under the correct group:
   - `/v1/*` public (JWT), `/v1/org/*` (org membership middleware), `/v1/admin/*` (admin JWT).
   - Any POST that creates money movement MUST use the idempotency middleware.
5. If the module emits/consumes events: publish ONLY via the outbox, i.e. the sqlc query `InsertOutbox` inside the same transaction as the state change (there is no `outbox.Enqueue` helper), never direct `XADD`.
6. Add at least one happy-path handler test (see `gen-api-test` skill for the pattern).
7. Run `go build ./... && go vet ./... && gofmt -l cmd internal` and the tests through `.\scripts\test-backend.ps1` (golangci-lint is currently disabled in CI; do not claim it ran). All green before reporting done.

## Checklist before finishing

- [ ] No sqlc-generated struct leaks into API DTOs
- [ ] Row-level ownership enforced in queries (`WHERE owner_id = $current_user`), not only in handlers
- [ ] New error codes added to `errmap.go` AND documented in `api/openapi.yaml`
- [ ] EVERY new/changed endpoint added to `api/openapi.yaml` in the SAME commit — no exceptions (user rule)
- [ ] Module registered in `cmd/server/main.go` wiring for the correct `--role`

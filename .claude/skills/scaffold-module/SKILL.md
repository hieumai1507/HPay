---
name: scaffold-module
description: Scaffold a new backend module for hpay-backend following project conventions. Trigger when the user asks to "create module X", "add a new domain/feature module", or when implementing a feature that has no existing folder under internal/.
---

# scaffold-module — Generate a complete module skeleton

## Project conventions (MUST follow)

- Language: Go 1.22+, HTTP via **Gin**, DB via **sqlc + pgx/v5**, events via **Redis Streams**, jobs via **asynq**.
- Layout: one folder per domain under `internal/<module>/` (e.g. `internal/approval/`).
- SQL lives in `internal/store/queries/<module>.sql`; NEVER write raw SQL strings inside Go files.
- Money is always `int64` (minor units). NEVER float. NEVER string arithmetic.
- All state transitions go through the module's `transition.Apply()`-style function with an explicit allowed-transitions map.
- Errors: return typed sentinel errors (`ErrInsufficientBalance`) mapped to API codes (`ERR_INSUFFICIENT_BALANCE`) in one place: `internal/api/errmap.go`.

## Steps

1. **Read context first.** Open `hpay-docs/index.html` section for this feature and the schema in doc 00. Confirm which tables/queries the module needs. If schema changes are required, STOP and run `gen-migration` first.
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
5. If the module emits/consumes events: publish ONLY via the outbox (`outbox.Enqueue(tx, event)`), never direct `XADD`.
6. Add at least one happy-path handler test (see `gen-api-test` skill for the pattern).
7. Run `go vet ./... && golangci-lint run && go test ./...` — all green before reporting done.

## Checklist before finishing

- [ ] No sqlc-generated struct leaks into API DTOs
- [ ] Row-level ownership enforced in queries (`WHERE owner_id = $current_user`), not only in handlers
- [ ] New error codes added to `errmap.go` AND documented in `api/openapi.yaml`
- [ ] EVERY new/changed endpoint added to `api/openapi.yaml` in the SAME commit — no exceptions (user rule)
- [ ] Module registered in `cmd/server/main.go` wiring for the correct `--role`

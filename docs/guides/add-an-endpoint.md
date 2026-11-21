---
title: Add an HTTP endpoint
type: guide
status: verified-against-code
last_verified: 2026-11-21
---

# Add an HTTP endpoint

Sources: `src/backend/internal/api/router.go`, `errmap.go`, `middleware.go`, `vault_handler.go`, `request_handler.go`, `scripts/check-openapi-sync.py`, `test/integration/transaction_request_http_test.go`, recipe `references/api-test.md`.

## Surfaces and where routes go

`api.NewRouter` mounts groups by process role:

| Group | Variable in `router.go` | Prefix | Auth | Role |
|-------|-------------------------|--------|------|------|
| public auth | `ag` | `/v1/auth` | none, rate limited | `api`, `all` |
| user | `me` | `/v1` | `RequireAuth(issuer)` (user JWT) | `api`, `all` |
| org | `og` | `/v1/org` | `RequireAuth(issuer)`; membership checked in the service layer | `admin`, `all` |
| admin | `adg` | `/v1/admin` | `RequireAdmin(adminIssuer)` (audience `admin`) | `admin`, `all` |

## Steps

1. Spec first: for a new feature follow `hpay-feature` (spec, plan, test cases). Add the endpoint to `src/backend/api/openapi.yaml` in the same change.
2. Add (or reuse) a service method in the owning `internal/<module>/`. Business rules, ownership checks and SQL access live there, not in the handler. Scope queries by owner (`WHERE owner_id = $user`) so a foreign id returns "not found" (pattern: `vault.ErrNotFound`).
3. Add sentinel errors in the module and map each in the `errTable` in `internal/api/errmap.go` (`ERR_*` code + HTTP status). Unmapped errors become `500 ERR_INTERNAL`. Do not match on error text.
4. Add the handler in `internal/api/<module>_handler.go`:
   - Request DTO with Gin `binding` tags; bind with `c.ShouldBindJSON`, on error `respondValidation(c, err.Error())`.
   - Get the caller with `mustUserID(c)`; call the service; on error `respondErr(c, err)`; otherwise `c.JSON(...)`.
   - Never return sqlc structs directly; map to response structs.
   - Money is `int64` minor units in JSON.
5. Register the route in `router.go` on the right group. For anything that creates money movement add `RequireIdempotencyKey()` (it requires an `Idempotency-Key` header of at most 64 characters and returns `400 ERR_IDEMPOTENCY_KEY_REQUIRED` otherwise). Rate limits: `RateLimit(rdb, n, window)` per IP and route, `RateLimitUser` per user.
6. Update `api/openapi.yaml`: path, method, tags, security, and a response entry for each `ERR_*` code. The CI gate `scripts/check-openapi-sync.py` compares every `r/v1/me/org/adm/adg/...` route in `router.go` against the spec and fails on drift either way. It only recognises these router variable names: `r`, `v1`, `ag`, `me`, `org`, `og`, `adm`, `adg`, `fg`. Use one of them or extend the script's `group_prefix`.
   ```powershell
   cd src\backend
   python scripts\check-openapi-sync.py     # expected: "openapi sync OK: 68 operations match" (before your change)
   ```
7. Write tests (see below). Run `go vet ./...` and `.\scripts\test-backend.ps1`.
8. If admin or mobile consume the endpoint, regenerate or update the typed client (`npm run gen:api` in `src/admin`) and the mobile API layer.

## Tests

- Cross-module, black-box tests go in `test/integration/` (package `integration`) using the harness in `helpers_test.go` (`newEnv`, `userWithMoney`, `drain`, `postTransfer`, `txnLedgerNet`). Handler-level tests build a small gin router with the real handler and middleware (see `transactionRequestRouter` in `transaction_request_http_test.go`, which injects `user_id` into the context and mounts `RequireIdempotencyKey`).
- Single-package pure logic: colocated `*_test.go`.
- Every endpoint suite: happy path with DB assertions (ledger rows, balance, status), idempotent retry (same key twice gives one effect), authorization (another user's resource is invisible), validation (0, negative, over limit, self transfer), async settlement for engine flows (`env.drain(t)` pumps the outbox then assert the `step_executions` trail).
- Edge cases: name tests `TestEdge_<ID>_<slug>` (CI greps `func TestEdge_E..._` for E01 to E20).
- Tests need `TEST_DATABASE_URL`, otherwise they skip.

## Known gaps to keep in mind (audit)

- Idempotency keys are globally unique, not per user, and not bound to the payload (BE-H1). `RequireIdempotencyKey` is only attached at two routes: `POST /transactions/request` and `POST /org/:id/transactions`; `POST /money-requests`, `/money-requests/split`, `POST /vaults` have none (confirmed in the audit appendix).
- `RequireAdmin` grants every admin every permission (BE-H6); new admin routes must add their own role check until RBAC exists.
- `/transactions/confirm` and `/verify` do not verify PIN or OTP (BE-C2).

## Recipe check (api-test.md)

| Recipe claim | Finding |
|--------------|---------|
| Helpers `testutil.NewUser(t, kycTier)`, `testutil.Fund`, `testutil.Do(...)`, `testutil.WaitTxnStatus(...)` | DO NOT EXIST. `internal/testutil` has `PoolFromEnv`, `NewUserWithWallet`, `SystemWallet`, `WalletBalance`, `LedgerSum`, `RunSerialized`. The richer helpers are in `test/integration/helpers_test.go` (`newEnv`, `userWithMoney`, `drain`, `fund`, ...) |
| "Each test gets an isolated DB schema OR truncates tables" | Not true: tests share one database and create random users; isolation is by unique data, not by schema |
| Setup "docker compose up -d && make migrate-up" | Correct; on this machine use `scripts/test-backend.ps1` |
| `go test` with `-race` always | CI uses `-p 1 ./... -race` |
| Examples cite `internal/auth/otp_test.go` and `internal/workflow/integration_test.go` | Both exist |

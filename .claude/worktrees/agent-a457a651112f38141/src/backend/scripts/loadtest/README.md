# Load tests

**Use the Go load test.** `make loadtest` runs `test/loadtest/p2p_load_test.go` (build tag `loadtest`). It drives the real service path: `ExecuteEngineRequest`, then `ExecuteEngineConfirm` with the PIN, then the workflow through the outbox relay, Redis Streams and the workflow workers, and it checks ledger integrity at the end (every wallet balance equals its ledger sum; the money moved between users equals the fees charged).

```
createdb load            # a FRESH database, migrated: make migrate-up with DB_URL pointing at it
LOAD_USERS=200 LOAD_WORKERS=50 LOAD_SECONDS=60 make loadtest
```

Results and history: `docs/quality/performance/`.

## `k6-transfer.js` is stale and not wired up
It calls `POST /v1/transactions/transfer` and `POST /v1/wallets/topup`, which the router no longer has. The current flow is `POST /v1/transactions/request` followed by `POST /v1/transactions/confirm` with the PIN. An HTTP load test also hits the rate limits: register and login are limited to 5 per minute per IP and confirm to 20 per minute per user, so it needs many users or a load-test configuration with higher limits. Rewrite it for request plus confirm before using it.
[← HPay Docs](../../../overview-v2.md) · repo `hpay-backend` · plan

# Implementation Plan — Vaults through the Engine

> **Feature slug:** `vaults-engine` · **Phase:** C (backend half) · **Source:** [doc 04 §2.2 — Vaults (P2). Mobile half: `src/mobile/docs/plans/m6-vaults-hardening/`.

## 1. Current state (surveyed)

| Component | Reality | Conclusion |
| --- | --- | --- |
| `wallets.purpose` / `label` | Already present (`main` \| `vault`, label VARCHAR(60)) | ✅ no column needs to be added |
| `txn_type` | Already has the value `vault_move` | ✅ use it as is |
| Service catalog | `VAULT_SAVINGS` already seeded, `wallet_strategy='user_to_vault'`, `ledger_txn_type='vault_move'` | ✅ already configured |
| `planWallets` | `user_to_vault` is **not implemented** — it falls into `default` and returns an error | ❌ must be done |
| `wallet_owner_unique` | `UNIQUE (owner_type, owner_id, currency, purpose)` — **no `label`** | ❌ each user can create only **1** vault; doc 04 says the unique constraint must include `label` |
| Seeder `vaultWallet` | Already queries by `label` | ⚠️ assumes multiple vaults — contradicts the current constraint |
| API | No vault endpoint at all | ❌ must be done |

## 2. Scope & Non-goals

### Ships

- Relax `wallet_owner_unique` so that a user can have multiple vaults (distinguished by `label`).

- `planWallets` supports both the **deposit** and **withdraw** directions between the `main` wallet and a specified vault.

- API: list vaults, create a vault. Deposits/withdrawals go through exactly the 3 engine APIs with `service_code=VAULT_SAVINGS` — there is no dedicated money endpoint.

### Non-goals (defer)

- **Round-up** (doc 04 marks it stretch): requires adding an optional `ROUND_UP` step in the workflow definition — a separate plan.

- The savings goal (`target_amount`) is only a display column; there is no locking/enforcement logic.

- Interest / terms — out of scope.

## 3. Task breakdown

| # | Task | Files | Size |
| --- | --- | --- | --- |
| T1 | Migration 000008: change `wallet_owner_unique` to `(owner_type, owner_id, currency, purpose, label)`; add `target_amount BIGINT` for vaults | `db/migrations/000008_*.up/down.sql` | M |
| T2 | Queries: `ListUserVaults`, `CreateVaultWallet`, `GetVaultWalletForUser` (with an owner condition — AuthZ at the SQL layer) | `internal/store/queries/wallets.sql` | S |
| T3 | `planWallets`: strategy `user_to_vault` reads `extra_params.vault_id` + `extra_params.direction` (`deposit` \| `withdraw`) and builds the correct wallet pair | `internal/transfer/service.go` | M |
| T4 | Module `internal/vault`: list/create + label validation, block duplicate labels | `internal/vault/*.go` | M |
| T5 | Routes `GET /v1/vaults`, `POST /v1/vaults` + `ERR_*` into errmap + openapi | `internal/api/*.go`, `api/openapi.yaml` | M |
| T6 | Tests: deposit/withdraw balance the ledger, withdrawing more than the vault balance is blocked, another person's vault is not accessible, multiple vaults per user | `test/integration/vault_test.go` | L |

## 4. API changes

```text
GET  /v1/vaults                    → [{id, label, balance, target_amount, created_at}]
POST /v1/vaults {label, target_amount?}   → 201 {id, label, balance:0}

# Deposit/withdraw has NO dedicated endpoint — it goes through the engine:
POST /v1/transactions/request
  { "service_code": "VAULT_SAVINGS", "amount": 200000,
    "extra_params": { "vault_id": "...", "direction": "deposit" } }
POST /v1/transactions/confirm  { transaction_id, pin }
```

New error codes: `ERR_VAULT_NOT_FOUND`, `ERR_VAULT_LABEL_TAKEN`, `ERR_VAULT_DIRECTION_INVALID`. Withdrawing more than the vault balance reuses `ERR_INSUFFICIENT_BALANCE`.

## 5. Test plan

| Test | Asserts |
| --- | --- |
| `TestVault_DepositMovesMoneyAndBalances` | main decreases, vault increases, the user's total ledger is unchanged, 2 ledger entries |
| `TestVault_WithdrawReturnsToMain` | the reverse direction balances correctly |
| `TestVault_WithdrawOverBalanceRejected` | withdrawing more than the vault balance → `ERR_INSUFFICIENT_BALANCE`, no ledger entries at all |
| `TestVault_ForeignVaultRejected` | passing another user's `vault_id` → rejected (the owner condition lives in SQL, the handler is not trusted) |
| `TestVault_MultipleVaultsPerUser` | creating 2 vaults with different labels succeeds (proves T1) |
| Invariant | after every test, `balance = SUM(ledger_entries)` for every wallet |

## 6. Rollout & risk

- **Changing the UNIQUE constraint** is a schema change that could fail if existing data already violates it — but the new constraint is **looser** than the old one, so it is always safe.

- `label` is **nullable**: two `main` wallets of the same user both have `label IS NULL`, and in Postgres `NULL`s are not equal ⇒ the UNIQUE constraint will **no longer prevent** creating duplicate main wallets. This must be compensated with a partial unique index for `purpose='main'`.

- There is no dedicated money-transfer endpoint ⇒ vaults inherit the engine's OfferGuard + auth matrix + idempotency in full, with no shortcut opened.

- Rollback: the down migration restores the old constraint (accepting the loss of the 2nd and later vaults if they were created).

## 7. Execution checklist

- `impact()` on `planWallets` before editing.

- **gen-migration** → T1 → `make migrate-verify` (E18).

- `make sqlc` → T2; **scaffold-module** → T4.

- T3 → T5 → update `openapi.yaml` + `check-openapi-sync.py`.

- **gen-api-test** → T6.

- `detect_changes()` → **fintech-review** must be `READY` → PR.

---
HPay · plan generated by skill `gen-feature-plan` · repo hpay-backend · 2026-07-23

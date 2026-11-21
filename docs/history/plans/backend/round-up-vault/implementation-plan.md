[← HPay Docs](../../../overview-v2.md) · repo `hpay-backend` · plan

# Implementation Plan — Round-up Vault Step

> **Feature slug:** `round-up-vault` · **Phase:** C follow-up (deferred from [vaults-engine](../vaults-engine/implementation-plan.md)) · **Source:** [doc 04 §2.2: "round-up — every small spending transaction automatically moves the rounded-up difference into a vault (add one optional `ROUND_UP` step in the definition)". Marked **stretch/P2** — the main goal is to showcase an engine that is configurable through workflow definitions, not to build a full savings product.

## 1. Current state (surveyed)

| Component | Reality | Conclusion |
| --- | --- | --- |
| Service `VAULT_SAVINGS` | Already seeded with `full_name='Vault Savings & Round-up'`, `action_priority='after_gl'` | ⚠️ The name suggests round-up belongs to the vault, but round-up actually has to run on a **spending** transaction (P2P), not on the vault's own deposit/withdraw transactions |
| `workflow_definitions.p2p_transfer` v1 | 4 steps: VALIDATE → FRAUD_CHECK → MOVE_FUNDS → CONFIRM | ❌ no place for ROUND_UP yet |
| `ActionPipeline` handler (`internal/workflow/handlers/action_pipeline.go`) | Already coded and registered in the registry, but **no workflow definition references the `action_pipeline` handler** | ⚠️ Pre-existing gap, outside the scope of this task — not touched, only noted |
| `wallets` | Has `purpose`/`label`/`target_amount` (migration 000008); no flag for "this is the vault that receives round-up" | ❌ a column must be added |
| `ledger.Post` | Accepts multiple legs for the same `transaction_id` (already used for the fee leg) — no "post only once" assumption | ✅ calling it once more from the ROUND_UP step is safe |
| `FailTransactionGuarded` | `WHERE status IN ('created','processing','pending_approval','processing_async')` — excludes `completed` | ✅ if ROUND_UP (which runs after CONFIRM) hard-fails, the original transaction stays `completed`; only the instance timeline shows the last step as "failed" — acceptable because the step is optional |

## 2. Scope & Non-goals

### Ships

- Column `wallets.round_up_target BOOLEAN` — each user has at most 1 round-up vault at a time (partial unique index).

- API for the user to select/deselect a vault as the round-up target.

- Step handler `round_up`: after `CONFIRM` of `p2p_transfer`, compute the remainder of rounding `txn.Amount` up to a configured multiple (default 10,000 VND), and if the user has a round-up target and enough balance, post one additional main→vault pair of ledger entries. Every unmet condition (not enabled, insufficient balance, evenly divisible) is **skipped silently** — it never fails the main transaction.

- `workflow_definitions.p2p_transfer` version 2 (version 1 is set `is_active` = false) adds the `ROUND_UP` step.

- Mobile: a "Use as round-up vault" toggle on the existing Vault detail screen.

### Non-goals (defer)

- Applying round-up to services other than `P2P_TRANSFER` (MONEY_REQUEST, ORG_DISBURSEMENT, ...).

- User-customizable rounding unit — a single system-wide constant is used (env-configurable); there is no UI to choose 1,000/10,000/50,000.

- Fixing the pre-existing gap where `ActionPipeline` is not wired into any workflow definition — noted, not fixed in this plan.

## 3. Task breakdown

| # | Task | Files | Size | Depends on |
| --- | --- | --- | --- | --- |
| T1 | Migration 000009: `wallets.round_up_target` + partial unique index; `workflow_definitions` version 2 of `p2p_transfer` (add step `ROUND_UP`, deactivate version 1) | `db/migrations/000009_round_up_vault.up/down.sql` | M | — |
| T2 | Queries: `SetRoundUpTarget` (unset the old one + set the new one in 1 transaction, owner-scoped), `GetRoundUpTargetVault` (owner-scoped) | `internal/store/queries/wallets.sql` | S | T1 |
| T3 | `vault.Service`: `SetRoundUpTarget(ctx, userID, vaultID, enabled)`, `RoundUpTarget(ctx, userID) (Vault, bool, error)` | `internal/vault/vault.go` | S | T2 |
| T4 | Step handler `round_up`: read the txn via the workflow context, compute the remainder, fetch the target vault, check the balance, post 2 legs through `ledger.Post`. Never returns a terminal error for business reasons (only infrastructure errors produce a `RetryableError`) | `internal/workflow/handlers/round_up.go` (new, using skill `gen-workflow-step`) | L | T1, T3 |
| T5 | Register the handler in the registry (`RegisterAll` or a dedicated function such as `RegisterActionPipeline`) | `internal/workflow/handlers/handlers.go`, call site in `cmd/server/main.go` | S | T4 |
| T6 | API: `PATCH /v1/vaults/{id}/round-up-target` {enabled} → toggle; returns `ERR_VAULT_NOT_FOUND` when not owned | `internal/api/*.go`, `api/openapi.yaml` | M | T3 |
| T7 | Mobile: toggle button in Vault detail, calls the T6 API, shows a "Receives round-up" badge on the enabled vault | `src/mobile/app/(main)/vaults/[id].tsx`, `src/queries/useVaultQueries.ts` | M | T6 |
| T8 | Tests: rounding to the correct multiple, evenly divisible is skipped, no target is skipped, insufficient funds for round-up is skipped (main transaction still completed), changing the target automatically unsets the old one | `test/integration/round_up_test.go` | L | T4–T6 |

## 4. Schema changes

```text
-- 000009_round_up_vault.up.sql
ALTER TABLE wallets ADD COLUMN round_up_target BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX idx_wallet_round_up_target_unique
  ON wallets (owner_type, owner_id, currency)
  WHERE round_up_target AND purpose = 'vault';

UPDATE workflow_definitions SET is_active = false WHERE name = 'p2p_transfer' AND version = 1;

INSERT INTO workflow_definitions (name, version, steps, is_active) VALUES (
  'p2p_transfer', 2, '[
    {"name":"VALIDATE","type":"auto","handler":"validator","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"FRAUD_CHECK","type":"auto","handler":"fraud","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":3,"timeout_ms":5000,"on_failure":"compensate","config":{"compensatable":true}},
    {"name":"CONFIRM","type":"auto","handler":"confirmer","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"ROUND_UP","type":"auto","handler":"round_up","retry":3,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true
);
```

Following the `gen-migration` convention exactly: the version increases on the same `name`, v1 is not edited in place (in-flight instances still reference v1's definition_id).

## 5. API changes

```text
PATCH /v1/vaults/{id}/round-up-target
  Body: {"enabled": true}
  200: {"id","label","round_up_target": true}
  404: ERR_VAULT_NOT_FOUND (not owned / does not exist)
```

No money endpoint is added — round-up is not a user-initiated transaction; it is an automatic consequence of `MOVE_FUNDS` in `p2p_transfer`.

## 6. Test plan

| Case | Test |
| --- | --- |
| Basic rounding | `TestRoundUp_AppliesDifference` — transfer 123,000, unit 10,000 → round-up of 7,000 into the target vault |
| Evenly divisible | `TestRoundUp_ExactAmount_Skipped` — transfer 120,000 → no round-up ledger entries at all |
| No target | `TestRoundUp_NoTarget_Skipped` — user has not enabled any vault → step OK, balances unchanged |
| Insufficient funds for round-up | `TestRoundUp_InsufficientBalance_SkippedNotFailed` — after MOVE_FUNDS the remaining balance is < the remainder → step skips silently, the main transaction is still `completed` |
| Changing the target | `TestRoundUp_ChangingTarget_UnsetsOld` — enabling vault B automatically disables vault A (DB constraint + service logic) |
| Does not touch other users' vaults | `TestRoundUp_TargetIsOwnerScoped` — API PATCH on a vault not owned → 404 |

## 7. Rollout & risk

- **Feature flag:** not needed — by default a user who has not enabled a round-up target gets a step that is always a no-op, so it is safe to merge directly.

- **Backfill:** not needed, the new column defaults to `false`.

- **Known risk:** if the `round_up` handler hits an infrastructure error through all its retries, `failInstance` still sets `workflow_instances.status='failed'` at the `ROUND_UP` step even though `transactions.status` was already `completed` (due to the `FailTransactionGuarded` guard). The mobile timeline will show the instance as "failed" at the last step although the money has already moved — accepted because this is a non-mandatory step and the risk is low (only genuine infrastructure errors fall into this branch). `failInstance` is not changed in this plan because it affects the last step of every definition.

- **Rollback:** deactivate v2 (`is_active`) and re-enable v1 — new instances will have no ROUND_UP; in-flight v2 instances (rare, the workflow is short) complete normally on their own.

## 8. Execution checklist

- `gen-migration` → T1

- sqlc generate → T2

- `gen-workflow-step` → T4, T5

- Manual → T3, T6, T7

- `gen-api-test` for the T8 cases that have no existing pattern

- `fintech-review` before commit (float? idempotency? lock ordering? ledger UPDATE?)

---
HPay · plan generated by skill `gen-feature-plan` · repo hpay-backend · 2026-07-24

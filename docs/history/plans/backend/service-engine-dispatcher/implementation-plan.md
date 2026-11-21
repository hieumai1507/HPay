[← HPay Docs](../../../overview-v2.md) · repo `hpay-backend` · plan

# Implementation Plan — Dynamic Service Engine Dispatcher

> **Feature slug:** `service-engine-dispatcher` · **Phase:** A (post-audit hardening) · **Design source:** [doc 05 — Offer Factory + [doc 06 — Service Engine Consolidation. Closes gaps G1–G4 found in the docs↔code review.

## 1. Scope & Non-goals

### Ships

- **G1 — Dynamic dispatch:** `ExecuteEngineRequest` reads the `services` configuration by `service_code` to choose: (a) the ledger `txn_type`, (b) the matching `workflow_definition`, (c) the wallet-resolution strategy (debit source / credit target) according to `service_target`. No more hardcoded `p2p_transfer` / `TxnTypeP2p`.

- **G2 — OfferGuard:** a middleware/service that looks up `user_offers → offer_services` and enforces `min_amount, max_amount_per_txn, daily_limit, monthly_limit, interval_seconds, is_enabled` BEFORE the workflow is created.

- **G3 — Auth matrix from the DB:** the authentication level (pin/otp/2fa) is read from `offer_auth_matrix` by `(offer_service, amount)`, replacing the hardcoded 2M/50M block at `service.go:384–389`.

- **G4 — Action Pipeline executor:** a step handler reads the `services.actions` JSONB (already present in the schema) and runs the `GL Posting` + `Directional Integration (HTTP gateway)` actions in `action_priority` order (`before_gl`/`after_gl`), going through the engine + outbox.

### Non-goals (defer)

- Admin UI to assign offers to users (Phase B). In this phase only an API/seed for assignment is needed.

- Vaults `VAULT_SAVINGS` end-to-end (Phase C, v2.1).

- Changing the 3-step API contract — the endpoints `/v1/transactions/request\|confirm\|verify` stay unchanged (the router is already pruned correctly, verified).

## 2. Task breakdown

Each row ≈ 1 commit. Sizes: S < 100 LOC · M 100–300 · L > 300.

| # | Task | Files touched | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Migration 000006: `user_offers` table; add a `workflow_name` column + `debit_source`/`credit_target` strategy to `services`; seed the map for the 5 services + assign a default offer to the seed users | `db/migrations/000006_*.up/down.sql` | M | — |
| T2 | sqlc queries: `GetUserOffer`, `GetOfferServiceForUser`, `ListAuthMatrix`, `SumUserServiceVolume` (daily/monthly), `GetServiceDispatch` (code→type/def/target/actions) | `internal/store/queries/*.sql`, regenerate `store/gen` | M | T1 |
| T3 | **G1** Dispatcher: refactor `ExecuteEngineRequest` — resolve `txn_type`, def name and wallet strategy from the service; extract a `resolveWallets(strategy)` function | `internal/transfer/service.go` | L | T2 |
| T4 | **G2/G3** OfferGuard module: `Check(ctx, userID, serviceCode, amount) → (authMethod, error)`; wire into `ExecuteEngineRequest` (after fetching the service, before Begin tx) + `ExecuteEngineConfirm` | `internal/offerguard/*.go`, `internal/transfer/service.go` | L | T2 |
| T5 | **G4** Action Pipeline executor: step handler `action_pipeline` reads `services.actions`, runs GL Posting + HTTP gateway per `before_gl/after_gl`; a `Gateway` interface with a mock for dev/test | `internal/workflow/handlers/action_pipeline.go`, `internal/gateway/*.go` | L | T3 |
| T6 | Wire the auth method into the response `EngineRequestResult.AuthRequired` (from OfferGuard, not hardcoded) | `internal/transfer/service.go`, `internal/api/transaction_engine_handler.go` | S | T4 |
| T7 | Tests + openapi.yaml note + fintech-review | `test/integration/*`, `internal/offerguard/*_test.go`, `api/openapi.yaml` | L | T3–T6 |

## 3. Schema changes (migration 000006)

Most of the schema already exists (`services.actions`, `offer_services`, `offer_auth_matrix` in 000003/000004). Only the following needs to be added:

```text
-- Assign an Offer package to a user (missing — the source for the OfferGuard lookup)
CREATE TABLE user_offers (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    offer_id   UUID NOT NULL REFERENCES offers(id),
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Dispatch metadata for a service (missing — the source for G1)
ALTER TABLE services ADD COLUMN workflow_name VARCHAR(60) NOT NULL DEFAULT 'p2p_transfer';
ALTER TABLE services ADD COLUMN ledger_txn_type txn_type NOT NULL DEFAULT 'p2p';
ALTER TABLE services ADD COLUMN wallet_strategy VARCHAR(30) NOT NULL DEFAULT 'user_to_user';
-- wallet_strategy ∈ {user_to_user, syscash_to_user (topup), org_to_user (disbursement)}

-- Seed map
UPDATE services SET workflow_name='org_disbursement', ledger_txn_type='disbursement', wallet_strategy='org_to_user' WHERE code='ORG_DISBURSEMENT';
UPDATE services SET ledger_txn_type='topup', wallet_strategy='syscash_to_user' WHERE code='TOPUP_BANK';
-- Backfill: assign OFFER_CUST_L2 to every existing user
INSERT INTO user_offers (user_id, offer_id)
  SELECT id, 'b0000000-0000-0000-0000-000000000002' FROM users ON CONFLICT DO NOTHING;
```

**E18:** `make migrate-verify` (up→down→up on a clean DB) must be green before merge. Use the `gen-migration` skill to generate the up/down pair following the convention.

## 4. API changes

No new endpoints. Internal behavior changes + error codes:

| Endpoint | Change | New ERR_* |
| --- | --- | --- |
| `POST /v1/transactions/request` | Branches by `service_code`; OfferGuard runs first; `auth_required` comes from the DB | `ERR_SERVICE_DISABLED`, `ERR_LIMIT_DAILY`, `ERR_LIMIT_MONTHLY`, `ERR_LIMIT_PER_TXN`, `ERR_TOO_FREQUENT` (interval), `ERR_SERVICE_NOT_IN_OFFER` |
| `POST /v1/transactions/confirm` | Authenticates with exactly the `auth_method` returned by the DB (pin vs otp vs 2fa) | `ERR_AUTH_METHOD_MISMATCH` |

Update `api/openapi.yaml`: add the `ERR_*` codes to the error code enum + the `auth_required` field in the response schema.

## 5. Test plan

| Test | Goal | Edge |
| --- | --- | --- |
| `TestEngine_TopupBank` | service_code=TOPUP_BANK → topup def, debit SYS_CASH, credit user; does not require a receiver phone | — |
| `TestEngine_OrgDisbursement` | service_code=ORG_DISBURSEMENT → org_disbursement def, passes through WAIT_APPROVAL (human_approval) | E12 |
| `TestOfferGuard_DailyLimit` | Exceeding `daily_limit` → `ERR_LIMIT_DAILY`; under the limit → pass | E13 |
| `TestOfferGuard_Interval` | 2 transactions closer together than `interval_seconds` → `ERR_TOO_FREQUENT` | — |
| `TestAuthMatrix_FromDB` | Editing an `offer_auth_matrix` row → the response `auth_required` changes accordingly (not hardcoded) | — |
| `TestActionPipeline_BeforeGL` | The mock gateway is called in the correct before_gl order; gateway failure → step retry/compensate | E04 |
| `TestEdge_E20_RecoverStuckClaimLease` | Add the missing test: 2 workers recover at the same time, the claim-lease lets only 1 replay | E20 |

Use the `gen-api-test` skill (testcontainers: real Postgres + Redis). `check-edge-tests.sh` must cover E01–E20 after this phase.

## 6. Rollout & risk

- **Hub symbol:** `ExecuteEngineRequest` has a wide blast radius — run `impact({target:"ExecuteEngineRequest", direction:"upstream"})` and report before editing (expected HIGH). Keep the function signature stable, refactor internally.

- **Feature flag:** `ENGINE_DYNAMIC_DISPATCH` (env) — false = keep the old P2P path; enable gradually per service_code so rollback is fast if something breaks.

- **Half-done state:** if merged while T5 is not finished, the action pipeline gateway call is skipped but GL posting still runs (safe — no money is lost). OfferGuard is fail-closed: an offer lookup error → the transaction is rejected, never let through.

- **Backfill:** `user_offers` is backfilled with OFFER_CUST_L2 for existing users (idempotent).

- **Rollback:** migration 000006 down removes the added columns/table; turning the feature flag off restores the old behavior immediately.

## 7. Execution checklist (skill-loop)

- `impact()` on `ExecuteEngineRequest` + report the blast radius (mandatory, CLAUDE.md).

- **gen-migration** → T1 (000006) → `make migrate-verify`.

- `make sqlc` regen → T2 queries.

- Code T3 (dispatcher) → T4 (**scaffold-module** offerguard) → T5 (**gen-workflow-step** action_pipeline) → T6.

- **gen-api-test** → T7 tests; update `openapi.yaml`.

- `detect_changes({scope:"compare", base_ref:"main"})` → **fintech-review** → must be `VERDICT: READY`.

- If the code drifts from the plan → update these 2 docs in the same PR.

---
HPay · plan generated by skill `gen-feature-plan` · repo hpay-backend · 2026-07-22

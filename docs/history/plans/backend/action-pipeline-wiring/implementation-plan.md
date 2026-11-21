[← HPay Docs](../../../overview-v2.md) · repo `hpay-backend` · plan

# Implementation Plan — Wire Action Pipeline

> **Feature slug:** `action-pipeline-wiring` · **Source:** [doc 06 Task 2 + [mockup 07 (Action Priority column)](../../../../design/mockups/README.md). This is technical-debt cleanup, not a new feature — the code already exists, it just was never wired into any workflow definition.

## 1. Current state (investigated)

| Component | Reality | Conclusion |
| --- | --- | --- |
| `ActionPipeline` (`internal/workflow/handlers/action_pipeline.go`) | Fully implemented: reads `services.actions`, compares `action_priority` with the phase derived from the step name (`phaseOf`: name contains "AFTER_GL" → after_gl, otherwise before_gl), calls the gateway for actions of type "Directional Integration", records (does not re-post) for "GL Posting" | ✅ logic is correct, no change needed |
| Handler registration | `RegisterActionPipeline` is called in `cmd/server/main.go` (production) but is **not in the test harness** (`test/integration/helpers_test.go`) | ❌ must be added, otherwise every test using `p2p_transfer`/`org_disbursement` will hang because the step has no handler |
| `workflow_definitions.p2p_transfer` (currently v2, added by plan `round-up-vault`) | VALIDATE → FRAUD_CHECK → MOVE_FUNDS → CONFIRM → ROUND_UP | ❌ no step has a name matching the `action_pipeline` handler |
| `workflow_definitions.org_disbursement` v1 | VALIDATE → FRAUD_CHECK → WAIT_APPROVAL → MOVE_FUNDS → CONFIRM | ❌ same |
| Service catalog (migration 000003 + mockup 07) | `action_priority`: P2P_TRANSFER/MONEY_REQUEST/TOPUP_BANK/ORG_DISBURSEMENT = `before_gl`; VAULT_SAVINGS = `after_gl`. Only `P2P_TRANSFER.actions` has real content (`gl_posting` + `bank_gateway_notify`, the latter `required:false`); the other 4 services have `actions='[]'` | ✅ because actions are empty, wiring for the other 4 services is a complete no-op as soon as it is merged — the risk lies only in P2P_TRANSFER |

## 2. Scope & Non-goals

### Ships

- Add 2 step slots to `p2p_transfer` (version 3): `ACTIONS_BEFORE_GL` (before `MOVE_FUNDS`) and `ACTIONS_AFTER_GL` (after `MOVE_FUNDS`, before `CONFIRM`) — both use the `action_pipeline` handler; `phaseOf` reads the step name to derive the phase, so whichever phase a service is configured for is the step that actually runs, and the other skips itself.

- `org_disbursement` (version 2): add only `ACTIONS_BEFORE_GL` — the only service that uses this definition (ORG_DISBURSEMENT) has only `action_priority='before_gl'`; adding `ACTIONS_AFTER_GL` now would be a design for a hypothetical case that has not occurred.

- Register `RegisterActionPipeline` in the test harness with `gateway.NewMock()` — matches production behavior outside the `production` environment.

- A test confirming that actions really run (call the gateway) for P2P_TRANSFER, and a test confirming that the 4 empty-actions services still complete normally (regression, does not break the existing timeline).

### Non-goals (defer)

- Enabling a real `bank_gateway_notify` outside the mock — that is real gateway infrastructure, outside the scope of "wire into the definition".

- Changing the existing `action_priority` value of any service.

- The Admin UI "Action Pipeline Builder" (5-tab, doc 06 Tab 3) — backend runtime only.

## 3. Task breakdown

| # | Task | Files | Size | Depends on |
| --- | --- | --- | --- | --- |
| T1 | Migration 000010: `p2p_transfer` v3 (add 2 steps), `org_disbursement` v2 (add 1 step); set `is_active` to false on the corresponding old versions | `db/migrations/000010_wire_action_pipeline.up/down.sql` | M | — |
| T2 | Test harness: register `RegisterActionPipeline(reg, pool, gateway.NewMock())` in `newEnv` | `test/integration/helpers_test.go` | S | — |
| T3 | Fix the hard-coded timeline step-count assertions (`TestP2PTransferEndToEnd` currently expects 5, now becomes 7) | `test/integration/transfer_test.go` | S | T1 |
| T4 | New test: P2P_TRANSFER really calls the mock gateway for `bank_gateway_notify` (verified via the mock's log/counter), and the 4 empty-actions services still reach `completed` normally | `test/integration/action_pipeline_test.go` (new, separate from the existing unit test of the same name in `internal/workflow/handlers`) | L | T1–T3 |

## 4. Schema changes

```text
-- 000010_wire_action_pipeline.up.sql
UPDATE workflow_definitions SET is_active = false WHERE name = 'p2p_transfer' AND version = 2;
INSERT INTO workflow_definitions (name, version, steps, is_active) VALUES (
  'p2p_transfer', 3, '[
    {"name":"VALIDATE","type":"auto","handler":"validator","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"FRAUD_CHECK","type":"auto","handler":"fraud","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"ACTIONS_BEFORE_GL","type":"auto","handler":"action_pipeline","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":3,"timeout_ms":5000,"on_failure":"compensate","config":{"compensatable":true}},
    {"name":"ACTIONS_AFTER_GL","type":"auto","handler":"action_pipeline","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"CONFIRM","type":"auto","handler":"confirmer","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"ROUND_UP","type":"auto","handler":"round_up","retry":3,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true
);

UPDATE workflow_definitions SET is_active = false WHERE name = 'org_disbursement' AND version = 1;
INSERT INTO workflow_definitions (name, version, steps, is_active) VALUES (
  'org_disbursement', 2, '[
    {"name":"VALIDATE","type":"auto","handler":"validator","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"FRAUD_CHECK","type":"auto","handler":"fraud","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"ACTIONS_BEFORE_GL","type":"auto","handler":"action_pipeline","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"WAIT_APPROVAL","type":"human_approval","retry":0,"timeout_ms":86400000,"on_failure":"fail"},
    {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":3,"timeout_ms":5000,"on_failure":"compensate","config":{"compensatable":true}},
    {"name":"CONFIRM","type":"auto","handler":"confirmer","retry":3,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true
);
```

**Why `ACTIONS_BEFORE_GL` comes before `WAIT_APPROVAL` in `org_disbursement`:** "before GL" means before the money moves (`MOVE_FUNDS`), not necessarily before approval — but running it before approval avoids calling a 3rd-party action for a payroll disbursement order that has not been approved. If that action hard-fails, the saga fails early, before spending a 24h WAIT_APPROVAL round.

## 5. Test plan

| Case | Test |
| --- | --- |
| The pipeline really runs for P2P_TRANSFER | `TestActionPipelineWiring_P2PCallsGateway` — uses `gateway.Mock` to count `Call` invocations, asserts exactly 1 call for `bank_gateway_notify` at step `ACTIONS_BEFORE_GL` (the P2P_TRANSFER action_priority in the seed is `before_gl`, see §1); `ACTIONS_AFTER_GL` must report `skipped_phase_before_gl` and must not call the gateway at all |
| The 4 empty-actions services are unaffected | `TestActionPipelineWiring_EmptyActionsStillCompletes` — running P2P through `e.svc.Transfer` (MONEY_REQUEST/TOPUP_BANK share the same path) still reaches `completed`, with a full 7-step timeline |
| Regression of the whole existing suite | `go test ./test/integration/...` all green, especially the tests involving `org_disbursement` (approval_test.go) and vault (shares p2p_transfer) |

## 6. Rollout & risk

- **Main risk:** if in the future someone sets `required:true` on an after_gl integration action and that gateway fails, the step will return `RetryableError` and may then hard-fail after `MOVE_FUNDS` has already posted — the same caveat already recorded in plan `round-up-vault` for `ROUND_UP`. This plan does not change `failInstance`.

- **Rollback:** set `is_active` to false on v3/v2 and re-enable the old definitions — safe because all current actions are `required:false` or empty.

- **No feature flag needed** — 4 of 5 services have `actions=[]`, so the wiring is nearly a no-op as soon as it is merged; only P2P_TRANSFER has newly observable behavior (1 HTTP call to the mock/gateway).

## 7. Execution checklist

- `gen-migration` → T1

- Manual → T2, T3

- `gen-api-test` for T4

- `fintech-review` before committing

---
HPay · plan generated by the gen-feature-plan skill · repo hpay-backend · 2026-07-24

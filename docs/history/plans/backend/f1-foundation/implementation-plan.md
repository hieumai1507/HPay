[← Master Plan · f1-foundation

# F1 Foundation — Implementation Plan

Repo: `hpay-backend` · Phase 1/10 · Companion document: [solution-architecture.md](solution-architecture.md)

## 1. Scope & non-goals

| Ships | Not in F1 |
| --- | --- |
| Standard repo structure, go.mod, config/env, slog logger, pgx + redis connections, Gin `/healthz`, `--role` flag (api/worker/admin/all), **all schema migrations** (docs 00§3 + 04§2), sqlc setup, docker-compose, Makefile, CI (lint · sqlc vet · migrate up-down-up · test · build), openapi.yaml skeleton, skills copied into `.claude/skills/`, `check-edge-tests.sh` | Business logic (auth, ledger service, engine — F2+); F2 tables (refresh_tokens, admin_users, audit_logs — added via gen-migration in F2); handlers other than healthz; seed data |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | Repo skeleton + go.mod + gitignore + env example | `go.mod`, `.gitignore`, `.env.example` | S | — |
| T2 | Migrations 000001–000008 (up/down) with the full schema + append-only ledger trigger | `db/migrations/*` | L | T1 |
| T3 | Platform: config, logger, db pool, redis client | `internal/platform/*` | M | T1 |
| T4 | `cmd/server` with `--role`, router + `/healthz` (ping pg+redis) | `cmd/server/main.go`, `internal/api/*` | M | T3 |
| T5 | sqlc.yaml + initial queries (wallets) | `sqlc.yaml`, `internal/store/queries/wallets.sql` | S | T2 |
| T6 | docker-compose (pg16, redis7, app), Makefile | `docker-compose.yml`, `Makefile`, `Dockerfile` | M | T1 |
| T7 | CI pipeline + check-edge-tests.sh | `.github/workflows/ci.yml`, `scripts/*` | M | T2,T6 |
| T8 | openapi.yaml skeleton + README + copy skills | `api/openapi.yaml`, `README.md`, `.claude/skills/` | S | T1 |

## 3. Schema changes

8 migrations, content exactly per docs 00§3 merged with the 04§2 delta (not waiting for a separate migration): `wallets.purpose/label`, `transactions.reversal_of`, status `processing_async`, approval generalized into `approval_requests(subject_type, subject_id)`, plus `money_requests` and `fee_policies`.

| File | Content |
| --- | --- |
| `000001_users` | enum kyc_status, account_status · users table |
| `000002_orgs` | organizations, enum org_role, org_members |
| `000003_wallets_ledger` | enum owner_type/txn_type/txn_status (with processing_async)/entry_direction · wallets (purpose,label) · transactions (reversal_of, idempotency) · ledger_entries + append-only trigger |
| `000004_workflow` | workflow_definitions (unique active/name), wf_status, workflow_instances, step_status, step_executions |
| `000005_approval` | approval_policies · enum approval_subject/approval_status/approval_action · approval_requests (policy_snapshot, expires_at) · approval_actions (unique request+checker) · otp_challenges |
| `000006_kyc_fraud_recon` | kyc_requests, flag_status, fraud_flags, bank_statements, recon_result, reconciliation_runs/items |
| `000007_outbox` | outbox + partial index unpublished |
| `000008_requests_fees` | money_request_status, money_requests (split_group_id, request/paid txn), fee_policies |

## 4. API changes

Only `GET /healthz` (200 when the pg+redis pings are OK, 503 with details when degraded — to serve E17 later). openapi.yaml declares info + healthz + the standard `ERR_*` errorResponse schema.

## 5. Test plan

| ID | Test | Where |
| --- | --- | --- |
| E18 | Migrations up → down → up on a clean Postgres | CI job `migrate` (must be green) + `make migrate-verify` locally |
| — | `/healthz` returns 200/503 correctly | `internal/api/health_test.go` (runs when testcontainers are available — completed from F2) |
| — | Append-only ledger trigger blocks UPDATE/DELETE | SQL test in the CI migrate job (psql -c UPDATE → expect error) |

## 6. Rollout & risk

- New repo, no users yet — rollback = revert commit.

- Main risk: schema locked in too early turns out wrong → mitigated by two rounds of docs review; later changes go through the normal gen-migration.

- The sandbox has no Go toolchain: code is verified by review + CI is the real compile gate. The first commit needs `go mod tidy` run on a dev machine to generate `go.sum`.

## 7. Execution checklist

- [x] gen-feature-plan (this file + solution-architecture)

- [ ] T1→T8 in order

- [ ] SQL parse-check of all migrations

- [ ] fintech-review → VERDICT READY

- [ ] Commit: `feat: F1 foundation — repo skeleton, full schema migrations, CI`

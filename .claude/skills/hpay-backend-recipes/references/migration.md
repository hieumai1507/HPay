# Create a PostgreSQL migration

> Trigger: Create a PostgreSQL schema migration for hpay-backend. Trigger when the user asks to add/alter tables, columns, enums, or indexes, or when another skill discovers a schema change is needed.

# gen-migration — Schema change with up/down + sqlc sync

## Conventions

- Tool: **golang-migrate**, files in `db/migrations/`, named `NNNN_short_description.up.sql` / `.down.sql` (NNNN = next sequence number, zero-padded).
- The canonical schema reference is docs `architecture/00-backend-design-v2-base.html` §3 and `04-feature-extensions.html` §2. If the requested change conflicts with docs, STOP and tell the user docs must be updated first (docs-first rule).

## Hard rules for this codebase

1. Money columns: `BIGINT`, with `CHECK (col >= 0)` or `> 0` as appropriate. NEVER `NUMERIC` for amounts, NEVER float.
2. `ledger_entries` is immutable: any migration touching it may only ADD columns or indexes — never ALTER/DROP existing columns, never add UPDATE-enabling triggers.
3. Enums: create with `CREATE TYPE ... AS ENUM`; extending uses `ALTER TYPE ... ADD VALUE` (remember: cannot run inside a transaction block — put it in its own migration file).
4. Every FK gets an index if it will be filtered on. Partial indexes for hot status queries (`WHERE status IN (...)`).
5. Down migration must FULLY reverse the up (drop in reverse order). If truly irreversible, the down file contains a loud comment and fails intentionally.
6. New tables need: `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` (or IDENTITY for append-only logs), `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`.

## Steps

1. Determine next sequence number from `ls db/migrations/`.
2. Write `.up.sql` and `.down.sql`.
3. Verify locally against a scratch DB (docker-compose postgres):
   ```
   make migrate-up && make migrate-down && make migrate-up
   ```
   All three must succeed (up → down → up proves reversibility).
4. Update/add queries in `internal/store/queries/*.sql` if the change affects them, then `make sqlc` and fix compile errors across the repo.
5. If the change adds a status/state: update the transition map and its unit test in the owning module.
6. Report: migration number, tables touched, whether openapi.yaml or docs need a follow-up change.

## sqlc lessons learned (do not repeat)

- A table and an enum must not share a singular name: table `approval_actions` + enum `approval_action` collide as struct `ApprovalAction` (fixed: enum renamed `approval_decision`).
- Params inside expressions (`NULLIF($2,...)`, `COALESCE`) get meaningless generated names (`Column2`) — always use `sqlc.arg(name)` for those.
- UPDATE with a self-referencing subquery needs table aliases or sqlc reports "column reference is ambiguous".
- Nullable uuid columns map to `pgtype.UUID` by default; this repo overrides them to `uuid.UUID` — therefore NEVER SELECT a nullable uuid column directly, wrap with `COALESCE(col, '00000000-0000-0000-0000-000000000000'::uuid)`.
- After every query change: `sqlc generate` MUST run clean BEFORE `go build`; if build shows a wall of `store.X undefined`, the generate step failed — read its error first.

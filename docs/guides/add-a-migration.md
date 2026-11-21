---
title: Add a database migration
type: guide
status: verified-against-code
last_verified: 2026-11-21
---

# Add a database migration

Sources: `src/backend/db/migrations/`, `sqlc.yaml`, `Makefile`, `.github/workflows/backend-ci.yml` (job `migrate`), recipe `.claude/skills/hpay-backend-recipes/references/migration.md`.

Current state: 13 migrations (`000001` to `000013`); the next file is `000014_*`. The audit plans a `000014` for missing constraints (see `docs/audit/AUDIT-REPORT.md`, "Migration / DB"); check `ls db/migrations` before choosing a number.

## Steps

1. Check docs first. If the change contradicts `docs/domain/*` or the schema docs, stop and update the docs (docs-first rule).
2. Pick the next number. Files are `NNNNNN_short_description.up.sql` and `.down.sql`, six digits, e.g. `000014_ledger_unique_posting.up.sql`. Tool: golang-migrate (external `migrate` CLI; the server never migrates).
3. Write the `.up.sql`. Conventions that the existing migrations follow:
   - Money columns are `BIGINT` with `CHECK (amount > 0)` or `>= 0` (never `NUMERIC`/float).
   - `ledger_entries` is append-only (trigger from `000007`); only add columns or indexes, never alter or drop existing columns.
   - New enums: `CREATE TYPE ... AS ENUM`. Extending an enum: `ALTER TYPE ... ADD VALUE` must be the only statement in its own migration file (see `000011_approval_cancelled_status`).
   - New tables: UUID PK `DEFAULT gen_random_uuid()` (or `BIGINT GENERATED ALWAYS AS IDENTITY` for append-only logs) and `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`.
   - Index foreign keys you filter on; use partial indexes for hot status queries.
   - A unique constraint is the arbiter of a race, not a read-then-insert (example: `000013_policy_version_unique`).
4. Write the `.down.sql` that fully reverses the up in reverse order. Postgres cannot remove an enum value; for `ADD VALUE` migrations the down is a documented no-op or a rebuild. Say so loudly in a comment.
5. Prove reversibility on a scratch database (this wipes it):
   ```powershell
   cd src\backend
   migrate -path db/migrations -database "postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable" up
   migrate -path db/migrations -database "postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable" down -all
   migrate -path db/migrations -database "postgres://HPay:HPay@localhost:5432/HPay?sslmode=disable" up
   ```
   Equivalent: `make migrate-verify`. Expected: three successful runs. CI does the same and then proves the ledger triggers still block UPDATE and DELETE.
6. If queries are affected, edit `internal/store/queries/*.sql`, then `sqlc generate` (`make sqlc`). Run it BEFORE `go build`; a wall of `store.X undefined` means generate failed.
7. If you added a status or state, update the owning module's transition map and its unit test.
8. If the change affects an endpoint or a status enum exposed on the wire, update `api/openapi.yaml` (`python scripts/check-openapi-sync.py`).
9. Run the tests: `.\scripts\test-backend.ps1` (it applies pending migrations to the test database automatically).

## sqlc rules (learned the hard way, see `sqlc.yaml`)

- A table and an enum must not share a singular name (`approval_actions` vs enum `approval_action` collided; the enum became `approval_decision`).
- Use `sqlc.arg(name)` for parameters inside expressions (`COALESCE`, `NULLIF`), otherwise you get `Column2`.
- Nullable `uuid` columns are overridden to non-null `uuid.UUID`, so NEVER select a nullable uuid column directly; wrap with `COALESCE(col, '00000000-0000-0000-0000-000000000000'::uuid)` (see `ClaimStuckInstances`).
- Self-referencing UPDATE subqueries need table aliases.

## Recipe check (migration.md) and corrections

| Recipe claim | Finding |
|--------------|---------|
| Files named `NNNN_...` | Real files use six digits (`000013_...`) |
| Canonical schema is `docs architecture/00-backend-design-v2-base.html` and `04-feature-extensions.html` | These now live under `docs/history/design-v2/` as Markdown and are history, not truth. The truth is the migrations plus `docs/domain/*` and `docs/reference/*` |
| `make migrate-up && make migrate-down && make migrate-up` | `make migrate-down` runs `down -all`; `make migrate-verify` already wraps the three steps |
| "Update queries ... then `make sqlc`" | Correct. Note `internal/store/gen` is git-ignored, so it must be regenerated on every fresh checkout |

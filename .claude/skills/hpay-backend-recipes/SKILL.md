---
name: hpay-backend-recipes
description: Recipes (conventions + checklists) for backend building blocks in src/backend - new module, SQL migration, workflow step handler, endpoint integration tests, demo seed data. Use inside hpay-feature stage 4-5 when the task creates one of these artifacts, or when the user asks to add a module/migration/step handler/endpoint test/seed.
---

# hpay-backend-recipes

Pick the recipe, read ONLY that file, follow it. These are building blocks used by `hpay-feature` / `hpay-bugfix`; they do not replace the workflow.

| Need | Read |
|------|------|
| New domain module under `internal/` | [references/module.md](references/module.md) |
| Schema change (table/column/enum/index/constraint) | [references/migration.md](references/migration.md) |
| New workflow step handler | [references/workflow-step.md](references/workflow-step.md) |
| Integration tests for endpoints / edge IDs | [references/api-test.md](references/api-test.md) |
| Demo or e2e seed data | [references/seed-demo-data.md](references/seed-demo-data.md) |

Order when several apply: migration -> module -> workflow-step -> api-test. A schema change always goes first; run `sqlc generate` after queries change.
Fact checks: tests need `TEST_DATABASE_URL` (no testcontainers); migrations run with the external `migrate` CLI; `api/openapi.yaml` must pass `python scripts/check-openapi-sync.py`.
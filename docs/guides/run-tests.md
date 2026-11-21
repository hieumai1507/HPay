---
title: Run the tests
type: guide
status: verified-against-code
last_verified: 2026-11-21
---

# Run the tests

Sources: `scripts/test-backend.ps1`, `scripts/test-backend.sh`, `src/backend/Makefile`, `.github/workflows/backend-ci.yml`, `admin-ci.yml`, `src/admin/package.json`, `src/mobile/package.json`, `docs/handoff/baseline.md`.

## Backend

### Option A (default on this dev machine): Docker script

Windows Application Control blocks Go-built test executables, so use the script. Docker Desktop must be running.

```powershell
cd D:\hpay
.\scripts\test-backend.ps1                                        # go test ./... -count=1
.\scripts\test-backend.ps1 -GoArgs "./internal/ledger -race -count=1 -v"
```
The parameter is `-GoArgs` (the comment at the top of the script says `-Args`, which is wrong; the `param` block is the truth). Arguments are split on spaces.

What the script does:
1. `docker compose -p hpay up -d postgres redis` in `src/backend`.
2. Runs `migrate/migrate:v4.17.1 ... up` against `postgres:5432` on the `hpay_default` network. It prints `no change` on stderr when already migrated; PowerShell shows that as a `NativeCommandError` red block. That is cosmetic, tests still run.
3. Runs `golang:1.25` with the source mounted, `TEST_DATABASE_URL=postgres://HPay:HPay@postgres:5432/HPay?sslmode=disable` and `REDIS_ADDR=redis:6379`, then `go test <args>` via `scripts/test-backend.sh`.

Verified on 2026-11-21: `.\scripts\test-backend.ps1 -GoArgs "./internal/fee ./internal/ledger -count=1"` printed `ok ... internal/fee` and `ok ... internal/ledger`.

Important caveats:
- The script tests against the SAME database as local development (database `HPay`, compose project `hpay`), not an isolated test database. Tests create random users and rows, so the dev database accumulates test data (seen: workflow instances, outbox rows).
- The script sets `REDIS_ADDR` but not `TEST_REDIS_ADDR`. Two tests read `TEST_REDIS_ADDR` (`TestEdge_E05` in `internal/workflow/integration_test.go`, `TestEdge_E17` in `test/integration/money/redis_down_test.go`); without it their real-Redis leg is skipped with a log line while the test still passes. CI sets `TEST_REDIS_ADDR`. To run those legs, add `-e TEST_REDIS_ADDR=redis:6379` to the script's `docker run` (not changed here).
- The script does not pass `-p 1`. CI does (`go test -p 1 ./... -race -count=1`) because packages that pump the shared outbox must not run concurrently (`testutil.RunSerialized`). If you see events stolen between packages, rerun with `-GoArgs "./... -p 1 -count=1"`.

### Option B: raw `go test` (when Go executables are allowed, e.g. CI or Linux)

```bash
cd src/backend
docker compose up -d
migrate -path db/migrations -database "$DB_URL" up       # make migrate-up
make test                                                 # go test ./... -count=1  (unit tests; integration tests SKIP)
make test-integration                                     # same, with TEST_DATABASE_URL="$(DB_URL)"
go test -p 1 ./... -race -count=1                         # what CI runs
bash ./scripts/check-edge-tests.sh                        # CI gate: a TestEdge_<ID>_ function must exist for E01..E20 (E18 skipped)
python scripts/check-openapi-sync.py                      # router.go routes == api/openapi.yaml
go vet ./...
```
Rules:
- Integration tests need `TEST_DATABASE_URL` pointing at a migrated database. `testutil.PoolFromEnv` calls `t.Skip` when it is unset, so a green run without it proves little.
- There are no testcontainers; Postgres and Redis come from docker compose or CI services.
- `golangci-lint` is NOT run in CI (a TODO in `backend-ci.yml`); `make lint` calls it and needs it installed locally.
- Always add `-race` for concurrency-related changes.

Verified on 2026-11-21: `python scripts/check-openapi-sync.py` printed `openapi sync OK: 68 operations match`.

### CI jobs (backend-ci.yml)

`lint` (sqlc generate, `go mod tidy`, gofmt, vet, openapi sync), `sqlc`, `migrate` (up, down -all, up, ledger append-only trigger check), `test` (race tests with a separate `HPay_test` Postgres on 5433 and Redis on 6380, then edge-ID gate), `build`, `vuln` (govulncheck), `image-scan` (Trivy HIGH/CRITICAL).

## Admin (`src/admin`)

```powershell
cd src\admin
npm ci
npx tsc --noEmit        # or npm run typecheck
npx eslint .            # or npm run lint
npm run build           # typecheck + vite build
```
Verified 2026-11-21: `tsc --noEmit` exit 0. `eslint .` reports `12 problems (6 errors, 6 warnings)` (pre-existing, matches the baseline).

### Playwright end-to-end (`src/admin/e2e`, 10 spec files)

Requirements (mirrors `admin-ci.yml`):
1. Postgres and Redis running, migrations applied.
2. Seeded database WITH e2e admins: `go run ./cmd/seed --e2e-admins` in `src/backend`.
3. Backend running on `:8080` (`go run ./cmd/server`, or a built binary).
4. Admin built and served by `vite preview` on `:4173`:
   ```powershell
   cd src\admin
   npm run build
   npx playwright install chromium
   npx vite preview --port 4173
   ```
5. In another terminal: `npx playwright test --project=chromium` (or `npm run test:e2e`). Base URL defaults to `http://localhost:4173`, override with `E2E_BASE_URL`.

Notes: workers are capped (3 locally, 2 in CI) because admin login is rate limited to 5 requests per minute per IP. Reports go to `playwright-report/`. `e2e/.auth/` holds real JWTs and is git-ignored; delete it after runs. Baseline: NOT RUN in the 2026-11-21 baseline; the CI comment records 29 passed, 2 skipped, 0 failed on a fresh seed. Not executed in this verification (needs the full stack).

## Mobile (`src/mobile`)

```powershell
cd src\mobile
npm ci
npx tsc --noEmit        # npm run typecheck
npx jest                # npm test
npx eslint .            # npm run lint
```
Verified 2026-11-21: `npx jest` gave `Test Suites: 15 passed, 15 total`, `Tests: 80 passed, 80 total`. ESLint gives `11 problems (3 errors, 8 warnings)` (pre-existing).

## QA workbooks (manual and automated case catalog)

The five workbooks in `docs/qa/test-cases/*.xlsx` are generated from Python data by the skill `.claude/skills/hpay-qa-testcases`. Never run `generate_test_cases.py` directly (it resets every Status to "Not Run"); use the merge script, which preserves recorded results (columns M to R):

```powershell
python -X utf8 .claude/skills/hpay-qa-testcases/scripts/merge_results.py                       # in place
python -X utf8 .claude/skills/hpay-qa-testcases/scripts/merge_results.py --out .gotmp/qa-verify # dry run
```
Workflow: execute the case (named `TestEdge_*` test or Playwright spec, or manual steps), record Status (`Not Run`, `Pass`, `Fail`, `Blocked`, `N/A`), actual result and evidence, never mark Pass for something not run. On Fail, open a `hpay-bugfix` run and cite the test ID (`TC-<GROUP>-NNN`). Back up first: `Copy-Item docs/qa/test-cases .gotmp/qa-backup -Recurse`.

## Current baseline (docs/handoff/baseline.md, 2026-11-21, before any fix)

| Check | Result |
|-------|--------|
| `go build ./...`, `go vet ./...` | PASS |
| backend `go test ./... -race -count=1 -v` via script | PASS: 160 run, 160 pass, 0 fail, 0 skip |
| `npx tsc --noEmit` admin / mobile | PASS / PASS |
| mobile `npx jest` | PASS: 15 suites, 80 tests |
| admin `npx eslint .` | FAIL (pre-existing): 6 errors, 6 warnings |
| mobile `npx eslint .` | FAIL (pre-existing): 3 errors, 8 warnings |
| Playwright e2e | NOT RUN |

Do not count the lint errors as regressions, but do not add new ones.

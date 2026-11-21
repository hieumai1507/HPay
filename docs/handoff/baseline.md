# Baseline (2026-11-21, before any fix)

Windows Application Control blocks every Go-built `.exe` on the dev machine, so backend tests run in Docker:
`.\scripts\test-backend.ps1` (starts Postgres 16 + Redis 7, applies all migrations, runs `go test` in `golang:1.25`).
Arguments: `.\scripts\test-backend.ps1 -GoArgs "./internal/ledger -race -count=1 -v"`.

| Check | Result |
|-------|--------|
| `go build ./...`, `go vet ./...` (src/backend) | PASS |
| `go test ./... -race -count=1 -v` via script | PASS: 160 run, 160 pass, 0 fail, 0 skip (includes `test/integration` against real Postgres + Redis) |
| `npx tsc --noEmit` admin / mobile | PASS / PASS |
| `npx jest` (mobile) | PASS: 15 suites, 80 tests |
| `npx eslint .` admin | FAIL (pre-existing): 6 errors, 6 warnings |
| `npx eslint .` mobile | FAIL (pre-existing): 3 errors, 8 warnings |
| Playwright e2e (admin) | NOT RUN (needs running backend + seed) |

Notes
- Many backend packages have no tests (`transfer`, `reversal`, `recon`, `outbox`, `org`, `vault`, `product`, `gateway`, `config`). Money-path fixes there must add tests first.
- Lint errors are pre-existing; do not count them as regressions, but do not add new ones.
# Baseline (2026-11-21)

| Check | Result |
|-------|--------|
| `go build ./...` (src/backend) | PASS |
| `go vet ./...` | PASS |
| `go test ./...` | NOT RUN: Windows Application Control blocks `*.test.exe` (also with GOTMPDIR=D:\hpay\.gotmp). Only `fee` and `workflow/handlers` reported ok. Not a code failure. |
| Integration tests | NOT RUN: Docker daemon not running; TEST_DATABASE_URL unset (tests skip) |
| `npx tsc --noEmit` admin | PASS |
| `npx tsc --noEmit` mobile | PASS |
| jest (mobile), playwright (admin) | NOT RUN |

Blocker for Wave 1+: need a way to run Go tests (allow a directory in App Control, or run in WSL/Docker). Do not start money/auth fixes without it.
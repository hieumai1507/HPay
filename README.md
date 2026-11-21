# HPay

Fintech e-wallet: Go backend, React admin console, React Native (Expo) mobile app. One repository.

## Layout

```
hpay/
├── src/                  # ALL CODE
│   ├── backend/          # Go + Gin + PostgreSQL + Redis (module github.com/hieumai1507/hpay-backend)
│   ├── admin/            # React + Vite + TanStack (back office)
│   └── mobile/           # Expo / React Native (end-user app)
├── docs/                 # KNOWLEDGE BASE for people and agents (see docs/README.md)
├── scripts/              # Dev helpers (Docker-based backend test runner)
├── .claude/
│   ├── skills/           # Shared skills (single source for the whole repo)
│   └── agents/           # Subagents (implementer, verifier, reviewers)
├── .github/workflows/    # CI: backend-ci.yml, admin-ci.yml
├── CLAUDE.md, AGENTS.md  # Instructions for agents (GitNexus, workflow rules)
└── .gitignore, .gitattributes
```

## Conventions

- **Everything is written in English**: docs, code comments, commit messages, test cases.
- **Docs first, code second.** Design lives in `docs/`; change the docs before changing the design in code. Where docs and code disagree, the code is the fact (see `docs/audit/AUDIT-REPORT.md`).
- **Skills live only at the root** (`.claude/skills/`). Do not put skills under `src/*`.
- Each app keeps its own `CLAUDE.md` / `AGENTS.md` for app-specific rules.
- Never commit `.env`, `data/`, `e2e/.auth/` or `.exe` files. See `.gitignore`.

## Quick start

```bash
# Backend (needs Docker for Postgres + Redis)
cd src/backend && cp .env.example .env && docker compose up -d && make migrate-up && go run ./cmd/server

# Admin
cd src/admin && npm ci && npm run dev

# Mobile
cd src/mobile && npm ci && npx expo start --web
```

## Running backend tests

On machines where Windows Application Control blocks Go-built executables, run the tests in Docker:

```powershell
.\scripts\test-backend.ps1                                   # all packages
.\scripts\test-backend.ps1 -GoArgs "./internal/ledger -race -count=1 -v"
```

The script starts Postgres 16 and Redis 7, applies all migrations and runs `go test` inside `golang:1.25`.

## Documentation

Start at [`docs/README.md`](docs/README.md). Current fix plan: [`docs/planning/PLAN.md`](docs/planning/PLAN.md). Audit report: [`docs/audit/AUDIT-REPORT.md`](docs/audit/AUDIT-REPORT.md). Baseline test results: [`docs/handoff/baseline.md`](docs/handoff/baseline.md).
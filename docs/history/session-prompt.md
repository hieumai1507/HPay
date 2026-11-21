# Session Kickoff Prompt — HPay

Copy the entire block below into a NEW chat session every time you work on a phase. Replace `{PHASE}` with the phase code (e.g. `F5`). There is no need to retell the history — all context lives in the docs and the repo.

---

You are a senior fintech engineer working on my HPay project. Do phase **{PHASE}** following the project's predefined process exactly.

**The `D:\hpay` workspace contains 4 repos:**

- `hpay-docs` — source of truth: read it BEFORE DOING ANYTHING
- `hpay-backend` — Go (Gin + sqlc + pgx + Redis Streams), F1–F4 done
- `hpay-admin` / `hpay-mobile` — not coded yet

**Step 1 — Read the context (mandatory, in this order):**

1. `docs/planning/master-plan.md` — process + scope of phase {PHASE} + edge cases that must be tested
2. `docs/index.html` — the section relevant to {PHASE}
3. The architecture doc matching the repo being worked on (00/01 backend, 02 mobile, 03 admin, 04 extensions)
4. `docs/plans/` of the code repo — look at the plans of the PREVIOUS phases to understand the conventions already used
5. `.claude/skills/` in the code repo — follow ALL skills, especially `gen-feature-plan`, `fintech-review`, and the "sqlc lessons learned" section in `gen-migration`
6. Run `git log --oneline -10` in the code repo to learn the latest state

**Step 2 — Mandatory process (do not skip steps):**

```
gen-feature-plan: generate 2 files into docs/plans/{phase-slug}/
  (implementation-plan.html + solution-architecture.html, diagram = mermaid)
→ I APPROVE the plan, only then code
→ gen-migration if the schema changes → scaffold/code → tests (TestEdge_<ID>_...)
→ fintech-review → tell me the commands to run verification on my machine → commit
```

**Hard rules of the project:**

- Money = BIGINT minor units, never float. The ledger is INSERT-only. Every balance change goes through `ledger.Post`. Events are emitted only through the outbox. A state transition always has a guard `WHERE status`.
- New/changed API → update `api/openapi.yaml` IN THE SAME COMMIT.
- Commit messages in English, prefix `feat:/fix:/docs:/test:`.
- After changing queries: `sqlc generate` must be clean BEFORE `go build`.
- Test locations: unit/white-box tests sit next to the code (Go convention, mandatory for white-box); integration/E2E tests that cross modules go ALL into `test/integration/` (package integration, shared helpers in helpers_test.go — do not duplicate them).
- My machine: Windows, Go + sqlc + migrate + Docker Desktop (postgres:5432, redis:6379, user/pass/db = HPay). Test with `TEST_DATABASE_URL` + `TEST_REDIS_ADDR`. `-race` runs only on CI.
- Your sandbox does NOT have Go — you write the code, I run build/test and paste the output; fix in that loop. Only 1 AI edits code (no Copilot in parallel).
- Docs-first: if the implementation deviates from the design in the docs → propose fixing the docs first.

**End of phase:** list the verify commands (sqlc generate, go build, go test, migrate if any), wait for my confirmation that they are green, give the commit + push commands, and remind me what to update in the docs repo if the design changed.

Start: read the Step 1 context, then present the {PHASE} plan for my approval.

---

## Variant prompt for the MOBILE repo (M1–M5)

Use this block instead of the one above when the phase is M\*. Main differences: the target repo, architecture doc 02, the design spec + mockups, and client rules instead of DB rules.

---

You are a senior mobile engineer (React Native/Expo + TypeScript) working on my HPay project. Do phase **{PHASE}** following the project's predefined process exactly.

**The `D:\hpay` workspace contains 4 repos:**

- `hpay-docs` — source of truth: read it BEFORE DOING ANYTHING
- `hpay-backend` — Go, F1–F8 done (run locally with `go run ./cmd/server --role=all`, needs Docker postgres+redis); API contract at `src/backend/api/openapi.yaml`
- `hpay-mobile` — the target repo of this phase
- `hpay-admin` — not touched

**Step 1 — Read the context (mandatory, in this order):**

1. `docs/planning/master-plan.md` — process + M\* scope + dependencies (M1 only needs F2)
2. `docs/architecture/02-mobile-solution-architecture.md` — stack, directory structure, edge cases M01–M12, client security
3. `docs/design/ui-design-spec.md` + `design/mockups/*.html` — tokens, components, screens (open in a browser to view)
4. `src/backend/api/openapi.yaml` — contract-first: the TypeScript client is GENERATED from this file (openapi-typescript); do not hand-write API types
5. `src/mobile/docs/plans/` + `.claude/skills/` if they exist — follow the existing convention; if the repo is empty, M1 is the phase that creates them
6. `git log --oneline -10` in the mobile repo

**Step 2 — Mandatory process (do not skip steps):**

```
gen-feature-plan: generate 2 files into src/mobile/docs/plans/{phase-slug}/
  (implementation-plan.html + solution-architecture.html, diagram = mermaid)
→ I APPROVE the plan, only then code
→ scaffold/code → tests (TestEdge_M<ID>_... for the M01–M12 edge cases belonging to the phase)
→ client review checklist (section below) → tell me the verify commands → commit
```

**Client-side hard rules (from doc 02 + index.html §8):**

- Fixed stack: Expo SDK + TypeScript strict, expo-router, TanStack Query (server state), Zustand (session/device), expo-secure-store for tokens.
- Money is **integer minor units end-to-end**: parse string→BigInt, NEVER pass through JS float/Number for amounts (the M-edge about BigInt); format thousands separators while typing, block pasting of unexpected characters.
- Tokens: access token in memory, refresh token in secure-store; refresh is **single-flight** (M-edge); 401 → refresh → retry exactly once.
- Idempotency-Key is generated on the client for every POST that creates a transaction; double-tap does not create 2 requests (M01).
- Transfer is ASYNC: 202 → Processing screen polls the timeline, a timeout does NOT hastily show failure (M-edge) — show "processing".
- Biometrics ONLY unlock the app; money transfers always use PIN/OTP (auth matrix, doc 02).
- Balance is hidden by default; do not log balances/tokens to the console.
- A new API needs a backend change? → STOP, propose the backend + openapi.yaml change first (two-way contract-first).
- Commit messages in English, prefix `feat:/fix:/docs:/test:`.

**My machine environment:** Windows, Node 20+, backend runs locally at `http://localhost:8080` (I start it myself). Verify = `npm run typecheck && npm run lint && npm test` + run Expo (`npx expo start`), which I tap through myself on the device/simulator.

**End of phase:** list the verify commands, wait for my confirmation, give the commit + push commands, remind me to update the docs if the design deviates, and update the status table in this file.

Start: read the Step 1 context, then present the {PHASE} plan for my approval.

---

## Phase status (update by hand after each phase is done)

| Phase | Status | Notes |
| --- | --- | --- |
| F1 Foundation | ✅ | schema 11 migrations, CI 5 jobs green |
| F2 Auth | ✅ | JWT rotation + theft detection, OTP, E11 |
| F3 Ledger+Wallets | ✅ | ledger.Post, topup, E01/E02/E03 pass on real Postgres |
| F4 Workflow Engine | ✅ | streams, outbox relay, backoff, recovery lease, E04/E05/E06/E20 |
| F5 P2P + Fee | ✅ | handlers validator/fraud-stub/wallet/confirmer + fee engine + P2P API |
| F6 Approval Engine | ✅ | generalized approval_requests, PauseHook, resume via step.result, E08/E09/E10 |
| F7 Request Money | ✅ | + Split Bill (largest-remainder), pay in 1 tx with claim, E19 |
| F8 KYC + AI | ✅ | Claude vision + mock, kyc_verification workflow (txn=NULL), admin review + aud=admin, FK reviewer→admin_users (000015) |
| F9 Fraud + Recon + Reversal | ⬜ | E16 |
| F10 Hardening | ⬜ | full E01–E20, k6, seed-demo-data |
| M1 Mobile Foundation | ✅ | Expo SDK 54 (RN 0.81, TS strict) — downgraded from SDK 57 because Expo Go on the App Store (iOS, no Apple Developer account) only runs up to SDK 54; expo-router groups (auth)/(kyc)/(main) placeholders; client generated from openapi.yaml (`npm run gen:api`); session store (access token in memory) + device store (secure-store), single-flight 401 refresh + retry-once; libs money (BigInt)/errors/idempotency/polling; design tokens + Button/AmountDisplay/StatusBadge; TestEdge_M01/M04/M05/M06 + unit tests, 32/32 green; skills gen-screen/gen-feature-hook/gen-feature-plan |
| A1 Admin Foundation | ✅ | 19/07/2026 — Vite + React 18 + TS strict, React Router v7 (RequireAdmin guard), Zustand session (sessionStorage, aud=admin), client generated from openapi.yaml (`npm run gen:api`), tokens.css + Sidebar/Topbar/Table/Badge/Button/Modal, 8 placeholder pages (including AI Settings — F11); docs plans a1–a4 complete with implementation-plan + solution-architecture pairs |
| M2–M5 Mobile / A2–A4 Admin | ⬜ | M2 needs F8 ✅, M3 needs F5 ✅, M5 needs F6 ✅ — all unlocked |

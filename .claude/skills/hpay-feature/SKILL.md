---
name: hpay-feature
description: End-to-end workflow for ANY new feature or change in HPay (backend/admin/mobile), from a vague request to verified code. Use when the user asks to implement, build, add or change behavior, or starts a phase/wave. Runs 7 gated stages - clarify, spec, plan, red tests, implement, verify, review - with a fixed effort level per stage. Not for bug reports (use hpay-bugfix) or pure review (use hpay-review).
argument-hint: "<feature description or ticket>"
---

# hpay-feature - one workflow, seven stages

Each stage has an **entry condition**, an **output file**, a **gate** that must pass before the next stage, and a recommended **effort**. Never skip a gate. A stage can be skipped only when its "Skip if" holds, and the skip must be written in the stage's output file.

Slug = `<phase-or-area>-<short-name>`, e.g. `w2-ledger-idempotent-post`. All outputs are Markdown under `docs/`.

| # | Stage | Output | Gate (all must hold) | Effort | Who runs it |
|---|-------|--------|----------------------|--------|-------------|
| 1 | Clarify | `docs/specs/<slug>/clarify.md` | Open questions answered or recorded as assumptions the user accepted | high | main session (needs the user) |
| 2 | Spec | `docs/specs/<slug>/spec.md` | Goal, non-goals, acceptance criteria are testable; invariants listed; user approved | high | main |
| 3 | Plan | `docs/plans/<app>/<slug>/plan.md` | Tasks ordered, each <= 1 commit, each maps to a test; file ownership per task; user approved | high | main |
| 4 | Red tests | test files only | New tests exist AND fail for the right reason (output pasted) | medium | implementer agent |
| 5 | Implement | code | Failing tests now pass; no unrelated test breaks | medium (high for money/auth) | implementer agent, one per task |
| 6 | Verify | `docs/handoff/<slug>-verify.md` | Fresh output of every verify command (see below), zero failures | low | verifier agent |
| 7 | Review | review report | No unresolved blocker/high from `hpay-review` | high | reviewer agents (parallel) |

## Stage details

### 1. Clarify (skip if: request already names who/why/success/constraint)
Invoke skill `interview-me`: one question at a time, each with your guess attached, until ~95% confidence. Do NOT run this in a background agent; it needs the user. Write answers and remaining assumptions to `clarify.md`.

### 2. Spec
Read first: `docs/README.md`, the architecture doc for the app, `docs/audit/AUDIT-REPORT.md` entries touching the same module, and the app `CLAUDE.md`.
Write `spec.md` with: Problem, Goals, Non-goals, Acceptance criteria (Given/When/Then, each with an ID `AC-n`), Invariants touched (money = integer minor units, ledger append-only, idempotency scope per user, lock order, state machine via owning function, outbox only), API/schema delta, Edge-case IDs (E/M/A) covered.
If the spec contradicts `docs/architecture/*`: STOP and propose the docs change first (docs-first rule).
**Docs are only trustworthy where `docs/audit/AUDIT-REPORT.md` says so. When docs and code disagree, code is the fact; record the mismatch in the spec.**

### 3. Plan
Write `plan.md`: task table (id, files, depends-on, size S/M/L, test that proves it, effort), schema changes (use the recipe `hpay-backend-recipes` -> `references/migration.md`), API changes (`src/backend/api/openapi.yaml` must stay in sync: `python scripts/check-openapi-sync.py`), rollout/rollback, risk.
Mark tasks that touch **money, auth, or migrations** as `HIGH-RISK`: they get effort `high`, a `doubt-driven-development` pass on the plan, and a second reviewer.
Parallelism: tasks may run in parallel only if their file sets are disjoint (list owners). Otherwise sequential.

### 4-5. Red then green (per task)
Follow `test-driven-development`. Use the right recipe:
- backend: `hpay-backend-recipes` (module, migration, workflow-step, api-test, seed)
- admin: `hpay-admin-recipes` (page, hook)
- mobile: `hpay-mobile-recipes` (screen, hook)
One implementer agent per task, fresh context, worktree isolation when tasks run in parallel. Commit per task. Before editing any symbol run GitNexus `impact`; before committing run `detect_changes` (see root CLAUDE.md).
Fix loop on failure: use `systematic-debugging` (no fix before root cause). **Max 3 fix attempts per failing test; on the 4th, stop and report to the user with what was tried.**

### 6. Verify (use `verification-before-completion`: no claim without fresh output)
Run what applies, paste results:
- backend: `cd src/backend && go build ./... && go vet ./... && go test ./... -race -count=1` and, with a DB, `TEST_DATABASE_URL=... go test ./test/... -count=1`; `python scripts/check-openapi-sync.py`
- admin: `cd src/admin && npx tsc --noEmit && npm run lint`; e2e when UI changed: `npm run test:e2e`
- mobile: `cd src/mobile && npx tsc --noEmit && npm run lint && npm test`
- docs: links resolve; no stale paths.
If a command cannot run in this environment (blocked binary, no Docker), say so explicitly. Never report "passing" for something not run.

### 7. Review
Run skill `hpay-review`. Blockers and highs go back to stage 5 (counts against the 3-attempt cap). Then update spec/plan if reality diverged, in the same change.

## Stop conditions (ask the user)
Irreversible or destructive action; secret/credential handling; anything outside this repo (push, publish, deploy); a business decision not in the spec (limits, fees, reversal policy); 3 failed fix attempts.

## Definition of done
All AC IDs have a passing test; stage 6 evidence attached; review clean; docs updated; no secrets in the diff.

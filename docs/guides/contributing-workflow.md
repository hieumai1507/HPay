---
title: Contributing workflow for humans and agents
type: guide
status: verified-against-code
last_verified: 2026-11-21
---

# Contributing workflow for humans and agents

Project instructions live in [`CLAUDE.md`](../../CLAUDE.md) (root, GitNexus code-intelligence rules) and per app in `src/backend/CLAUDE.md`, `src/admin/CLAUDE.md`, `src/mobile/CLAUDE.md`. Read the root file and the app file before touching code.

## Rules in one list

1. Docs first, code second. The knowledge base is `docs/` (index: `docs/README.md`). Pages with `status: verified-against-code` are trusted; `docs/history/` is history, not truth. When docs and code disagree, code is the fact: record the mismatch and fix the doc.
2. Everything in the repository is written in English (ADR 0002).
3. Every change goes through a skill workflow (below), not ad-hoc edits.
4. Before editing a symbol run GitNexus `impact` on it and report the blast radius; before committing run `detect_changes()` (CLAUDE.md). Warn about HIGH or CRITICAL risk before proceeding. Rename with the `rename` tool, never find-and-replace.
5. Money, ledger, auth and migrations are HIGH-RISK: tests first, second reviewer, effort high.
6. Never commit secrets (see below).

## Skills (in `.claude/skills/`)

| Skill | Use when | What it does |
|-------|----------|--------------|
| `hpay-feature` | any new feature or behavior change (backend, admin, mobile) | Gated stages with an output file each under `docs/specs/<slug>/`: clarify, spec, plan, test cases, red tests, implement, verify and fix loop, review, update knowledge base. Never skip a gate |
| `hpay-bugfix` | a bug, failing test, or an audit finding | Search the bug ledger (`docs/bugs/INDEX.md`) first, reproduce, find the root cause, write a regression test (red), fix, verify, review, record the bug and fix in `docs/bugs` |
| `hpay-review` | before a commit or PR | Fintech invariant checklist plus three independent parallel reviewers (code, security, tests), merged into one verdict |
| `hpay-backend-recipes`, `hpay-admin-recipes`, `hpay-mobile-recipes` | inside the implement stage | Checklists for module, migration, workflow step, endpoint tests, seed, admin page, mobile screen. Treat them as guidance and verify against code: see the correction tables in the guides (`add-a-migration`, `add-an-endpoint`, `add-a-workflow-step`, `add-a-backend-module`) |
| `hpay-qa-testcases` | adding or recording QA cases | Catalog in Python, merged into the `docs/qa/test-cases/*.xlsx` workbooks without losing recorded results |
| `fintech-review` | final step of any money-related change | Invariant checklist |
| `test-driven-development`, `systematic-debugging`, `verification-before-completion`, `interview-me` | supporting | Red-green-refactor, root-cause first, evidence before success claims, clarifying questions |

Choose: new behavior -> `hpay-feature`; something broken -> `hpay-bugfix`; just reviewing -> `hpay-review`.

## Agents (in `.claude/agents/`)

| Agent | Role | Tools |
|-------|------|-------|
| `implementer` | implements ONE task from an approved plan with red-green TDD, in an isolated worktree | read/write/edit, PowerShell |
| `verifier` | runs the verification commands and reports raw evidence; never fixes | read-only plus PowerShell |
| `code-reviewer` | correctness, races, state-machine discipline, architecture fit | read-only |
| `security-auditor` | authn/authz, IDOR, idempotency scope, secrets and PII, SSRF, uploads, rate limiting | read-only |
| `test-engineer` | maps acceptance criteria to tests, finds missing failure, concurrency and replay cases | read-only |

Rules for agents: one task per implementer, fresh context, file sets disjoint when parallel; reviewers and verifiers never edit; the main session owns anything needing the user (clarify, approvals).

## Docs-first flow for a typical change

1. Read `docs/domain/<context>.md`, `docs/audit/AUDIT-REPORT.md` entries for the module, `docs/bugs/INDEX.md`.
2. Write or update the spec and plan under `docs/specs/<slug>/` and get approval.
3. Write failing tests, implement, run [the tests](run-tests.md).
4. Review, then update the domain and reference docs, write an ADR for any real decision (`docs/adr/`, accepted ADRs are immutable: supersede with a new one).

## Verification before saying "done"

Run the commands and paste the evidence. For the backend use `.\scripts\test-backend.ps1` (Docker) on this machine. Compare against the baseline in `docs/handoff/baseline.md`; do not add new lint errors.

## Commit conventions

Observed in the history: conventional-commit style prefixes, lower-case, imperative, e.g. `chore: docker-based backend test runner and recorded baseline`, `chore: init hpay monorepo (src/ code, docs/ knowledge-base, root skills)`. Use `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `refactor:`. One logical change per commit, tasks sized for one commit each (plan stage). Never skip hooks or signing. Do not commit unless the human asked for it.

Git history was reset on 2026-11-21 (ADR 0001); old history is not available.

## Never commit secrets or PII

- `.env` and `.env.*` are git-ignored (only `.env.example` is tracked); also `*.pem`, `*.key`, `src/backend/data/` (KYC images are PII), `**/e2e/.auth/` (real JWTs), `*.exe`, `bin/`, `tmp/`, `.gotmp/`, `playwright-report/`.
- The audit found real credentials in an old `.env` (Neon DB password, Cloudinary secret, JWT secret appeared in audit output) and recommends rotating them. Treat any such value as compromised.
- Do not paste `.env` contents or tokens into docs, issues, prompts or logs. Demo-only passwords (`Demo@123`, `E2ETest@123`) are documented fixtures; never reuse them anywhere real.
- Before `git add`, check `git status` for `.env*`, `data/`, `sessions.json`, `.exe`.

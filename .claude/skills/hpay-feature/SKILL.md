---
name: hpay-feature
description: End-to-end workflow for ANY new feature or change in HPay (backend/admin/mobile), from a vague request to verified code. Use when the user asks to implement, build, add or change behavior, or starts a phase/wave. Runs 9 gated stages - clarify, spec (with architecture impact), plan (with constitution check and design gate), design test cases, red tests, implement, verify and fix loop, review, update knowledge base and architecture - with a fixed effort level per stage. Not for bug reports (use hpay-bugfix) or pure review (use hpay-review).
argument-hint: "<feature description or ticket>"
---

# hpay-feature - one workflow, nine stages

Each stage has an **entry condition**, an **output file**, a **gate** that must pass before the next stage, and a recommended **effort**. Never skip a gate. A stage can be skipped only when its "Skip if" holds, and the skip must be written in the stage's output file.

Slug = `<phase-or-area>-<short-name>`, e.g. `w2-ledger-idempotent-post`. All outputs are Markdown under `docs/`.

| # | Stage | Output | Gate (all must hold) | Effort | Who runs it |
|---|-------|--------|----------------------|--------|-------------|
| 1 | Clarify | `docs/specs/<slug>/clarify.md` | Open questions answered or recorded as assumptions the user accepted | high | main session (needs the user) |
| 2 | Spec | `docs/specs/<slug>/spec.md` | Goal, non-goals, acceptance criteria are testable; invariants listed; **Architecture impact classified (none / amend / new) with the affected pages named**; user approved | high | main |
| 3 | Plan + design gate | `docs/specs/<slug>/plan.md` (tasks, owners, tests; history of past plans is in `docs/history/plans/`) | Tasks ordered, each <= 1 commit, each maps to a test; file ownership per task; **constitution check filled in; for amend/new architecture impact an ADR with status `proposed` exists (options, decision, consequences) and the user approved it BEFORE any code**; user approved the plan | high | main |
| 4 | Test cases | `docs/specs/<slug>/test-cases.md` + `test-cases.json` + the feature workbook `docs/qa/test-cases/<slug>-testcase.xlsx` | Every AC-n has >= 1 case; each case has ID, type (happy / negative / edge / boundary / security), priority, steps, expected result, executor (AUTO-API / AUTO-WEB / HUMAN) and the test name that will automate it; edge IDs (E/M/A) mapped; a workbook of its own rendered by skill `hpay-qa-testcases` from the JSON (the five catalog workbooks and `generate_test_cases.py` are not touched); user approved the list | high | main |
| 5 | Red tests | test files only | Automated cases exist as tests AND fail for the right reason (output pasted); HUMAN cases have written steps | medium | implementer agent |
| 6 | Implement | code | Failing tests now pass; no unrelated test breaks | medium (high for money/auth) | implementer agent, one per task |
| 7 | Verify + fix loop | `docs/handoff/<slug>-verify.md` + results recorded in the workbook | Run EVERY case of the feature: automated via commands, HUMAN ones executed or marked Blocked with a reason. Record Status/Actual Result/Evidence/Executed By/Date per case. Any Fail goes through the fix loop below until all Pass or the cap is hit; no case left "Not Run" without a reason | low (fix loop: medium/high) | verifier agent (run) + implementer agent (fix) |
| 8 | Review | review report | No unresolved blocker/high from `hpay-review` | high | reviewer agents (parallel) |
| 9 | Update knowledge base and architecture | edits under `docs/` (architecture pages, ADR -> accepted) | Domain/reference/product/architecture docs describe the feature as shipped; **for a UI change the user guide is updated (skill `hpay-user-guide`) or the spec says "guide: not affected" with the reason**; every proposed ADR is now `accepted` (or `rejected`/`superseded` with the reason); spec marked done; test-case results consistent with the workbook | low | main |

## Stage details

### 1. Clarify (skip if: request already names who/why/success/constraint)
Invoke skill `interview-me`: one question at a time, each with your guess attached, until ~95% confidence. Do NOT run this in a background agent; it needs the user. Write answers and remaining assumptions to `clarify.md`.


### 2. Spec
Read first: `docs/README.md`, `docs/constitution.md`, `docs/domain/<context>.md` for every context touched (current rules and known gaps), `docs/architecture/` for the app, `docs/bugs/INDEX.md` for earlier bugs in the same area (do not reintroduce them), `docs/audit/AUDIT-REPORT.md` entries for the same module, and the app `CLAUDE.md`.
Write `spec.md` with: Problem, Goals, Non-goals, Acceptance criteria (Given/When/Then, each with an ID `AC-n`), Invariants touched (use the C-xx ids from `docs/constitution.md`), API/schema delta, Edge-case IDs (E/M/A) covered, an **Architecture impact** section, and a **User guide impact** line.

**Architecture impact** (mandatory, one of three levels; when unsure choose the higher one):
- `none`: the change fits inside existing building blocks, runtime flows and cross-cutting concepts. State which `architecture/` pages you read to conclude this.
- `amend`: an existing architecture page changes (a block gains a responsibility or dependency, a runtime flow gets a step, a cross-cutting concept changes). Name every page, and every `domain/` page, that will change.
- `new`: a new component, process, external system, data store, runtime flow, or cross-cutting concept appears. Name the new page(s) to create from `docs/_templates/architecture-doc.md` and the pages that must link to it.
Triggers that force at least `amend`: a new package under `internal/`, a new table owned by a different context, a new background job or consumer, a new external call, a new endpoint family, a change to auth, idempotency, locking, outbox or error handling, a new dependency in `go.mod` / `package.json`. Anything that touches a principle in `constitution.md` also needs an ADR.
If the spec contradicts `docs/domain/*` or `docs/architecture/*`: STOP and propose the docs change first (docs-first rule).
**Pages with `status: verified-against-code` are trusted; anything under `docs/history/` is history, not truth. When docs and code disagree, code is the fact: record the mismatch in the spec and fix the doc in stage 9.**

**User guide impact** (mandatory one line): `updated` when a person sees or does something different in the admin or mobile UI (a page, tab, label, button, flow, role or permission, a message they can read), naming the chapters; `not affected` with the reason otherwise (backend only, invisible change). A UI change with `not affected` is rejected in review.

### 3. Plan
Write `plan.md`: task table (id, files, depends-on, size S/M/L, test that proves it, effort), schema changes (recipe `hpay-backend-recipes` -> `references/migration.md`), API changes (`src/backend/api/openapi.yaml` must stay in sync: `python scripts/check-openapi-sync.py`), rollout/rollback, risk.
For a change with User guide impact `updated`, add a last task "update the user guide" (skill `hpay-user-guide`: chapter edits, retaken screenshots, HTML copy, `check-guide.mjs`); it depends on the implementation tasks because the screenshots need the finished UI.: effort `high`, a `doubt-driven-development` pass on the plan, a second reviewer.

**Constitution check (mandatory).** Add the table from the end of `docs/constitution.md` to the plan and answer it for every principle the change touches. A plan that touches money, auth, workflow or migrations must answer C-01 to C-08 and C-13 explicitly. A deviation needs a justification and an ADR; an unjustified deviation blocks the plan.

**Design check (mandatory).** Add the table from section 7 of `docs/architecture/design-principles.md` to the plan (SOLID ownership, new interfaces, dependency direction, new patterns, rule of three). A new interface or pattern without a concrete second use or test fake is a plan defect.

**Design gate (when Architecture impact is `amend` or `new`, or a constitution principle is touched).**
1. Write `docs/adr/NNNN-<slug>.md` from `docs/_templates/adr.md` with status `proposed`: context, at least two real options, the decision, constitution check, consequences, documentation to update. Take the next free number from `docs/adr/README.md`; add the row there with status `proposed`.
2. For `new`, also draft the architecture page skeleton (purpose, context diagram, building blocks) in the spec folder so the design can be reviewed as a picture, not only as prose.
3. If the decision reverses an earlier accepted ADR, the new ADR lists it under `supersedes`.
4. **Stop and get the user's approval of the ADR before writing any test or code.** Implementation starts from the approved decision; if implementation proves the decision wrong, stop, update the ADR (still `proposed`) and ask again rather than diverging silently.
Parallelism: tasks may run in parallel only if their file sets are disjoint (list owners). Otherwise sequential.

### 4. Test cases (before any code)
Design the test cases from the acceptance criteria, using skill `hpay-qa-testcases`.
1. For each `AC-n` write at least one case; add negative, boundary, edge (E/M/A IDs), security/authz, and for money/workflow features also concurrency, replay (double submit, redelivery) and failure/compensation cases.
2. Each case: ID (`TC-<GROUP>-NNN`), feature, role, type, priority (P0/P1/P2), preconditions, steps, test data, expected result, executor (AUTO-API / AUTO-WEB / HUMAN), and for automated ones the test function name that will implement it (`TestEdge_*`, Playwright spec, jest test).
3. Write them to `docs/specs/<slug>/test-cases.md` (table, traceable `AC-n` -> `TC-` ids) and to `docs/specs/<slug>/test-cases.json` (the data the workbook is rendered from; its shape is in the docstring of `render_feature_workbook.py`). Render the feature workbook with `python -X utf8 .claude/skills/hpay-qa-testcases/scripts/render_feature_workbook.py <slug>`, which writes `docs/qa/test-cases/<slug>-testcase.xlsx`. Do NOT add the cases to `generate_test_cases.py` or to the five catalog workbooks: the generator only supplies the sheet format, the cases come from the feature.
4. The user approves the case list before stage 5. A case with no executable steps or no expected result is rejected.

### 5-6. Red then green (per task)
Follow `test-driven-development`. Stage 5 turns every AUTO case into a failing test (paste the failing output); HUMAN cases keep written steps. Stage 6 makes them pass. Use the right recipe:
- backend: `hpay-backend-recipes` (module, migration, workflow-step, api-test, seed)
- admin: `hpay-admin-recipes` (page, hook)
- mobile: `hpay-mobile-recipes` (screen, hook)
One implementer agent per task, fresh context, worktree isolation when tasks run in parallel. Commit per task. Before editing any symbol run GitNexus `impact`; before committing run `detect_changes` (see root CLAUDE.md).

### 7. Verify and fix loop (use `verification-before-completion`: no claim without fresh output)
Run **every case of this feature**, not just "the test suite":
1. Automated: run the commands, paste results.
   - backend: `.\scripts\test-backend.ps1` (Docker runner; add `-GoArgs "./internal/<pkg> -race -count=1 -v"` to target) plus `cd src/backend && go build ./... && go vet ./... && python scripts/check-openapi-sync.py`
   - admin: `cd src/admin && npx tsc --noEmit && npm run lint`; UI changes also `npm run test:e2e`
   - mobile: `cd src/mobile && npx tsc --noEmit && npm run lint && npm test`
2. HUMAN cases: execute the steps, or mark `Blocked` with the reason. Never mark Pass for something not executed.
3. Record each case in the workbook (Status, Actual Result, Evidence, Executed By, Last Run Date) and summarize in `docs/handoff/<slug>-verify.md` (counts of Pass / Fail / Blocked / Not Run).
4. **Fix loop:** for every `Fail`, use `systematic-debugging` (no fix before root cause), fix, re-run that case AND the cases of the same area, update the workbook. Repeat until all cases are Pass or Blocked with a reason. **Max 3 fix attempts per failing case; on the 4th, stop and report what was tried.** A failure that exposes a pre-existing bug is recorded as a bug per `hpay-bugfix` stage 7.
5. If a command cannot run in this environment (blocked binary, no Docker), say so explicitly. Never report "passing" for something not run.
6. **Performance gate.** If the change touches the money path or its locking (ledger, transfer, offerguard, per-user locks), the workflow engine (consumer, relay, outbox, streams), or the cost of a hot path (auth hashing, a new query on confirm/request), run the load test before and after the change on a fresh database (on this Windows machine: `$env:LOAD_USERS=200; $env:LOAD_WORKERS=50; $env:LOAD_SECONDS=60; .\scripts\test-backend.ps1 -Db load<n> -GoArgs "-tags loadtest ./test/loadtest -run TestP2PLoad -v -count=1 -timeout 15m"`, a new `-Db` name per run so the database is fresh; elsewhere `make loadtest`) (see `src/backend/scripts/loadtest/README.md`; it also checks ledger integrity). Record both runs in `docs/quality/performance/<date>-<slug>.md` (throughput, p95, errors, integrity result) and note that numbers are noisy on a laptop. Ledger integrity failing is a Fail; a clear drop in throughput or rise in p95 needs an explanation or a fix. Otherwise write "performance: not affected" in the verify note. The load test is not in CI (build tag `loadtest`, noisy), so this step is the only place it runs.

### 8. Review
Run skill `hpay-review`. Blockers and highs go back to stage 6 and then re-run stage 7 for the affected cases (counts against the 3-attempt cap). Then update spec/plan if reality diverged, in the same change.

### 9. Update the knowledge base (required, part of done)
The knowledge base (`docs/`) describes the system as it is, so a feature is not done until it is there. In the same change:
- `docs/domain/<context>.md`: add or change business rules (new `BR-` IDs with Enforced-by and Test columns; the Test column now names the cases from stage 4), state machines, flows, invariants, edge cases; set `last_verified`; remove a "Known gaps" entry only when the fix is shipped and tested. New bounded context: copy `docs/_templates/domain-doc.md`.
- `docs/reference/`: `data-model.md` (new tables/columns/constraints), `api.md` (must match `openapi.yaml`), `error-codes.md`, `configuration.md`, `glossary.md`.
- **User guide** (when the spec said `updated`): run skill `hpay-user-guide` for the admin guide `docs/guides/admin-user-guide.md` (and the mobile guide once it exists): edit the chapters, retake the screenshots that show a changed screen and look at them, add a line to "What changed in this version", regenerate the HTML copy, run `node docs/guides/admin/check-guide.mjs` and paste its output. Problems you notice in the product while doing this go to `docs/guides/admin/OPEN-ISSUES.md` and to the user., and `roles-and-permissions.md` / `kyc-tiers-and-limits.md` if they changed.
- `docs/architecture/` (follow the Architecture impact decided in stage 2, and correct it if reality differs):
  - `amend`: edit the existing page in place; update its building blocks table, mermaid diagrams, code map and risks; bump `last_verified`; keep its front matter; add the ADR to its `adr:` list. Do not leave a changed flow described the old way.
  - `new`: create the page from `docs/_templates/architecture-doc.md`, fill every section from the code as built (not from the plan), and link it from `architecture/README.md` and from every page that depends on it. A new component also gets its glossary terms in `reference/glossary.md` and its endpoints, tables and config in `reference/`.
  - Reversing an earlier decision: the old ADR gets status `superseded` and `superseded_by`; its body is not edited; the architecture pages describe only the new reality.
- `docs/adr/`: move each `proposed` ADR from this change to `accepted` (or `rejected` with the reason) and update `docs/adr/README.md`; add an ADR for any decision made during implementation that was not in the plan. Accepted ADRs are immutable: supersede instead of editing. If `constitution.md` itself should change, that is a separate ADR and needs the user's approval.
- `docs/specs/<slug>/spec.md`: set status to done and link the commit. Keep it as history; the domain docs are the current truth.
- Anything you learned that was wrong in the KB: fix it now, do not leave a note.
This stage also runs when a feature is changed (not only created). If a doc cannot be updated (unknown behavior), record it under that doc's "Open questions".

## Stop conditions (ask the user)
Irreversible or destructive action; secret/credential handling; anything outside this repo (push, publish, deploy); a business decision not in the spec (limits, fees, reversal policy); **an architecture decision awaiting approval (stage 3 design gate)**; 3 failed fix attempts.

## Definition of done
Every `AC-n` has at least one case; every case is Pass or Blocked-with-reason and recorded in the workbook; stage 7 evidence attached; review clean; architecture and knowledge base updated and no ADR left `proposed` (stage 9); the user guide updated and its checker passing, or "guide: not affected" with the reason (UI changes only); no secrets in the diff. If the feature also fixed a bug, follow `hpay-bugfix` stage 7 for that bug.
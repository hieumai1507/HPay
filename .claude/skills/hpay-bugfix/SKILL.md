---
name: hpay-bugfix
description: Workflow for fixing a bug, failing test, audit finding, or unexpected behavior in HPay. Searches the bug ledger (docs/bugs) for known or related bugs first, then reproduces, finds the root cause, adds a regression test, fixes, verifies, reviews, and records the bug and the fix in docs/bugs so the next run can reuse it. Use when something is broken, a test fails, or an item from docs/audit/AUDIT-REPORT.md must be fixed. Not for new features (use hpay-feature).
argument-hint: "<bug description, failing test, or audit finding id>"
---

# hpay-bugfix - recall, reproduce, root-cause, fix, prove, record

| # | Stage | Output | Gate | Effort |
|---|-------|--------|------|--------|
| 0 | Recall | short note in the working summary | Ledger searched; known/related bugs listed or "none found" stated | low |
| 1 | Reproduce | failing test or exact repro commands | Bug observed with fresh output. Audit findings are claims until reproduced; one that cannot be reproduced is recorded as `cannot-reproduce`, not "fixed" | medium |
| 2 | Root cause | `docs/handoff/<slug>-rootcause.md` | Cause named with file:line; every other call site with the same flaw listed | high |
| 3 | Regression test + test case | a test that fails now, for that cause, plus a case (`TC-` id, steps, expected, executor) added to the QA workbook via skill `hpay-qa-testcases` and linked in the bug record | Fails for the stated reason; the case exists and maps to the test | medium |
| 4 | Fix | minimal change | Regression test passes; related tests still pass | medium (high for money/auth) |
| 5 | Verify + fix loop | evidence + workbook results | Run the regression case AND every existing case of the same area (workbook rows with the same feature or edge ID), same commands as `hpay-feature` stage 7, fresh output; record Status/Evidence/Date in the workbook; any new Fail goes back to stage 2 | low |
| 6 | Review | `hpay-review` | Clean | high |
| 7 | Record | `docs/bugs/BUG-NNNN-<slug>.md` + `docs/bugs/INDEX.md` row + docs updates | Record complete per template; domain/reference docs corrected; same change as the fix | low |

## Stage 0 - Recall (never skip)
Search before debugging. Derive 3-6 keywords: the error text or ERR_* code, the package/area, the symptom class (race, replay, auth...), and any audit ID.
1. `docs/bugs/INDEX.md` (tags, areas, titles) and grep `docs/bugs/*.md` for the keywords (the "How to recognize this again" sections are written for this).
2. `docs/audit/AUDIT-REPORT.md` for the same area/ID.
3. `docs/domain/<area>.md` section "Known gaps".
Then state one of: **exact match** (an earlier record describes this; read its root cause and fix; check whether the fix regressed or was incomplete; reuse its regression test and link it), **related** (same area/tag/cause; read it and its "Other places with the same flaw" list before forming a hypothesis), or **none found**. A match is a hypothesis to test, never a conclusion: still reproduce and prove the cause.

## Rules
- Use `systematic-debugging` for stages 1-2: no fix without root cause. Symptom patches are rejected in review.
- Use `test-driven-development` for 3-4. Test cases are part of the fix, not optional: stage 3 adds the case, stage 5 records its result.
- Money/auth/migration bugs: add a second regression test for concurrency or replay (double submit, redelivery, parallel requests), run with `-race`.
- Before editing a symbol: GitNexus `impact` and report blast radius; before commit: `detect_changes`.
- **3 failed fix attempts: stop and report** what was tried and learned. Still write the record (status `open`) with what was ruled out; that is valuable for the next attempt.
- If the bug is really a missing business rule, switch to `hpay-feature` stage 1.
- **If the correct fix changes architecture** (a new component, a changed runtime flow, a new constraint model such as per-user idempotency keys, a change to a constitution principle), it is not a minimal fix: stop, write a `proposed` ADR from `docs/_templates/adr.md` with the options, get the user's approval, then implement. In stage 7, update the affected `architecture/` pages and move the ADR to `accepted`; link the ADR in the bug record.
- Check `docs/constitution.md`: a bug that is a violation of a principle (C-01..C-15) gets that id in the bug record, and the fix must not add a second violation.

## Stage 7 - Record (required, part of done)
1. Take the next number from `docs/bugs/INDEX.md`; copy `docs/_templates/bug-record.md` to `docs/bugs/BUG-NNNN-<slug>.md`; fill every section. Fill `related` with every BUG found in stage 0 and add a reciprocal line to those records' `related` (the only allowed edit to an old record).
2. Add the row to the INDEX table and any new tag to the vocabulary.
3. If an audit finding was involved, set its status in `docs/audit/AUDIT-REPORT.md` (fixed + BUG id, or refuted/cannot-reproduce + evidence).
4. Correct the knowledge base where the bug showed it was wrong or incomplete: `docs/architecture/` pages if a flow or dependency was described wrongly, `docs/domain/<area>.md` (rules, invariants, "Known gaps": remove the gap only if fixed and tested), `docs/reference/*` (error codes, data model, API) if they changed.
5. Verification command output for the fix goes in the record or the handoff note, not just in chat; list the `TC-` ids that were added or re-run and their recorded results.
Commit message: `fix(<area>): <what> [BUG-NNNN]`.
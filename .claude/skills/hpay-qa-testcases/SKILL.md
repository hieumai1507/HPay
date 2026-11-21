---
name: hpay-qa-testcases
description: Generate, update and execute the HPay manual/automated QA test-case workbooks (docs/qa/test-cases/*.xlsx) from the test-case generator, keeping recorded execution results. Use when adding or changing test cases, mapping edge-case IDs (E/M/A) to tests, recording a run result, or preparing the test-then-fix loop in hpay-feature stage 6 / hpay-bugfix. Not for writing Go/TS unit tests (use test-driven-development).
---

# hpay-qa-testcases

Two kinds of workbook live in `docs/qa/test-cases/`:
1. **The five catalog workbooks** (`01`..`05`): the standing regression catalog, generated from Python data (below).
2. **One workbook per feature**, `<slug>-testcase.xlsx` (for example `w4-pin-reset-testcase.xlsx`). The cases of a new feature are NOT added to the catalog or to `generate_test_cases.py`. The skill designs the cases for the feature (hpay-feature stage 4), writes them to `docs/specs/<slug>/test-cases.json` next to the spec, and `render_feature_workbook.py` only turns that file into the workbook. The script holds no cases and never touches the catalog workbooks, so recorded results there cannot be erased. Results of a run are written back into the JSON (`status`, `actual`, `evidence`, `executed_by`, `last_run`) and the workbook is rendered again.
```
python -X utf8 .claude/skills/hpay-qa-testcases/scripts/render_feature_workbook.py <slug>
```
The JSON shape is in the docstring of the script. Fold a stable feature case set into the catalog only when the owner asks for it.

The 5 workbooks in `docs/qa/test-cases/` (personal user, business maker-checker, admin ops, platform cross-cutting, edge cases) are generated from data in `scripts/generate_test_cases.py` (the case catalog is Python data, not hand-edited cells). Recorded results (Status, Actual Result, Evidence, Executed By, Last Run Date, Defect ID/Notes = columns M-R) are NOT produced by the generator, so they are carried over by `merge_results.py`.

## Rules
1. Never run `generate_test_cases.py` directly against `docs/qa/test-cases` - it writes every Status as "Not Run" and erases recorded results. Always use `merge_results.py`.
2. Edit the catalog in `generate_test_cases.py` (IDs stay stable: `TC-<GROUP>-NNN`, edge IDs E01-E20, M01-M12, A01-A07), then run the merge.
3. All text in English.
4. Run with `python -X utf8` (Windows console encoding).

## Commands (from the repo root)
```
# regenerate, preserving recorded results (default recorded dir = out dir = docs/qa/test-cases)
python -X utf8 .claude/skills/hpay-qa-testcases/scripts/merge_results.py

# dry run into a scratch directory
python -X utf8 .claude/skills/hpay-qa-testcases/scripts/merge_results.py --out .gotmp/qa-verify
```
The merge aborts if the Test-ID set changes between old and new workbooks. When you intentionally add or remove cases, run it with `--recorded` pointing at a directory that contains only workbooks with the same ID set, or update the recorded workbook first; never bypass the ID check by editing the script.

## Recording a result (test-then-fix loop)
1. Execute the case (automated: the named `TestEdge_*` / Playwright spec; manual: steps in the sheet).
2. Update columns M-R in the workbook (Status in {Not Run, Pass, Fail, Blocked, N/A}) with the real outcome and evidence (command output or file:line). Never mark Pass for something not executed.
3. On Fail: open a `hpay-bugfix` run, cite the Test ID in the defect column, and re-run the case after the fix.
4. Keep a copy before bulk changes: `Copy-Item docs/qa/test-cases .gotmp/qa-backup -Recurse`.

## References
- [references/test-case-schema.md](references/test-case-schema.md) - column schema, ID convention, executor rules, sheet breakdown.
- [references/qa-architecture.md](references/qa-architecture.md) - how the QA suite fits the system (API tests, e2e, manual).

## Known limits
Windows Application Control may block Go test executables on the dev machine; backend tests run through `scripts/test-backend.ps1` (Docker). Playwright e2e needs a running backend and the e2e seed (`go run ./cmd/seed --e2e-admins`).

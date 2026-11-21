---
name: hpay-bugfix
description: Workflow for fixing a bug, failing test, audit finding, or unexpected behavior in HPay. Use when something is broken, a test fails, or an item from docs/audit/AUDIT-REPORT.md must be fixed. Reproduce first, root-cause before any fix, then a regression test, fix, verify and review. Not for new features (use hpay-feature).
argument-hint: "<bug description, failing test, or audit finding id>"
---

# hpay-bugfix - reproduce, root-cause, fix, prove

| # | Stage | Output | Gate | Effort |
|---|-------|--------|------|--------|
| 1 | Reproduce | failing test or exact repro commands | The bug is observed with fresh output. Audit findings are claims until reproduced: a finding that cannot be reproduced is marked REFUTED in the audit file, not "fixed" | medium |
| 2 | Root cause | `docs/handoff/<slug>-rootcause.md` | Cause named with file:line evidence; list every other call site with the same flaw | high |
| 3 | Regression test | a test that fails now, for that cause | Test fails for the stated reason | medium |
| 4 | Fix | minimal change | Regression test passes; related tests still pass | medium (high for money/auth) |
| 5 | Verify | evidence | Same commands as `hpay-feature` stage 6, fresh output | low |
| 6 | Review | `hpay-review` | Clean | high |

Rules
- Use `systematic-debugging` for stages 1-2: **no fix without root cause**. Symptom patches are rejected in review.
- Use `test-driven-development` for 3-4.
- Money/auth/migration bugs: add a second regression test for concurrency or replay (double submit, redelivery, parallel requests), and run `go test -race`.
- Before editing a symbol: GitNexus `impact` and report blast radius; before commit: `detect_changes`.
- **3 failed fix attempts: stop and report** what was tried and what was learned.
- After the fix, update `docs/audit/AUDIT-REPORT.md` (status + commit) or the relevant doc if behavior changed.
- If the bug is really a missing business rule, switch to `hpay-feature` stage 1.

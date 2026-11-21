---
name: hpay-review
description: Pre-merge review of the current change in HPay. Runs the fintech invariant checklist plus three independent reviewers in parallel (code, security, tests), then merges findings into one verdict. Use before commit or PR, after implementing a task, or when asked to review/check my code.
disable-model-invocation: false
---

# hpay-review - invariants first, then parallel reviewers

## Step 1 - invariant gate (always, main session, low effort)
Run skill `fintech-review` against the diff. Any FAIL blocks.

## Step 2 - three independent reviewers (parallel, high effort)
Spawn all three in ONE assistant turn so they run concurrently. Give each only the diff, the spec/plan path, and its brief. Reviewers are read-only (no edits).

| Agent | Agent file | Looks for |
|-------|-----------|-----------|
| code-reviewer | `.claude/agents/code-reviewer.md` | correctness, race conditions, state-machine discipline, readability, architecture fit |
| security-auditor | `.claude/agents/security-auditor.md` | authn/authz and IDOR, idempotency scope, secrets/PII in logs, SSRF, file upload, rate limits, env guards |
| test-engineer | `.claude/agents/test-engineer.md` | AC coverage, failure/concurrency/replay cases, tests that cannot fail |

## Step 3 - merge (main session)
Deduplicate; rank blocker > high > medium > low > nit; every finding needs file:line and a concrete failure scenario, otherwise drop it. For a second opinion on HIGH-RISK plans use `doubt-driven-development`.

## Output
`VERDICT: READY` or `VERDICT: BLOCKED (n)` followed by the ranked list. Do not claim READY unless step 1 passed and no blocker/high remains.

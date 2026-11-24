---
name: hpay-review
description: Pre-merge review of the current change in HPay. Runs the fintech invariant checklist plus three independent reviewers in parallel (code, security, tests), then merges findings into one verdict. Use before commit or PR, after implementing a task, or when asked to review/check my code.
disable-model-invocation: false
---

# hpay-review - invariants first, then parallel reviewers

## Step 1 - invariant gate (always, main session, low effort)
Run skill `fintech-review` against the diff. Any FAIL blocks.

## Step 1b - constitution and architecture conformance (main session, medium effort)
1. Check the diff against `docs/constitution.md`: for each C-xx the change touches, is the plan's constitution check true of the code as written? A principle that is violated without an accepted ADR is a blocker.
2. Compare the change with its spec's **Architecture impact**: a new package, table owner, background job, external call, endpoint family, dependency, or change to auth / idempotency / locking / outbox / error handling that the spec classified as `none` is a blocker (the classification was wrong; stop and reclassify).
3. For `amend` / `new`: the referenced ADR exists and is `accepted` (or `proposed` only while the review is part of the design gate), and the architecture pages named in the spec were actually updated in this change. Pages that still describe the old flow are a high finding.
4. Look for decisions made during implementation that have no ADR.
5. **User guide.** If the diff touches src/admin/src or src/mobile/app in a way a person can see (a page, tab, label, button, flow, role or permission, a message), the spec must say User guide impact: updated and the diff must contain the matching change to docs/guides/admin-user-guide.md (or the mobile guide) plus retaken screenshots, and 
ode docs/guides/admin/check-guide.mjs must pass. A UI change with a stale guide, or marked 
ot affected without a reason, is a **high** finding.

6. **Performance.** If the diff touches the money path, locking, the workflow engine or a hot path (see `hpay-feature` stage 7 step 6), the verify note or bug record must hold before/after load-test numbers from `docs/quality/performance/` and a passing ledger integrity check. Missing numbers for such a change are a high finding; a regression without an explanation is a blocker.

## Step 2 - three independent reviewers (parallel, high effort)
Spawn all three in ONE assistant turn so they run concurrently. Give each only the diff, the spec/plan path, and its brief. Reviewers are read-only (no edits).

| Agent | Agent file | Looks for |
|-------|-----------|-----------|
| code-reviewer | `.claude/agents/code-reviewer.md` | correctness, race conditions, state-machine discipline, readability, architecture fit (dependency direction vs `architecture/building-blocks.md`) |
| security-auditor | `.claude/agents/security-auditor.md` | authn/authz and IDOR, idempotency scope, secrets/PII in logs, SSRF, file upload, rate limits, env guards |
| test-engineer | `.claude/agents/test-engineer.md` | AC coverage, failure/concurrency/replay cases, tests that cannot fail |

## Step 3 - merge (main session)
Deduplicate; rank blocker > high > medium > low > nit; every finding needs file:line and a concrete failure scenario, otherwise drop it. For a second opinion on HIGH-RISK plans use `doubt-driven-development`.

## Output
`VERDICT: READY` or `VERDICT: BLOCKED (n)` followed by the ranked list. Do not claim READY unless steps 1 and 1b passed and no blocker/high remains.

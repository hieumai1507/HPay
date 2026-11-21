---
name: gen-feature-plan
description: MANDATORY first step before implementing ANY feature in hpay repos. Generates the two required planning documents (implementation-plan.html + solution-architecture.html with mermaid diagrams) into the code repo. Trigger on "implement X", "build feature X", "start phase FN", or any coding request that has no approved plan yet.
---

# gen-feature-plan — Two planning docs before any code

## Rule (from master-plan.html §1)

No feature code is written until BOTH documents below exist in the target code repo and the user has approved them. If the user asks to implement something and `docs/plans/<feature-slug>/` doesn't exist, run this skill FIRST — do not start coding.

## Output location

```
<code-repo>/docs/plans/<feature-slug>/
├── implementation-plan.html
└── solution-architecture.html
```

`<feature-slug>` = phase id + short name, e.g. `f4-workflow-engine`, `m3-core-screens`, `a2-kyc-queue`.

## Before writing

1. Read the relevant sections of `hpay-docs`: `index.html`, the architecture doc for this repo (00/01/02/03), `04-feature-extensions.html` if applicable, and `planning/master-plan.html` for the phase's scope + edge case list.
2. Read existing code in the repo touched by this feature (do not plan blind against reality).
3. If the design in docs conflicts with what implementation needs → STOP, propose the docs change first (docs-first rule).

## Document 1 — implementation-plan.html

Self-contained HTML (Inter font, project doc styling, mermaid via CDN). Required sections:

1. **Scope & non-goals** — what ships, what explicitly doesn't.
2. **Task breakdown** — ordered table: task, files touched, est. size (S/M/L), depends-on. Granular enough that each row is one commit-ish unit.
3. **Schema changes** — migration list (or "none"), referencing gen-migration rules.
4. **API changes** — endpoints added/modified with `ERR_*` codes; note openapi.yaml diff.
5. **Test plan** — edge case IDs from master plan mapped to `TestEdge_*` names + key non-edge tests.
6. **Rollout & risk** — feature flag? backfill? what breaks if merged half-done; rollback note.
7. **Execution checklist** — the skill-loop for this feature (which of scaffold-module / gen-workflow-step / gen-migration / gen-api-test will be run, in order).

## Document 2 — solution-architecture.html

Same styling. ALL diagrams MUST be mermaid (no ASCII, no images). Required sections:

1. **Context** — where this feature sits in the system: mermaid `flowchart` (component view, only the parts this feature touches — not the whole system).
2. **Main flow** — mermaid `sequenceDiagram` end-to-end (API → engine/handlers → DB → events), including failure branches.
3. **State** — mermaid `stateDiagram-v2` if the feature introduces/changes any status lifecycle.
4. **Data model delta** — mermaid `erDiagram` of new/changed tables and their FKs to existing ones.
5. **Decisions & trade-offs** — short ADR-style table: decision, alternatives, why.
6. **Invariants touched** — which rows of the invariant table (index.html §8) this feature must uphold and how.

## After generating

- Present both files to the user for review. Do NOT proceed to implementation until approved.
- After implementation, if reality diverged from the plan, update the plan files in the same PR (plans are living documents, but divergence must be visible in the diff).

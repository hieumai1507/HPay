---
id: ADR-NNNN
title: <decision in one line>
status: proposed          # proposed | accepted | superseded | deprecated | rejected
date: 2026-11-21
supersedes: []            # ADR ids this replaces
superseded_by: []
spec: specs/<slug>/spec.md
affects:                  # architecture and domain pages this decision changes
  - architecture/<page>.md
  - domain/<context>.md
constitution: []          # principles touched, e.g. [C-02, C-03]
---

# ADR-NNNN: <decision in one line>

An accepted ADR is immutable. To change a decision, write a new ADR that supersedes it; only the `status` and `superseded_by` fields of the old one change.
Lifecycle: `proposed` (written at `hpay-feature` stage 3, reviewed by the user before any code) -> `accepted` (stage 9, once shipped and verified) -> `superseded` or `deprecated`. A decision that is rejected keeps its file with status `rejected` and the reason.

## Context
The forces at play: the problem, the constraints (technical, product, regulatory), what exists today (cite `domain/` and `architecture/` pages and code paths).

## Options considered
At least two real options, including "do nothing / keep current" when that is viable.

| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| A | | | |
| B | | | |

## Decision
What we will do, stated in the active voice, in a few sentences.

## Constitution check
Principles from `constitution.md` this decision touches and how it complies. Any deviation is listed here with its justification.

## Consequences
What becomes easier, what becomes harder, new risks, migration or rollout needs, what must be monitored. Include the follow-up work and where it is tracked.

## Documentation to update
Architecture and domain pages that must change when this is accepted (they are updated in `hpay-feature` stage 9).

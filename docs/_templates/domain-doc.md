---
title: <Bounded context name>
type: domain            # domain | reference | architecture | guide | decision | runbook
status: verified-against-code   # verified-against-code | from-design-doc | unverified
last_verified: 2026-11-21
code:                   # paths that implement this (source of truth)
  - src/backend/internal/<pkg>/
related:
  - domain/<other>.md
audit:                  # audit finding IDs that affect this doc (see ../audit/AUDIT-REPORT.md)
  - BE-C1
---

# <Bounded context name>

## 1. Purpose
One paragraph: what business capability this context owns and what it deliberately does NOT own.

## 2. Key concepts
Terms used here (link to `../glossary.md` for shared terms; define context-specific ones).

## 3. Business rules
Numbered, testable, one sentence each. Use stable IDs so tests, specs and reviews can cite them.

| ID | Rule | Enforced by (code or DB) | Test |
|----|------|--------------------------|------|
| BR-XXX-01 | ... | `path/file.go:Func`, constraint `name` | `TestName` or `none` |

## 4. Lifecycle / state machine
Mermaid `stateDiagram-v2`, then a table of allowed transitions and which function owns each transition.

## 5. Main flows
Mermaid `sequenceDiagram` per flow (happy path + the important failure branches).

## 6. Invariants
Things that must always be true (money, ledger, idempotency, ownership, ordering) and where they are guaranteed.

## 7. Data
Tables/columns this context owns (name, purpose, key constraints). Link `../reference/data-model.md`.

## 8. Interfaces
Endpoints, events, background jobs. Link `../reference/api.md`.

## 9. Edge cases
Edge-case IDs (E/M/A) and what the correct behavior is.

## 10. Known gaps (as-is vs intended)
What the design docs say vs what the code actually does today. Cite audit IDs. Do not hide gaps; do not describe planned fixes as if shipped.

## 11. Code map
Package -> responsibility. File:line for the entry points.

## 12. Open questions
Business decisions not yet made (owner and date).

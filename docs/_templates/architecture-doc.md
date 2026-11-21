---
title: <Component or concept name>
type: architecture
status: verified-against-code   # verified-against-code | proposed | unverified
last_verified: 2026-11-21
code:                           # paths that implement it
  - src/backend/internal/<pkg>/
adr:                            # decisions that shaped this page
  - adr/NNNN-<slug>.md
related:
  - domain/<context>.md
---

# <Component or concept name>

Use this template for a NEW architecture page (a new component, runtime flow or cross-cutting concept). When a change only amends an existing page, edit that page in place instead, update `last_verified`, and keep its front matter.

## 1. Purpose and scope
What this part of the system is for, and what it deliberately does not do. One paragraph.

## 2. Context
Who and what it talks to (actors, other components, external systems). Mermaid `flowchart`.

## 3. Building blocks
Packages / modules / processes, one line of responsibility each, and the real dependency direction (taken from imports, not from intent).

| Block | Responsibility | Depends on | Code |
|-------|----------------|------------|------|

## 4. Runtime view
The two or three scenarios that explain the behavior, as mermaid `sequenceDiagram`, including the important failure branches.

## 5. Data
Tables and events it owns or reads. Link `reference/data-model.md`; do not copy column lists here.

## 6. Cross-cutting concerns
How this part handles idempotency, locking, errors, auth, logging, configuration. Name the constitution principles (`constitution.md`, C-xx) it relies on.

## 7. Quality requirements
Concrete, testable statements (latency, throughput, consistency, availability) and how each is checked. Write "none defined" rather than inventing numbers.

## 8. Decisions
Links to the ADRs that apply, one line each on what they settled.

## 9. Risks and technical debt
Known weaknesses of the current implementation, with audit IDs and bug IDs. Remove an entry only when the fix is shipped and tested.

## 10. Code map
Entry points with `file:line`, so a reader can start reading the code from here.

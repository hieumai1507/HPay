---
title: ADR 0002 English-only documentation
type: decision
status: verified-against-code
last_verified: 2026-11-21
---

# ADR 0002: English-only documentation

Status: Accepted (2026-11-21)

## Context
Documents were a Vietnamese and English mix, in HTML. Agents and reviewers found contradictions between language versions (audit section 4: the `.en` and `.vi` guides gave different thresholds). Some code strings, mockups and seed data are still Vietnamese product content.

## Decision
- All documentation, specs, plans, ADRs, QA workbooks, skills and commit messages are written in English.
- Format is Markdown with Mermaid diagrams. HTML mockups under `docs/design/mockups/` remain design assets.
- User-facing product text (app copy, seed names, error messages that users see) is product content and is not governed by this ADR.
- Older converted documents live in `docs/history/` and are not truth.

## Consequences
- One language removes translation drift and lets agents grep reliably.
- Vietnamese literals remain in code and seed data; documentation must quote them with an English explanation.
- Contributors who think in Vietnamese must write English; review checks this.
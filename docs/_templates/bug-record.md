---
id: BUG-0000                 # next free number from docs/bugs/INDEX.md
title: <one-line symptom>
status: fixed                # open | fixed | wontfix | cannot-reproduce | duplicate-of:BUG-xxxx
severity: high               # blocker | high | medium | low
area: backend/ledger         # app/module, matches docs/domain/<file>.md where possible
tags: [idempotency, race, replay]   # reuse existing tags; see INDEX.md
audit: []                    # audit IDs this relates to, e.g. [BE-C1]
found: 2026-11-21
fixed: 2026-11-21
commit: <short hash or PR>
regression_test: src/backend/internal/<pkg>/<file>_test.go:TestName
related: []                  # other BUG ids that share cause, area or tags
---

# BUG-0000: <title>

## Symptom
What was observed, exactly (error text, wrong value, who sees it). Include the repro command or request.

## Root cause
The actual cause with file:line (before the fix). One paragraph. If the cause was a wrong assumption (not a typo), say which.

## Other places with the same flaw
Every call site checked for the same pattern, and what was done about each.

## Fix
What changed and why this is the minimal correct change. Link the commit.

## Regression test
Test name and what it asserts. Note any case that cannot be tested automatically.

## How to recognize this again
2-4 bullets: the symptoms, log lines or data patterns that point to this class of bug (this is what future triage searches for).

## Docs updated
Which `docs/domain/*`, `docs/reference/*` or ADR files were changed because of this fix.
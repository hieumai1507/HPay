---
id: ADR-0013
title: Enforce one reversal per transaction in the database and park impossible reversals as needs_manual
status: proposed
date: 2026-11-21
supersedes: []
superseded_by: []
spec: specs/w2-reversal-integrity/spec.md
affects:
  - domain/reversal.md
  - domain/approval-maker-checker.md
  - reference/data-model.md
  - architecture/runtime-views.md
constitution: [C-02, C-05, C-13]
---

# ADR-0013: Enforce one reversal per transaction in the database and park impossible reversals as needs_manual

Status: **proposed** (implements the direction of ADR-0009; written under the lead's instruction to complete the plan before review, so the user approves it with the final change set).

## Context
Uniqueness of reversals is claimed in comments but not enforced (audit BE-H3), and a reversal that cannot complete because the receiver spent the money rolls back the admin's whole decision and leaves the request pending with no signal (BE-H4). ADR-0009 already decided the behavior: needs_manual, audit, no negative balance.

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| **A. Partial unique indexes (`transactions(reversal_of) WHERE reversal_of IS NOT NULL`, `approval_requests(subject_type, subject_id) WHERE status = 'pending'`) plus a new `needs_manual` approval status; the executor failure is handled by re-running the decision in a second transaction that records the action and the status** | Database-level guarantee; the approver's decision and an audit trail survive; small schema change | New enum value needs its own migration; the second transaction must be written carefully | Medium |
| B. Receivable (debt) account that lets the receiver go negative | Reversal always completes | Breaks C-02/non-negative customer wallets; needs a collections process; contradicts ADR-0009 | High |
| C. Keep rolling back and only improve the error message | No schema change | The decision is still lost; the request still hangs until it expires | Not acceptable |
| D. Savepoint inside the same transaction | One transaction | The executor receives only tx-scoped queries and cannot open a savepoint; would need an interface change across the approval engine | Higher |

## Decision
Adopt **A**. `needs_manual` is terminal for the decision flow (a person resolves it outside the system for now). The unique indexes make double reversal impossible even if the executor runs twice. The executor additionally locks the original `FOR UPDATE` and requires `completed`.

## Constitution check
C-02 (one posting per reversal via ADR-0010, one reversal per original via the index), C-05 (status changes through the approval service), C-13 (fail closed). No deviation.

## Consequences
- Easier: reversal races and replays are impossible; operators see failed reversals instead of silent expiry.
- Harder: another approval status for every consumer (admin UI, inbox filters, docs). Resolution of `needs_manual` is manual until a follow-up feature.
- Risk: an enum value cannot be added inside a transaction, so the migration is split in two files.

## Documentation to update
`domain/reversal.md`, `domain/approval-maker-checker.md`, `reference/data-model.md`, `architecture/runtime-views.md`, `adr/README.md`, `constitution.md` status column, bug record.
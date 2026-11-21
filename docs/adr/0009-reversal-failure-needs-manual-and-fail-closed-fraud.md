---
title: ADR 0009 Reversal failure needs manual, fail-closed fraud
type: decision
status: proposed-not-implemented
last_verified: 2026-11-21
---

# ADR 0009: Reversal failure goes to needs_manual, fraud failures fail closed

Status: Accepted, not yet implemented

## Context
Two behaviors are unsafe today.
1. When a reversal or a compensation cannot complete because the receiver already spent the money, `ledger.ErrInsufficientBalance` rolls back the whole reversal finalize and the approval stays `pending` (BE-H4); a failed compensation only logs "COMPENSATION FAILED", sets the instance `failed`, may leave the transaction `processing`, and nothing retries it (`dispatchCompensate` uses attempt 101; the sweep ignores `compensating`).
2. The fraud step calls an LLM advisor in the gray zone, but behavior on LLM error or timeout is not specified (audit section 5, Fraud). Rule signals are computed first, and database errors in the step are retryable; the advisory runs off the critical path and only annotates the flag.

## Decision
1. A reversal or compensation that cannot complete moves to an explicit `needs_manual` status (new transaction and workflow status, added by migration and the transition maps) together with an audit record containing who, why, amounts and the blocking wallet. It never creates a negative balance (the `balance >= 0` check stays) and never silently drops the request. Operators resolve it through an admin task (retry, hold, or book a receivable). The sweep includes `compensating` instances and the guarded transaction update handles the new status.
2. Fraud is fail-closed: if rule evaluation or the LLM advisor errors or times out where its verdict is required, the transaction is held for manual review (status `pending_approval` or a dedicated review state with a fraud flag) rather than allowed through. Retryable infrastructure errors retry first, then hold. The decision is recorded on the flag (`reasons`) and audited.

## Consequences
- Removes stuck-and-silent failures and prevents fraud bypass during provider outages.
- More manual work and slower transfers during outages; needs an operator queue, alerting and SLAs.
- Requires migrations for the new status values (`ALTER TYPE ... ADD VALUE` in its own file), engine, approval and admin UI changes, and tests (reversal with empty receiver, failed compensation, advisor timeout).
- Fixes the symptoms of BE-H4 and the compensation gap; the double-reversal constraint (BE-H3) is a separate change.
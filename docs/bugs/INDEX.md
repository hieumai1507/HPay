# Bug ledger

Every bug fixed in this repository gets one file `BUG-NNNN-<slug>.md` here (template: `../_templates/bug-record.md`), created in the same change that fixes it. `hpay-bugfix` searches this ledger before debugging and writes to it after fixing.

Rules
- Numbers are sequential and never reused. Next free number: **BUG-0001**.
- Never delete a record. If a fix was wrong, add a new record and link it in `related`.
- Reuse tags. Tag vocabulary so far: `idempotency`, `race`, `replay`, `authz`, `idor`, `config`, `env-guard`, `migration`, `constraint`, `money-math`, `state-machine`, `outbox`, `workflow`, `frontend-money`, `session`, `upload`, `ssrf`, `docs-drift`, `test-infra`.
- Audit findings (`../audit/AUDIT-REPORT.md`) become bug records when they are reproduced and fixed. A finding that cannot be reproduced gets a record with status `cannot-reproduce` and the evidence.

## Index

| ID | Status | Severity | Area | Tags | Title | Audit |
|----|--------|----------|------|------|-------|-------|
| _(none yet)_ | | | | | | |
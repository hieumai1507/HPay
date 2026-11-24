# Bug ledger

Every bug fixed in this repository gets one file `BUG-NNNN-<slug>.md` here (template: `../_templates/bug-record.md`), created in the same change that fixes it. `hpay-bugfix` searches this ledger before debugging and writes to it after fixing.

Rules
- Numbers are sequential and never reused. Next free number: **BUG-0021** (BUG-0010 and BUG-0011 were taken by parallel work; check the folder before using it).
- Never delete a record. If a fix was wrong, add a new record and link it in `related`.
- Reuse tags. Tag vocabulary so far: `idempotency`, `race`, `replay`, `authz`, `idor`, `config`, `env-guard`, `migration`, `constraint`, `money-math`, `state-machine`, `outbox`, `workflow`, `frontend-money`, `session`, `upload`, `ssrf`, `docs-drift`, `test-infra`.
- Audit findings (`../audit/AUDIT-REPORT.md`) become bug records when they are reproduced and fixed. A finding that cannot be reproduced gets a record with status `cannot-reproduce` and the evidence.

## Index

| ID | Status | Severity | Area | Tags | Title | Audit |
|----|--------|----------|------|------|-------|-------|
| [BUG-0001](BUG-0001-ledger-double-post-on-step-replay.md) | fixed (uncommitted) | blocker | backend/ledger | replay, outbox, workflow, state-machine | A redelivered MOVE_FUNDS step posts the same transaction to the ledger twice | BE-C1 |
| [BUG-0010](BUG-0010-app-env-defaults-to-dev.md) | fixed (uncommitted) | high | backend/config | config, env-guard, authz | A missing APP_ENV silently runs the process as dev (OTP in responses, mock bank route, weak secrets) | BE-H7 |
| [BUG-0011](BUG-0011-reversal-integrity.md) | fixed (uncommitted) | high | backend/reversal | race, constraint, state-machine, money-math | Reversals can be duplicated, applied to a changed original, decided after expiry, and lose the approver's decision when the receiver has no funds | BE-H3, BE-H4 |
| [BUG-0002](BUG-0002-idempotency-key-global-and-unbound.md) | fixed (uncommitted) | high | backend/transfer | idempotency, replay, authz, constraint, migration | Idempotency-Key is global and not bound to the request, so another user's or a changed request returns a stale transaction | BE-H1 |
| [BUG-0003](BUG-0003-money-moving-writes-without-idempotency-key.md) | fixed (uncommitted) | high | backend/transfer | idempotency, replay | Money-moving writes could run without an idempotency key (random server key) and money-request creation was not idempotent | BE-H1 |
| [BUG-0012](BUG-0012-engine-confirm-not-authenticated.md) | fixed (uncommitted) | blocker | backend/transfer | authz, idor, state-machine, replay | Money moves at request time with no authentication; confirm/verify ignore the credential and expose any transaction by id | BE-C2 |
| [BUG-0013](BUG-0013-admin-routes-have-no-role-enforcement.md) | fixed (uncommitted) | high | backend/auth | authz, session, ai-key | Any authenticated admin, including a viewer, can call every admin route; a locked admin keeps access until the token expires | BE-H6 |
| [BUG-0014](BUG-0014-outbound-gateway-ssrf.md) | fixed (uncommitted) | high | backend/gateway | ssrf, config, authz | The action pipeline calls any URL stored in the database, including internal and metadata addresses, and follows redirects | SSRF |
| [BUG-0015](BUG-0015-kyc-upload-accepts-any-file-with-an-image-signature.md) | fixed (uncommitted) | high | backend/kyc | upload, pii, authz | KYC upload accepts any file that starts with a JPEG or PNG signature; files are served without nosniff | BE-H9 (upload part) |
| [BUG-0016](BUG-0016-login-has-no-per-account-lockout-and-leaks-account-existence.md) | fixed (uncommitted) | high | backend/auth | authn, brute-force, enumeration, timing | Login has no per-account lockout; response time reveals which accounts exist | BE-H8 |
| [BUG-0017](BUG-0017-offer-limit-window-race.md) | fixed (uncommitted) | high | backend/offerguard | race, money-math, state-machine | Daily and monthly limits can be exceeded by parallel confirms; limit windows follow the server time zone | BE-H2 |
| [BUG-0018](BUG-0018-limits-measure-request-time-not-start-time.md) | fixed (uncommitted) | medium | backend/offerguard | race, state-machine, money-math | The interval throttle and the quota windows measure when a request was made, not when money started to move | BE-H2 |
| [BUG-0019](BUG-0019-fraud-check-redelivery-duplicates-the-flag.md) | fixed (uncommitted) | medium | backend/fraud | idempotency, replay, outbox | A redelivered FRAUD_CHECK step writes a second fraud flag and a second advisory call | BE-C1 |
| [BUG-0020](BUG-0020-confirm-step-fails-on-redelivery.md) | fixed (uncommitted) | high | backend/workflow | idempotency, replay, state-machine | A redelivered CONFIRM step fails because the transaction is already completed | BE-C1 |

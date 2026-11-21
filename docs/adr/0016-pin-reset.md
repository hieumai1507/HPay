---
id: ADR-0016
title: Reset a forgotten PIN with password plus a one-time code, with an operator fallback
status: accepted
date: 2026-11-21
supersedes: []
superseded_by: []
spec: specs/w4-pin-reset/spec.md
affects:
  - architecture/building-blocks.md
  - architecture/crosscutting-concepts.md
  - architecture/runtime-views.md
  - domain/identity-and-auth.md
  - product/roles-and-permissions.md
constitution: [C-07, C-08, C-09, C-11]
---

# ADR-0016: Reset a forgotten PIN with password plus a one-time code, with an operator fallback

## Context
ADR-0008 and ADR-0014 made the transaction PIN a server-verified second factor: stored as an argon2id hash, five wrong attempts lock it for 15 minutes. That leaves no way back for a user who forgot it. `ChangePIN` needs the current PIN; `SetPIN` needs the password but is meant for the first PIN. The OTP machinery already exists (`otp_challenges`, 6 digits, 120 s, 3 attempts, 30 s cooldown) but has no delivery: the code is returned in the response only when `APP_ENV=dev`.

What other wallets do (sources of mixed quality; the Venmo help page and NIST are authoritative, the rest are blogs): Cash App sends a verification code to the registered e-mail or phone and lets the user set a new PIN; MoMo verifies by OTP to the registered phone and falls back to its hotline for locked accounts or a lost number; Venmo's official page says to reinstall the app for the app PIN and use support with ID for a lost account. Nobody returns the old PIN. NIST SP 800-63B-4 requires a notification to the subscriber after every account recovery and rules out knowledge-based questions.

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| A. Password + one-time code, new PIN chosen by the user; operator `pin.reset` fallback | Matches industry practice and NIST; reuses OTP, hash and lockout; no secret ever sent | Needs a delivery channel; the fallback needs an operator process | Medium |
| B. Send a new random PIN to the user | Simple | Impossible with hashed storage without generating and sending a secret; the PIN would sit in an SMS or mailbox; rejected by every guideline | High risk |
| C. Operator-only reset (call centre) | No channel needed | Slow, costly, a social-engineering target, does not scale | Medium |
| D. Reinstall the app to clear the PIN (Venmo style) | Trivial | The PIN here is server-side; reinstalling would not clear it, and it would let a thief with the phone bypass the PIN | Rejected |

## Decision
Take option A. A signed-in user proves the account password, receives a one-time code for the purpose `pin_reset` through a new `OTPSender` port, and sets a new PIN with the code; the reset clears the attempt counter and the lock. A user who cannot receive the code goes to an operator: a new permission `pin.reset` (superadmin only) clears the PIN after out-of-band identity verification, with a mandatory reason and an audit row; the user then sets a new PIN.
Guardrails: password required, code bound to user and purpose, existing OTP limits (120 s, 3 attempts, 30 s cooldown) plus a 3-per-hour cap, weak PINs refused without consuming the code, a notice to the user after every reset, and a 24-hour cooling period during which a transaction above 2,500,000 VND that needs a PIN is refused (`ERR_PIN_COOLING`) so a thief who completed a reset cannot drain the wallet at once.
The delivery port has a mock (dev and CI) and a log-only sender that refuses to run in production; the real SMS or e-mail provider is chosen in a separate ADR together with the 2FA decision, and until then a production deployment cannot send real codes (reset is unavailable there, which fails closed).

## Constitution check
- C-07: the code and the PIN are hashed, attempts counted atomically, failures audited without the secret. Complies.
- C-08: self-service uses the session user id, never a user id from the body; the operator route needs `pin.reset` checked on the server from the database role. Complies.
- C-09: the code appears in a response only in `dev`. Complies.
- C-11: neither the PIN nor the code is logged or stored in clear; the audit reason holds no secret. Complies.

## Consequences
- Easier: a locked-out user recovers by themselves; the lockout stops being permanent.
- Harder: account takeover now has a new target (password plus phone): the 24-hour cooling period, the notice and the audit rows limit the damage but do not remove it.
- New dependency boundary: `OTPSender`. A production deployment needs the provider ADR before reset works; until then it fails closed.
- An operator with `pin.reset` can clear a PIN (not read or set one); this is a powerful action, so it is superadmin-only, needs a reason and is audited.
- Follow-up: provider choice (with 2FA), login-password reset, a per-user notification centre.

## Documentation to update
`architecture/building-blocks.md`, `architecture/crosscutting-concepts.md`, `architecture/runtime-views.md`, `domain/identity-and-auth.md`, `reference/api.md`, `reference/error-codes.md`, `reference/data-model.md`, `reference/configuration.md`, `product/roles-and-permissions.md`, `adr/README.md` (row), ADR-0008 (reset flow no longer open).
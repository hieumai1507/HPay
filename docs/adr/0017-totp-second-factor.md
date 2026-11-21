---
id: ADR-0017
title: TOTP built in-house as the 2fa band factor, and a pluggable code-delivery provider starting with Twilio Verify
status: accepted
date: 2026-11-21
supersedes: []
superseded_by: []
spec: specs/w4-2fa/spec.md
affects:
  - architecture/building-blocks.md
  - architecture/crosscutting-concepts.md
  - architecture/runtime-views.md
  - architecture/deployment.md
  - domain/identity-and-auth.md
  - domain/transfer-and-fees.md
  - product/roles-and-permissions.md
constitution: [C-07, C-08, C-09, C-10, C-11, C-13]
---

# ADR-0017: TOTP built in-house as the 2fa band factor, and a pluggable code-delivery provider starting with Twilio Verify

An accepted ADR is immutable. This one completes the direction of ADR-0008 and ADR-0014 (the 2fa band was left "not available yet") and builds on the `Sender` port of ADR-0016.

## Context
The offer matrix has a `2fa` band (50,000,000 VND and above) that the server refuses, because there is no second factor to verify (ADR-0014, `ErrAuthMethodUnavailable`). Separately, one-time codes and notices have no real delivery channel: outside production the log-only sender is used and production uses a refusing sender (ADR-0016), so PIN reset and the otp band cannot work in production.

Facts gathered (prices are vendor-advertised and not verified for Vietnam; the legal text was read only through secondary summaries):
- NIST SP 800-63B-4 classes out-of-band authentication over the telephone network (SMS) as a restricted authenticator: allowed at AAL2 with an alternative and a risk explanation, not at AAL3; SIM change and porting should be considered. OTP apps (TOTP) are not restricted but are not phishing-resistant either.
- State Bank of Vietnam Decision 2345/QD-NHNN (in force 2024-07-01): online transfers above 10 million VND per transaction or above 20 million per day need biometric authentication. Whether HPay is in scope is for the owner and legal advice; nothing here decides it.
- Provider prices: Twilio Verify about USD 0.05 per successful verification plus channel fees; Vonage about EUR 0.052; AWS only per message and no OTP logic; aggregators quote USD 0.024 to 0.105 per Vietnam SMS; no price or API found for Vietnamese providers.

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| A. TOTP built in-house (RFC 6238) for the 2fa band; delivery port with Twilio Verify as the first adapter | No per-message cost and no SIM dependency for the strong factor; small, well-specified code; vendor only for delivery; a factor seam allows biometrics later | We hold a secret per user (must be encrypted); enrolment and recovery flows to build | Medium |
| B. SMS OTP also as the 2fa factor | Fastest, no enrolment | Restricted authenticator, SIM swap risk, per-message cost, depends on the vendor for every large transaction | Medium, weaker |
| C. A vendor service for everything (Twilio Verify TOTP or Authy-like) | Less code | Vendor lock-in for the strongest factor, per-verification fees, a vendor outage blocks large transfers | Higher cost |
| D. Leave the 2fa band refused | Nothing to build | Users cannot send 50 million or more | Product gap |

## Decision
Take option A.
- **Factor:** TOTP per RFC 6238 (HMAC-SHA1, 6 digits, 30 s step) verified in `internal/auth`. The 2fa band needs the PIN **and** a TOTP code (the otp band already needs the PIN and an OTP). One step of drift either way is accepted; a time step is accepted once (last used step stored), so a code cannot be replayed.
- **Secret protection:** the secret is stored AES-256-GCM encrypted with a key from `TOTP_ENCRYPTION_KEY`; it is shown once at enrolment, never returned again, never logged. The key is required outside dev and ci (startup fails otherwise). A database leak alone does not yield secrets.
- **Enrolment and recovery:** enrolment needs the account password and a first valid code; 10 single-use recovery codes (stored hashed) are shown once; an operator permission `totp.reset` (superadmin only, reason, audit, notice) removes a lost TOTP.
- **Abuse limits:** the same lockout rule as the PIN (5 wrong in a row lock TOTP use for 15 minutes), atomic counter, audit without secrets.
- **Seam for more factors:** the confirm step asks a `Factor` interface (`Verify(ctx, user, credential)`), so a biometric factor (device-bound, mobile) is a new implementation, not a change to the flow.
- **Delivery:** the `Sender` port gets a Twilio adapter chosen by `SENDER_PROVIDER` (changed during implementation with the owner's approval: it uses Twilio Programmable Messaging to deliver the code HPay generates, not Twilio Verify, because Verify generates and checks its own codes and HPay already keeps hashing, counting and expiry for PIN reset and the otp band; the CustomCode parameter of Verify could not be confirmed as available to every account); the mock stays default outside production; production without a provider still fails closed; the adapter's HTTP client goes through the outbound guard (C-10) and never logs a code.
- **Not decided here:** whether the 10 million biometric rule applies to HPay. The spec records it; the owner decides with legal advice.

## Constitution check
- C-07: PIN plus TOTP verified on the server, secret encrypted, attempt counter and lockout, audit without secrets. Complies.
- C-08: the operator route needs `totp.reset`, checked on the server; enrolment uses the session user. Complies.
- C-09: the secret and provisioning URI are returned once at enrolment (needed to set up the app) and nowhere else; codes never in responses outside dev. Complies.
- C-10: the Twilio client uses the guarded transport (https only, private addresses blocked, no redirects). Complies.
- C-11: no secret, code or recovery code in logs or audit. Complies.
- C-13: no enrolled TOTP means the 2fa transaction is refused; no provider in production means no send. Complies.

## Consequences
- Easier: users can send 50 million or more; PIN reset and the otp band can deliver codes in production once keys are supplied.
- Harder: a new secret to manage (`TOTP_ENCRYPTION_KEY`; losing it makes every TOTP secret unreadable, so users would re-enrol through the operator route; key rotation is a follow-up) and an enrolment screen to build in the mobile app.
- Not phishing-resistant: TOTP can be phished; the biometric factor (follow-up) is the answer to that.
- Vendor: Twilio is optional configuration, not a hard dependency; an outage only affects code delivery (reset, otp band), not the TOTP factor.
- Follow-ups: mobile enrolment and confirm screens, admin screen for `totp.reset`, key rotation, a biometric factor, a Vietnamese provider adapter, the compliance decision on Decision 2345.

## Documentation to update
`architecture/building-blocks.md`, `architecture/crosscutting-concepts.md`, `architecture/runtime-views.md`, `architecture/deployment.md`, `domain/identity-and-auth.md`, `domain/transfer-and-fees.md`, `reference/api.md`, `reference/error-codes.md`, `reference/data-model.md`, `reference/configuration.md`, `product/roles-and-permissions.md`, `adr/README.md` (row), ADR-0014 (2fa no longer open).
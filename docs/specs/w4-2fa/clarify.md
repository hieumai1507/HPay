---
title: 2FA and the real code-delivery provider - clarification
type: spec
status: done
last_verified: 2026-11-21
---

# Clarification

**Asked and answered 2026-11-21:**
1. Second factor for transactions in the 2fa band: **TOTP built in-house** with a hook for on-device biometrics later (recommended option chosen). No per-message cost, no SIM dependency, no vendor.
2. Code delivery provider: **a pluggable port plus Twilio Verify** as the first adapter (recommended option chosen); keys come from the owner later, a mock is used until then.

**Provider research** (prices are vendor-advertised and unverified for Vietnam): Twilio Verify about $0.05 per successful verification plus channel fees; Vonage Verify about EUR 0.052; AWS End User Messaging has no verify fee but per-message pricing and no OTP logic; aggregator pages quote USD 0.024 to 0.105 per Vietnam SMS; no price or API was found for eSMS, SpeedSMS, VIETGUYS or Stringee. NIST SP 800-63B-4 classes OTP over the telephone network as a "restricted" authenticator (allowed at AAL2 with an alternative and a risk explanation, not at AAL3).

**Legal point raised, not decided here:** State Bank of Vietnam Decision 2345/QD-NHNN (in force since 2024-07-01) requires biometric authentication for online transfers above 10,000,000 VND per transaction or above 20,000,000 VND per day. Sources are secondary (news and legal summaries). If HPay is a regulated wallet, the 2fa band may need a biometric factor from 10 million rather than 50 million. This spec builds the factor framework so a biometric factor can be added; it does not decide compliance. Owner and legal to decide.

**Assumptions the owner accepted by approving the recommended options:**
- TOTP per RFC 6238 (SHA-1, 6 digits, 30 s step, one step of drift allowed), the standard that authenticator apps implement.
- The 2fa band needs the PIN **and** the TOTP code, as the otp band needs the PIN **and** the OTP.
- Ten single-use recovery codes are shown once at enrolment.
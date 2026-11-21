---
title: PIN reset - clarification
type: spec
status: done
last_verified: 2026-11-21
---

# Clarification: forgotten transaction PIN

**Asked of the owner and answered 2026-11-21:**
1. OTP channel: design a delivery port (`OTPSender`) with a mock for dev; choose the real SMS or e-mail provider in a separate ADR together with the 2FA decision.
2. Fallback for users who cannot receive the OTP (lost phone): yes, an operator permission `pin.reset` in this same change.

**Research** (what other wallets do, with the reliability of each source) is in the conversation record and summarised in the ADR. Conclusions that shaped the design: nobody sends a PIN back (HPay cannot either: only an argon2id hash is stored); the common pattern is a one-time code that lets the user choose a new PIN; the phone-call route exists for locked accounts and lost numbers; NIST SP 800-63B-4 requires a notification after every recovery and forbids knowledge-based questions.

**Assumptions the owner accepted by approving the recommended options:**
- Self-service requires the signed-in session AND the account password AND an OTP (three things).
- After a reset, a cooling period limits transaction size (see spec).
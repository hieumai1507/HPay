---
title: TOTP second factor - test cases
type: spec
status: approved
last_verified: 2026-11-21
---

# Test cases: TOTP second factor and code delivery

Source of the workbook: `test-cases.json` in this folder. Render it with `python -X utf8 .claude/skills/hpay-qa-testcases/scripts/render_feature_workbook.py w4-2fa` (writes `docs/qa/test-cases/w4-2fa-testcase.xlsx`).

| TC | AC | Type | Pri | Expected | Test |
|----|----|------|-----|----------|------|
| TC-2FA-001 | AC-1 | Happy Path | P0 | A pending secret exists, the response carries the secret and an otpauth URI once, TOTP is not active | `TestTOTP_EnrollReturnsSecretOnce` |
| TC-2FA-002 | AC-1 | Security/AuthZ | P0 | 401 ERR_INVALID_CREDENTIALS and no secret is created | `TestTOTP_EnrollNeedsPassword` |
| TC-2FA-003 | AC-2 | Happy Path | P0 | TOTP active, 10 recovery codes returned once and stored only as hashes | `TestTOTP_ConfirmActivatesAndIssuesRecoveryCodes` |
| TC-2FA-004 | AC-3 | Negative | P0 | Stays inactive; the fifth wrong attempt locks TOTP use; the right code is refused while locked | `TestTOTP_WrongCodesLockUse` |
| TC-2FA-005 | AC-7 | Boundary | P0 | One step of drift is accepted, two steps are refused | `TestTOTP_AcceptsOneStepOfDrift` |
| TC-2FA-006 | AC-6 | Edge Case | P0 | The second use is refused: a step is accepted once | `TestTOTP_CodeIsSingleUsePerStep` |
| TC-2FA-007 | AC-9 | Happy Path | P0 | Accepted once, then burnt; the remaining count drops by one | `TestTOTP_RecoveryCodeIsSingleUse` |
| TC-2FA-008 | AC-11 | Security/AuthZ | P0 | The stored value is ciphertext, not the secret; a wrong key cannot decrypt it | `TestTOTP_SecretIsEncryptedAtRest` |
| TC-2FA-009 | AC-12 | Edge Case | P0 | Exactly one succeeds | `TestTOTP_ConcurrentVerifyOneWins` |
| TC-2FA-010 | AC-4 | Happy Path | P0 | The request is held in awaiting_auth with auth_method 2fa (no longer refused) | `TestRequest_TwoFactorBandIsHeldWhenEnrolled` |
| TC-2FA-011 | AC-5 | Happy Path | P0 | The workflow starts exactly once and the money moves | `TestConfirm_TwoFactorStartsWorkflowWithPINAndTOTP` |
| TC-2FA-012 | AC-5 | Negative | P0 | Neither starts the workflow; the wrong code counts toward the TOTP lockout | `TestConfirm_TwoFactorWithoutValidTOTPDoesNotStart` |
| TC-2FA-013 | AC-8 | Negative | P0 | 403 ERR_TOTP_NOT_ENROLLED and nothing is held | `TestRequest_TwoFactorBandRefusedWithoutEnrolment` |
| TC-2FA-014 | AC-14 | Edge Case | P1 | The TOTP is ignored and the PIN decides | `TestConfirm_PINBandIgnoresTOTP` |
| TC-2FA-015 | AC-5 | Security/AuthZ | P0 | Accepted once; the same recovery code on another transaction is refused | `TestConfirm_TwoFactorAcceptsRecoveryCodeOnce` |
| TC-2FA-016 | AC-10 | Happy Path | P0 | TOTP and recovery codes removed, audit totp.reset with admin id and reason, user notified | `TestTOTPAdminReset_RemovesAndAudits` |
| TC-2FA-017 | AC-10 | Security/AuthZ | P0 | 403 ERR_FORBIDDEN_PERMISSION for viewer, kyc_reviewer, risk_officer and ops | `TestTOTPAdminReset_OnlySuperadmin` |
| TC-2FA-018 | AC-1..3 | Happy Path | P0 | 200, 401 and 400 map to the documented error codes; GET never returns the secret | `TestTOTPHTTP_EnrolConfirmAndStatus` |
| TC-2FA-019 | AC-13 | Happy Path | P0 | The right path, Basic auth and form fields are sent; the response is parsed; the code never appears in a log | `TestTwilioSender_SendsThroughVerifyAPI` |
| TC-2FA-020 | AC-13 | Security/AuthZ | P0 | Production without a provider refuses; production with Twilio settings uses Twilio; dev and ci use the log-only sender | `TestSenderSelection_ByEnvironmentAndProvider` |
| TC-2FA-021 | C-10 | Security/AuthZ | P0 | The call is refused by the outbound guard before any network input or output | `TestTwilioSender_UsesOutboundGuard` |
| TC-2FA-022 | C-09 | Security/AuthZ | P0 | Startup fails without a 32-byte key outside dev and ci; dev and ci get a fixed test key | `TestConfig_TOTPKeyRequiredOutsideDev` |
| TC-2FA-023 | AC-10 | Security/AuthZ | P0 | totp.reset is in both and the tables are equal; the new route is in the RBAC table | `TestRolePermissionsMatchFrontend + TestEveryAdminRouteHasAPermission` |

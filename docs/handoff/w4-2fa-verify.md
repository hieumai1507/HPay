# Verification: TOTP second factor and Twilio delivery adapter (ADR-0017)

Date 2026-11-21. Spec `docs/specs/w4-2fa/spec.md`, cases `test-cases.md` and `test-cases.json`, workbook `docs/qa/test-cases/w4-2fa-testcase.xlsx` (TC-2FA-001 to 023, a feature workbook of its own).

## Results
| Check | Result |
|---|---|
| Cases TC-2FA-001 to 023 | 23 Pass, 0 Fail, 0 Blocked, 0 Not Run |
| Full backend suite, `-race -p 1`, 4-connection pool (CI-like) | all 23 packages ok |
| `go build`, `go vet`, `gofmt -l` (Docker, Go 1.26) | clean |
| OpenAPI YAML parse and `check-openapi-sync.py` | clean |
| Admin `tsc`, eslint, vitest | 0 type errors, 0 lint errors (6 old warnings), 20 of 20 tests |
| RFC 6238 vectors (6 time points) | match |

## Mutation checks (each made a test fail, then the code was restored)
- Step guard removed from the SQL: `TestTOTP_GuardedClaimHasOneWinner` fails (16 winners instead of 1). The service-level replay and concurrency tests did NOT catch this alone, because the Go side also compares the last step, so the store-level test was added.
- Recovery code reusable: `TestTOTP_RecoveryCodeIsSingleUse` fails.
- Secret stored in clear: `TestTOTP_SecretIsEncryptedAtRest` fails.
- Two steps of drift accepted: `TestTOTP_AcceptsOneStepOfDrift` fails.
- 2fa band skips the TOTP check: `TestConfirm_TwoFactorWithoutValidTOTPDoesNotStart` fails.
- PIN band also demands TOTP: `TestConfirm_PINBandIgnoresTOTP` fails.
- Loopback allowed for the Twilio client: `TestTwilioSender_UsesOutboundGuard` fails.
- `totp.reset` given to `ops`: `TestTOTPAdminReset_OnlySuperadmin` fails.

## Changed during implementation (owner approved)
The adapter uses Twilio Programmable Messaging to deliver the code HPay generates, not Twilio Verify (Verify generates and checks its own codes; HPay already keeps hashing, counting and expiry; the `CustomCode` parameter could not be confirmed for every account). ADR-0017 records this.

## Not verified
- The Twilio adapter was tested only against a local TLS stand-in that follows Twilio's documented request shape (path, Basic auth, form fields). It has never called the real Twilio: that needs account keys from the owner. The error body shape of Twilio was not confirmed.
- No mobile or admin screen exists for enrolment, reset or the operator actions; only the API.
- Whether State Bank Decision 2345/QD-NHNN (biometric above 10 million VND) applies to HPay is open (owner and legal). No biometric factor was built; the factor seam exists.
- `TOTP_ENCRYPTION_KEY` rotation is not implemented.
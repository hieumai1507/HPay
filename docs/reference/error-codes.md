---
title: Error codes
type: reference
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/internal/api/errmap.go, src/backend/internal/api/middleware.go, src/backend/internal/api/admin_ai.go, src/backend/internal/api/mockbank_handler.go, src/backend/api/openapi.yaml]
---

# Error codes

Wire format (all errors): `{"code": "ERR_...", "message": "...", "request_id": "..."}`. Note: middleware aborts (`RequireAuth`, `RateLimit`, `RequireIdempotencyKey`) omit `request_id` in the body (the `X-Request-Id` header is still set).

`respondErr` walks `errTable` in order with `errors.Is`; the first match wins. No match returns 500 `ERR_INTERNAL` / "internal error". `respondValidation` returns 400 `ERR_VALIDATION`.

## Codes from errTable (errmap.go)

| Code | HTTP | Meaning | Raised in (Go error) |
|---|---|---|---|
| ERR_PHONE_TAKEN | 409 | Phone already registered | auth.ErrPhoneTaken |
| ERR_INVALID_CREDENTIALS | 401 | Wrong phone or password | auth.ErrInvalidCredentials |
| ERR_ACCOUNT_LOCKED | 403 | User or admin account locked | auth.ErrAccountLocked |
| ERR_LOGIN_LOCKED | 429 | Five failed logins: the account is locked for 15 minutes, even for the correct password | auth.ErrLoginLocked |
| ERR_TOKEN_REUSED | 401 | Refresh token reused, all sessions revoked | auth.ErrTokenReused |
| ERR_TOKEN_EXPIRED | 401 | JWT expired | auth.ErrTokenExpired |
| ERR_INVALID_TOKEN | 401 | JWT missing or invalid | auth.ErrTokenInvalid |
| ERR_OTP_COOLDOWN | 429 | OTP requested too soon | auth.ErrOTPCooldown |
| ERR_OTP_EXPIRED | 400 | OTP expired or none issued | auth.ErrOTPExpired |
| ERR_OTP_MAX_ATTEMPTS | 400 | OTP attempts exhausted | auth.ErrOTPMaxAttempts |
| ERR_OTP_INVALID | 400 | OTP code wrong | auth.ErrOTPMismatch |
| ERR_AMOUNT_INVALID | 400 | Amount not positive | wallet.ErrAmountInvalid |
| ERR_LIMIT_EXCEEDED | 400 | Above the org per-transaction cap (org topup and disbursement only; personal limits come from the offer: `ERR_LIMIT_PER_TXN`, `ERR_LIMIT_DAILY`, `ERR_LIMIT_MONTHLY`) | wallet.ErrLimitExceeded |
| ERR_OFFER_INACTIVE | 403 | The user's offer is inactive; it does not fall back to another offer | offerguard.ErrOfferInactive |
| ERR_WALLET_NOT_FOUND | 404 | Wallet not found | wallet.ErrWalletNotFound |
| ERR_TXN_NOT_FOUND | 404 | Transaction not found | wallet.ErrTxnNotFound, reversal.ErrTxnNotFound |
| ERR_RECEIVER_NOT_FOUND | 404 | Receiver or payer not found | transfer.ErrReceiverNotFound, org.ErrReceiverNotFound, moneyrequest.ErrPayerNotFound |
| ERR_VAULT_NOT_FOUND | 404 | Vault not found (also for another user's vault) | vault.ErrNotFound |
| ERR_VAULT_LABEL_TAKEN | 409 | Vault name already used by the user | vault.ErrLabelTaken |
| ERR_VAULT_DIRECTION_INVALID | 400 | Direction not deposit/withdraw | vault.ErrDirectionInvalid |
| ERR_VALIDATION | 400 | Invalid input | vault.ErrLabelRequired, org.ErrBadRole, moneyrequest.ErrBadSplit, kyc.ErrBadImage, respondValidation |
| ERR_PAYLOAD_TOO_LARGE | 413 | Request body too large | `/v1/kyc/submit` body above 12 MiB (`respondTooLarge`) |
| ERR_SERVICE_NOT_IN_OFFER | 403 | Service not in the user's offer | offerguard.ErrServiceNotInOffer |
| ERR_SERVICE_DISABLED | 403 | Service disabled in the offer | offerguard.ErrServiceDisabled |
| ERR_AMOUNT_BELOW_MIN | 400 | Below offer minimum | offerguard.ErrBelowMin |
| ERR_LIMIT_PER_TXN | 400 | Above per-transaction limit | offerguard.ErrLimitPerTxn |
| ERR_LIMIT_DAILY | 400 | Daily limit exceeded | offerguard.ErrLimitDaily |
| ERR_LIMIT_MONTHLY | 400 | Monthly limit exceeded | offerguard.ErrLimitMonthly |
| ERR_TOO_FREQUENT | 429 | Offer `interval_seconds` throttle | offerguard.ErrTooFrequent |
| ERR_INSUFFICIENT_BALANCE | 400 | Balance too low | ledger.ErrInsufficientBalance |
| ERR_SELF_TRANSFER | 400 | Same wallet on both sides | ledger.ErrSelfTransfer |
| ERR_NOT_ORG_MEMBER | 403 | Caller not in the org | org.ErrNotMember |
| ERR_FORBIDDEN_ROLE | 403 | Org role not allowed | org.ErrForbiddenRole, approval.ErrNotChecker |
| ERR_MEMBER_EXISTS | 409 | Already a member | org.ErrMemberExists |
| ERR_USER_NOT_FOUND | 404 | User not found | org.ErrUserNotFound |
| ERR_ORG_NOT_FOUND | 404 | Org not found | org.ErrOrgNotFound |
| ERR_ORG_INACTIVE | 403 | Org not active | org.ErrOrgInactive |
| ERR_POLICY_VERSION_CONFLICT | 409 | Concurrent policy save (unique violation, migration 13) | org.ErrPolicyVersionConflict |
| ERR_INVALID_POLICY | 400 | Bad policy tiers | approval.ErrBadTiers |
| ERR_NO_POLICY | 400 | No active policy matches the amount | approval.ErrNoPolicy |
| ERR_APPROVAL_NOT_FOUND | 404 | Approval request not found / no standing | approval.ErrNotFound |
| ERR_ALREADY_FINALIZED | 409 | Approval already finalized | approval.ErrAlreadyFinalized |
| ERR_ALREADY_ACTED | 409 | Checker already acted | approval.ErrAlreadyActed |
| ERR_WRONG_LEVEL | 403 | Checker level does not match current tier | approval.ErrWrongLevel |
| ERR_APPROVAL_EXPIRED | 410 | Approval expired | approval.ErrExpired |
| ERR_SELF_APPROVAL | 403 | Approver is the creator | approval.ErrSelfApproval |
| ERR_SELF_REQUEST | 400 | Money request to self | moneyrequest.ErrSelfRequest |
| ERR_REQUEST_NOT_FOUND | 404 | Money request not found | moneyrequest.ErrRequestNotFound |
| ERR_REQUEST_NOT_PAYABLE | 409 | Request not in a payable state | moneyrequest.ErrNotPayable |
| ERR_REQUEST_EXPIRED | 410 | Request expired | moneyrequest.ErrExpired |
| ERR_FORBIDDEN | 403 | Not allowed on this money request | moneyrequest.ErrForbidden |
| ERR_REMIND_TOO_SOON | 429 | Reminder cooldown | moneyrequest.ErrRemindTooSoon |
| ERR_IDEMPOTENCY_KEY_REUSED | 409 | The same user already used this `Idempotency-Key` for a different request (ADR-0011) | wallet.ErrIdempotencyKeyReused |
| ERR_KYC_ALREADY_PENDING | 409 | KYC in progress | kyc.ErrAlreadyPending |
| ERR_KYC_ALREADY_VERIFIED | 409 | KYC already verified | kyc.ErrAlreadyVerified |
| ERR_KYC_NOT_FOUND | 404 | KYC request not found | kyc.ErrKYCNotFound |
| ERR_ALREADY_REVIEWED | 409 | KYC already reviewed | kyc.ErrAlreadyReviewed |
| ERR_INVALID_SIGNATURE | 403 | Bad signed file link | storage.ErrInvalidSignature |
| ERR_LINK_EXPIRED | 410 | Signed link expired | storage.ErrLinkExpired |
| ERR_FILE_NOT_FOUND | 404 | File missing or bad key | storage.ErrNotFound, storage.ErrBadKey |
| ERR_FLAG_NOT_FOUND | 404 | Fraud flag not found | fraud.ErrFlagNotFound |
| ERR_ALREADY_RESOLVED | 409 | Fraud flag already resolved | fraud.ErrAlreadyResolved |
| ERR_TXN_NOT_REVERSIBLE | 409 | Only completed transactions reverse | reversal.ErrNotReversible |
| ERR_ALREADY_REVERSED | 409 | Already reversed | reversal.ErrAlreadyReversed |
| ERR_REVERSAL_PENDING | 409 | Reversal request already pending | reversal.ErrReversalPending |
| ERR_RECON_ALREADY_RAN | 409 | Recon already ran for the date | recon.ErrAlreadyRan |
| ERR_RECON_NOT_FOUND | 404 | Recon run not found | recon.ErrRunNotFound |
| ERR_CALLBACK_AMOUNT_MISMATCH | 422 | Mock bank callback amount differs | wallet.ErrCallbackAmountMismatch |
| ERR_QR_INVALID | 400 | QR or receiver token invalid | qr.ErrQRInvalid, qr.ErrTokenInvalid |
| ERR_QR_EXPIRED | 410 | QR or receiver token expired | qr.ErrQRExpired, qr.ErrTokenExpired |
| ERR_INTERNAL | 500 | Fallback for any unmapped error | respondErr |

## Codes emitted inline (not in errTable)

| Code | HTTP | Where |
|---|---|---|
| ERR_INVALID_TOKEN | 401 | `RequireAuth` (missing bearer), `admin_handler.go` (admin token) |
| ERR_RATE_LIMITED | 429 | `RateLimit`, `RateLimitUser` in middleware.go |
| ERR_IDEMPOTENCY_KEY_REQUIRED | 400 | `RequireIdempotencyKey` (missing or longer than 64 chars); also `errTable` via wallet.ErrIdempotencyKeyRequired when a service is called without a key |
| ERR_INVALID_CALLBACK_SECRET | 401 (per spec) | mockbank_handler.go |
| ERR_KEY_NOT_FOUND | 404 | admin_ai.go (setting references unknown AI key) |
| ERR_KEY_INACTIVE | 400 | admin_ai.go (setting references inactive key) |
| ERR_INSUFFICIENT_BALANCE, ERR_FRAUD_BLOCKED, ERR_ACCOUNT_INACTIVE, ERR_TRANSFER_FAILED | n/a (field value) | transfer/service.go:198-204: terminal `error_code` in the timeline response of a failed or compensated workflow; not HTTP errors |

## In the OpenAPI spec but not emitted by the code

`ERR_NOT_FOUND` (PUT /admin/ai/keys/{id}), `ERR_SERVICE_NOT_FOUND`, `ERR_OFFER_NOT_FOUND`, `ERR_WORKFLOW_NOT_FOUND`. No handler produces them; the unmapped not-found paths for those admin resources fall to the generic behavior (check `admin_handler.go`; `pgx.ErrNoRows` is handled specially only in `user_handler.go`).

## Domain errors with no entry in errTable (generic 500 ERR_INTERNAL)

Package-level sentinel errors defined under `internal/` that `respondErr` cannot match:

| Error | Package | Likely surface |
|---|---|---|
| ErrPasswordMismatch | auth (password.go) | Normally converted to ErrInvalidCredentials inside the service; verify before relying |
| ErrBlocked ("transaction blocked by fraud rules") | fraud | Raised in the worker handler; surfaces to clients as workflow error code `ERR_FRAUD_BLOCKED` in the timeline, not as HTTP |
| ErrUnbalanced, ErrInvalidLeg | ledger | Internal invariant violations; 500 is intended |
| ErrWalletNotFound | ledger | Distinct from `wallet.ErrWalletNotFound`; a ledger-level miss returns 500 |
| ErrBalanceDrift | recon | Reconciliation invariant failure; 500 on `POST /admin/recon/run` |
| ErrDefinitionNotFound | workflow | Missing/inactive workflow definition (for example an unknown service workflow_name); 500 |

Also unmapped: raw database errors (unique violation 23505 on `transactions.reversal_of` races or `idempotency_key` collisions, FK errors), uuid parse failures that handlers ignore, and the engine confirm/verify "transaction not found" string error (audit BE-C2) which is not an `errTable` match.

## Added by ADR-0014 (transaction authentication)

| Code | HTTP | Meaning | Raised in |
|------|------|---------|-----------|
| ERR_PIN_WEAK | 400 | PIN is not 6 digits, or all the same digit, or a sequence | uth.ErrWeakPIN |
| ERR_PIN_NOT_SET | 403 | the user has no PIN; set one first | uth.ErrPINNotSet |
| ERR_PIN_INCORRECT | 403 | wrong PIN (counts toward the lockout) | uth.ErrPINIncorrect |
| ERR_PIN_LOCKED | 429 | five wrong PINs; PIN use locked for 15 minutes | uth.ErrPINLocked |
| ERR_PIN_COOLING | 403 | a PIN transaction above 2,500,000 VND within 24 hours of a PIN reset or operator clear | auth.ErrPINCooling |
| ERR_PIN_RESET_RATE_LIMITED | 429 | more than 3 PIN reset codes requested in an hour | auth.ErrPINResetRateLimited |
| ERR_OTP_DELIVERY_UNAVAILABLE | 503 | no real code delivery channel is configured (production and demo until a provider is chosen) | auth.ErrSenderUnavailable |
| ERR_TOTP_NOT_ENROLLED | 403 | the 2fa band (or enrolment confirm) needs an authenticator that is not set up | auth.ErrTOTPNotEnrolled |
| ERR_TOTP_INVALID | 400 | wrong, expired, replayed or already used authenticator or recovery code | auth.ErrTOTPInvalid |
| ERR_TOTP_LOCKED | 429 | five wrong authenticator codes; TOTP use locked for 15 minutes | auth.ErrTOTPLocked |
| ERR_TOTP_ALREADY_ENABLED | 409 | enrolment would overwrite an active authenticator | auth.ErrTOTPAlreadyEnabled |
| ERR_AUTH_METHOD_UNAVAILABLE | 422 | the offer needs a method with no provider (2fa) | 	ransfer.ErrAuthMethodUnavailable |
| ERR_AUTH_EXPIRED | 410 | not confirmed within 10 minutes | 	ransfer.ErrAuthExpired |
| ERR_CREDENTIAL_REQUIRED | 400 | the method needs a credential the caller did not send | 	ransfer.ErrCredentialRequired |
| ERR_ALREADY_CONFIRMED | 409 | the transaction is not waiting for authentication | 	ransfer.ErrAlreadyConfirmed |
| ERR_AUTH_NOT_CONFIGURED | 503 | the service was built without an authenticator | 	ransfer.ErrAuthNotConfigured |
| ERR_TXN_NOT_FOUND | 404 | also returned by confirm for ANOTHER user's transaction | 	ransfer.ErrTxnNotFound |

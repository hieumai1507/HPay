package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/fraud"
	"github.com/hieumai1507/hpay-backend/internal/kyc"
	"github.com/hieumai1507/hpay-backend/internal/ledger"
	"github.com/hieumai1507/hpay-backend/internal/moneyrequest"
	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	"github.com/hieumai1507/hpay-backend/internal/org"
	"github.com/hieumai1507/hpay-backend/internal/qr"
	"github.com/hieumai1507/hpay-backend/internal/recon"
	"github.com/hieumai1507/hpay-backend/internal/reversal"
	"github.com/hieumai1507/hpay-backend/internal/storage"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/vault"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

// AppError is the single wire format for failures (openapi Error schema).
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

var errTable = []struct {
	match error
	out   AppError
}{
	{auth.ErrPhoneTaken, AppError{"ERR_PHONE_TAKEN", "phone already registered", http.StatusConflict}},
	{auth.ErrWeakPIN, AppError{"ERR_PIN_WEAK", "PIN must be 6 digits and not all the same digit or a sequence", http.StatusBadRequest}},
	{auth.ErrPINNotSet, AppError{"ERR_PIN_NOT_SET", "set a PIN first", http.StatusForbidden}},
	{auth.ErrPINIncorrect, AppError{"ERR_PIN_INCORRECT", "PIN incorrect", http.StatusForbidden}},
	{auth.ErrPINLocked, AppError{"ERR_PIN_LOCKED", "PIN locked after too many failed attempts; try again later", http.StatusTooManyRequests}},
	{transfer.ErrTxnNotFound, AppError{"ERR_TXN_NOT_FOUND", "transaction not found", http.StatusNotFound}},
	{transfer.ErrAuthMethodUnavailable, AppError{"ERR_AUTH_METHOD_UNAVAILABLE", "the required authentication method is not available yet", http.StatusUnprocessableEntity}},
	{transfer.ErrAuthExpired, AppError{"ERR_AUTH_EXPIRED", "the request expired before it was confirmed; start again", http.StatusGone}},
	{transfer.ErrCredentialRequired, AppError{"ERR_CREDENTIAL_REQUIRED", "a credential is required to confirm this transaction", http.StatusBadRequest}},
	{transfer.ErrAlreadyConfirmed, AppError{"ERR_ALREADY_CONFIRMED", "transaction is not waiting for authentication", http.StatusConflict}},
	{transfer.ErrAuthNotConfigured, AppError{"ERR_AUTH_NOT_CONFIGURED", "transaction authentication is not configured", http.StatusServiceUnavailable}},
	{auth.ErrInvalidCredentials, AppError{"ERR_INVALID_CREDENTIALS", "invalid phone or password", http.StatusUnauthorized}},
	{auth.ErrAccountLocked, AppError{"ERR_ACCOUNT_LOCKED", "account is locked", http.StatusForbidden}},
	{auth.ErrTokenReused, AppError{"ERR_TOKEN_REUSED", "refresh token reused; all sessions revoked", http.StatusUnauthorized}},
	{auth.ErrTokenExpired, AppError{"ERR_TOKEN_EXPIRED", "token expired", http.StatusUnauthorized}},
	{auth.ErrTokenInvalid, AppError{"ERR_INVALID_TOKEN", "token invalid", http.StatusUnauthorized}},
	{auth.ErrOTPCooldown, AppError{"ERR_OTP_COOLDOWN", "otp requested too soon", http.StatusTooManyRequests}},
	{auth.ErrOTPExpired, AppError{"ERR_OTP_EXPIRED", "otp expired or not found", http.StatusBadRequest}},
	{auth.ErrOTPMaxAttempts, AppError{"ERR_OTP_MAX_ATTEMPTS", "otp attempts exhausted", http.StatusBadRequest}},
	{auth.ErrOTPMismatch, AppError{"ERR_OTP_INVALID", "otp code invalid", http.StatusBadRequest}},
	{wallet.ErrAmountInvalid, AppError{"ERR_AMOUNT_INVALID", "amount must be positive", http.StatusBadRequest}},
	{wallet.ErrLimitExceeded, AppError{"ERR_LIMIT_EXCEEDED", "amount exceeds your kyc tier limit", http.StatusBadRequest}},
	{wallet.ErrWalletNotFound, AppError{"ERR_WALLET_NOT_FOUND", "wallet not found", http.StatusNotFound}},
	{wallet.ErrTxnNotFound, AppError{"ERR_TXN_NOT_FOUND", "transaction not found", http.StatusNotFound}},
	// C-03 / ADR-0011: per-user, request-bound idempotency.
	{wallet.ErrIdempotencyKeyReused, AppError{"ERR_IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different request", http.StatusConflict}},
	{wallet.ErrIdempotencyKeyRequired, AppError{"ERR_IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header (<=64 chars) is required", http.StatusBadRequest}},
	{transfer.ErrReceiverNotFound, AppError{"ERR_RECEIVER_NOT_FOUND", "receiver not found", http.StatusNotFound}},
	// Vaults (docs 04 §2.2) — a foreign vault id is reported as not-found so we
	// never confirm that somebody else's pocket exists.
	{vault.ErrNotFound, AppError{"ERR_VAULT_NOT_FOUND", "vault not found", http.StatusNotFound}},
	{vault.ErrLabelTaken, AppError{"ERR_VAULT_LABEL_TAKEN", "you already have a vault with this name", http.StatusConflict}},
	{vault.ErrLabelRequired, AppError{"ERR_VALIDATION", "vault name is required", http.StatusBadRequest}},
	{vault.ErrDirectionInvalid, AppError{"ERR_VAULT_DIRECTION_INVALID", "direction must be deposit or withdraw", http.StatusBadRequest}},
	{offerguard.ErrServiceNotInOffer, AppError{"ERR_SERVICE_NOT_IN_OFFER", "service not available in your offer", http.StatusForbidden}},
	{offerguard.ErrOfferInactive, AppError{"ERR_OFFER_INACTIVE", "your offer is not active", http.StatusForbidden}},
	{offerguard.ErrServiceDisabled, AppError{"ERR_SERVICE_DISABLED", "service disabled for your offer", http.StatusForbidden}},
	{offerguard.ErrBelowMin, AppError{"ERR_AMOUNT_BELOW_MIN", "amount below service minimum", http.StatusBadRequest}},
	{offerguard.ErrLimitPerTxn, AppError{"ERR_LIMIT_PER_TXN", "amount exceeds per-transaction limit", http.StatusBadRequest}},
	{offerguard.ErrLimitDaily, AppError{"ERR_LIMIT_DAILY", "daily limit exceeded", http.StatusBadRequest}},
	{offerguard.ErrLimitMonthly, AppError{"ERR_LIMIT_MONTHLY", "monthly limit exceeded", http.StatusBadRequest}},
	{offerguard.ErrTooFrequent, AppError{"ERR_TOO_FREQUENT", "transactions too frequent; try again shortly", http.StatusTooManyRequests}},
	{ledger.ErrInsufficientBalance, AppError{"ERR_INSUFFICIENT_BALANCE", "insufficient balance", http.StatusBadRequest}},
	{ledger.ErrSelfTransfer, AppError{"ERR_SELF_TRANSFER", "cannot transfer to the same wallet", http.StatusBadRequest}},
	// F6 — org & approval engine
	{org.ErrNotMember, AppError{"ERR_NOT_ORG_MEMBER", "not a member of this organization", http.StatusForbidden}},
	{org.ErrForbiddenRole, AppError{"ERR_FORBIDDEN_ROLE", "your org role does not allow this action", http.StatusForbidden}},
	{org.ErrMemberExists, AppError{"ERR_MEMBER_EXISTS", "user is already a member of this organization", http.StatusConflict}},
	{org.ErrUserNotFound, AppError{"ERR_USER_NOT_FOUND", "user not found", http.StatusNotFound}},
	{org.ErrOrgNotFound, AppError{"ERR_ORG_NOT_FOUND", "organization not found", http.StatusNotFound}},
	{org.ErrOrgInactive, AppError{"ERR_ORG_INACTIVE", "organization is not active", http.StatusForbidden}},
	{org.ErrReceiverNotFound, AppError{"ERR_RECEIVER_NOT_FOUND", "receiver not found", http.StatusNotFound}},
	{org.ErrBadRole, AppError{"ERR_VALIDATION", "invalid role or checker level", http.StatusBadRequest}},
	{org.ErrPolicyVersionConflict, AppError{"ERR_POLICY_VERSION_CONFLICT", "policy was changed concurrently — reload and try again", http.StatusConflict}},
	{approval.ErrBadTiers, AppError{"ERR_INVALID_POLICY", "invalid policy definition", http.StatusBadRequest}},
	{approval.ErrNoPolicy, AppError{"ERR_NO_POLICY", "no active approval policy matches this amount", http.StatusBadRequest}},
	{approval.ErrNotFound, AppError{"ERR_APPROVAL_NOT_FOUND", "approval request not found", http.StatusNotFound}},
	{approval.ErrAlreadyFinalized, AppError{"ERR_ALREADY_FINALIZED", "approval request already finalized", http.StatusConflict}},
	{approval.ErrAlreadyActed, AppError{"ERR_ALREADY_ACTED", "you already acted on this request", http.StatusConflict}},
	{approval.ErrWrongLevel, AppError{"ERR_WRONG_LEVEL", "your checker level does not match the current tier", http.StatusForbidden}},
	{approval.ErrNotChecker, AppError{"ERR_FORBIDDEN_ROLE", "your org role does not allow this action", http.StatusForbidden}},
	{approval.ErrExpired, AppError{"ERR_APPROVAL_EXPIRED", "approval request expired", http.StatusGone}},
	// F7 — money requests & split bill
	{moneyrequest.ErrSelfRequest, AppError{"ERR_SELF_REQUEST", "cannot request money from yourself", http.StatusBadRequest}},
	{moneyrequest.ErrPayerNotFound, AppError{"ERR_RECEIVER_NOT_FOUND", "payer not found", http.StatusNotFound}},
	{moneyrequest.ErrRequestNotFound, AppError{"ERR_REQUEST_NOT_FOUND", "money request not found", http.StatusNotFound}},
	{moneyrequest.ErrNotPayable, AppError{"ERR_REQUEST_NOT_PAYABLE", "money request is not payable", http.StatusConflict}},
	{moneyrequest.ErrExpired, AppError{"ERR_REQUEST_EXPIRED", "money request expired", http.StatusGone}},
	{moneyrequest.ErrForbidden, AppError{"ERR_FORBIDDEN", "not allowed to act on this money request", http.StatusForbidden}},
	{moneyrequest.ErrBadSplit, AppError{"ERR_VALIDATION", "invalid split amounts", http.StatusBadRequest}},
	{moneyrequest.ErrRemindTooSoon, AppError{"ERR_REMIND_TOO_SOON", "money request reminded too soon", http.StatusTooManyRequests}},
	// F8 — kyc & admin review
	{kyc.ErrAlreadyPending, AppError{"ERR_KYC_ALREADY_PENDING", "a kyc request is already in progress", http.StatusConflict}},
	{kyc.ErrAlreadyVerified, AppError{"ERR_KYC_ALREADY_VERIFIED", "kyc already verified", http.StatusConflict}},
	{kyc.ErrBadImage, AppError{"ERR_VALIDATION", "invalid kyc image: a real jpeg or png, 200 to 8000 pixels per side, max 5MB", http.StatusBadRequest}},
	{kyc.ErrKYCNotFound, AppError{"ERR_KYC_NOT_FOUND", "kyc request not found", http.StatusNotFound}},
	{kyc.ErrAlreadyReviewed, AppError{"ERR_ALREADY_REVIEWED", "kyc request already reviewed", http.StatusConflict}},
	{storage.ErrInvalidSignature, AppError{"ERR_INVALID_SIGNATURE", "invalid file signature", http.StatusForbidden}},
	{storage.ErrLinkExpired, AppError{"ERR_LINK_EXPIRED", "signed link expired", http.StatusGone}},
	{storage.ErrNotFound, AppError{"ERR_FILE_NOT_FOUND", "file not found", http.StatusNotFound}},
	{storage.ErrBadKey, AppError{"ERR_FILE_NOT_FOUND", "file not found", http.StatusNotFound}},
	// F9 — fraud, recon, reversal
	{fraud.ErrFlagNotFound, AppError{"ERR_FLAG_NOT_FOUND", "fraud flag not found", http.StatusNotFound}},
	{fraud.ErrAlreadyResolved, AppError{"ERR_ALREADY_RESOLVED", "fraud flag already resolved", http.StatusConflict}},
	{reversal.ErrTxnNotFound, AppError{"ERR_TXN_NOT_FOUND", "transaction not found", http.StatusNotFound}},
	{reversal.ErrNotReversible, AppError{"ERR_TXN_NOT_REVERSIBLE", "only completed transactions can be reversed", http.StatusConflict}},
	{reversal.ErrAlreadyReversed, AppError{"ERR_ALREADY_REVERSED", "transaction already reversed", http.StatusConflict}},
	{reversal.ErrReversalPending, AppError{"ERR_REVERSAL_PENDING", "a reversal request is already pending", http.StatusConflict}},
	{approval.ErrSelfApproval, AppError{"ERR_SELF_APPROVAL", "cannot approve your own request", http.StatusForbidden}},
	{recon.ErrAlreadyRan, AppError{"ERR_RECON_ALREADY_RAN", "reconciliation already ran for this date", http.StatusConflict}},
	{recon.ErrRunNotFound, AppError{"ERR_RECON_NOT_FOUND", "reconciliation run not found", http.StatusNotFound}},
	// F10 — mock bank callback
	{wallet.ErrCallbackAmountMismatch, AppError{"ERR_CALLBACK_AMOUNT_MISMATCH", "callback amount does not match transaction", http.StatusUnprocessableEntity}},
	{qr.ErrQRInvalid, AppError{"ERR_QR_INVALID", "invalid QR signature or payload", http.StatusBadRequest}},
	{qr.ErrQRExpired, AppError{"ERR_QR_EXPIRED", "QR code has expired", http.StatusGone}},
	{qr.ErrTokenInvalid, AppError{"ERR_QR_INVALID", "invalid receiver token", http.StatusBadRequest}},
	{qr.ErrTokenExpired, AppError{"ERR_QR_EXPIRED", "receiver token has expired", http.StatusGone}},
}

// respondErr maps domain errors to the wire contract; unknown errors are 500
// without leaking internals (fintech-review #9).
func respondErr(c *gin.Context, err error) {
	for _, e := range errTable {
		if errors.Is(err, e.match) {
			c.JSON(e.out.Status, gin.H{"code": e.out.Code, "message": e.out.Message, "request_id": c.GetString("request_id")})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": "ERR_INTERNAL", "message": "internal error", "request_id": c.GetString("request_id")})
}

func respondValidation(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, gin.H{"code": "ERR_VALIDATION", "message": msg, "request_id": c.GetString("request_id")})
}

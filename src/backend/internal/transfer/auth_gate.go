package transfer

// Authentication gate for the engine request/confirm flow (ADR-0008, ADR-0014).
//
// A request whose offer requires pin, otp or 2fa is created in status
// awaiting_auth WITHOUT starting the workflow. confirm verifies the credential
// for that method, then moves awaiting_auth -> processing with a guarded UPDATE
// and starts the workflow in the same database transaction, so a confirm that is
// repeated or concurrent starts it exactly once.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

var (
	// ErrTxnNotFound is returned for an unknown transaction id AND for one that
	// belongs to someone else, so the two cannot be told apart (constitution C-08).
	ErrTxnNotFound = errors.New("transaction not found")
	// ErrAuthMethodUnavailable: the offer requires an authentication method the
	// system cannot perform yet (2fa has no provider). Fail closed.
	ErrAuthMethodUnavailable = errors.New("the required authentication method is not available yet")
	// ErrAuthNotConfigured: the service was built without an authenticator.
	ErrAuthNotConfigured = errors.New("transaction authentication is not configured")
	// ErrAuthExpired: the request waited too long for confirmation.
	ErrAuthExpired = errors.New("the request expired before it was confirmed; start again")
	// ErrCredentialRequired: the method needs a credential the caller did not send.
	ErrCredentialRequired = errors.New("a credential is required to confirm this transaction")
)

// AwaitingAuthTTL is how long an unconfirmed request is kept.
const AwaitingAuthTTL = 10 * time.Minute

// Authenticator verifies the second factor for a signed-in user. auth.Service
// implements it; transfer never compares credentials itself.
type Authenticator interface {
	VerifyPIN(ctx context.Context, userID uuid.UUID, pin string) error
	VerifyOTP(ctx context.Context, userID uuid.UUID, purpose, code string) error
	RequestOTPCode(ctx context.Context, userID uuid.UUID, purpose string) (string, error)
	// CheckPINCooling refuses a large amount right after a PIN reset (ADR-0016).
	CheckPINCooling(ctx context.Context, userID uuid.UUID, amount int64) error
}

// SetAuthenticator wires the credential verifier. Without it any request that
// needs authentication is refused (fail closed).
func (s *Service) SetAuthenticator(a Authenticator) { s.authn = a }

// otpPurpose binds an OTP challenge to one transaction. purpose is VARCHAR(40):
// "txn:" + 32 hex characters = 36.
func otpPurpose(txnID uuid.UUID) string {
	h := make([]byte, 0, 36)
	h = append(h, "txn:"...)
	const hexd = "0123456789abcdef"
	for _, b := range txnID {
		h = append(h, hexd[b>>4], hexd[b&0x0f])
	}
	return string(h)
}

// needsAuth reports whether a method requires holding the transaction.
func needsAuth(method string) bool { return method == "pin" || method == "otp" || method == "2fa" }

// confirmCredential verifies the credential for the method recorded on the
// transaction at request time (transactions.auth_method), not a recomputed one,
// so a change to the offer between request and confirm cannot weaken it.
func (s *Service) confirmCredential(ctx context.Context, userID, txnID uuid.UUID, method, pin, otp string) error {
	if s.authn == nil {
		return ErrAuthNotConfigured
	}
	switch method {
	case "pin":
		return s.authn.VerifyPIN(ctx, userID, pin)
	case "otp":
		// An OTP-band transaction needs BOTH the PIN and the OTP bound to this transaction.
		if err := s.authn.VerifyPIN(ctx, userID, pin); err != nil {
			return err
		}
		if otp == "" {
			return ErrCredentialRequired
		}
		return s.authn.VerifyOTP(ctx, userID, otpPurpose(txnID), otp)
	default: // 2fa and anything unknown
		return ErrAuthMethodUnavailable
	}
}

// ExpireAwaitingAuth fails requests that nobody confirmed within ttl. It moves no
// money: the ledger is untouched before MOVE_FUNDS. Returns how many it expired.
func (s *Service) ExpireAwaitingAuth(ctx context.Context, ttl time.Duration) (int, error) {
	ids, err := s.q.ExpireAwaitingAuth(ctx, pgtype.Timestamptz{Time: time.Now().Add(-ttl), Valid: true})
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

// ExecuteEngineChallenge issues an OTP bound to one awaiting_auth transaction.
// The code is returned to the caller only so the handler can expose it in dev;
// other environments must deliver it out of band.
func (s *Service) ExecuteEngineChallenge(ctx context.Context, userID, txnID uuid.UUID) (string, error) {
	if s.authn == nil {
		return "", ErrAuthNotConfigured
	}
	txn, err := s.q.GetOwnedTransactionForUpdate(ctx, store.GetOwnedTransactionForUpdateParams{ID: txnID, InitiatedBy: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrTxnNotFound
		}
		return "", err
	}
	if txn.Status != store.TxnStatusAwaitingAuth {
		return "", ErrAlreadyConfirmed
	}
	return s.authn.RequestOTPCode(ctx, userID, otpPurpose(txnID))
}

// ErrAlreadyConfirmed: the transaction is no longer waiting for authentication.
var ErrAlreadyConfirmed = errors.New("transaction is not waiting for authentication")

package auth

// Forgotten PIN reset (ADR-0016, spec w4-pin-reset).
//
// Self-service: the signed-in user proves the account password, receives a one-time code
// through the Sender port and sets a new PIN with it. The code is bound to the user and the
// purpose, single-use, limited to 3 attempts, 120 s, 30 s between requests and 3 requests per
// hour. A completed reset clears the PIN attempt counter and lock and starts a 24 hour cooling
// period that caps the size of PIN transactions (CheckPINCooling).
// Operator fallback: AdminService.ClearPIN (admin.go) removes the PIN after out-of-band identity verification.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

var (
	// ErrPINResetRateLimited: too many reset codes requested within the hour.
	ErrPINResetRateLimited = errors.New("too many PIN reset requests; try again later")
	// ErrPINCooling: a large transaction right after a PIN reset.
	ErrPINCooling = errors.New("transaction above the limit that applies right after a PIN reset")
	// ErrReasonRequired: an operator action without a reason.
	ErrReasonRequired = errors.New("a reason is required")
)

const (
	// PINResetMaxPerHour caps reset codes per user per hour.
	PINResetMaxPerHour = 3
	// PINCoolingPeriod is how long the cap below applies after a reset.
	PINCoolingPeriod = 24 * time.Hour
	// PINCoolingCap is the largest PIN transaction (VND) allowed during the cooling period.
	PINCoolingCap int64 = 2_500_000
)

// SetSender replaces the delivery channel (tests and wiring). The default is chosen from the
// environment in NewService.
func (s *Service) SetSender(sender Sender) { s.sender = sender }

// ResetRequestResult is what the request step returns to the caller.
type ResetRequestResult struct {
	ExpiresIn int    // seconds
	DevCode   string // only when the environment is dev; never in other environments
}

// RequestPINReset starts a reset: password step-up, limits, then a code through the Sender.
func (s *Service) RequestPINReset(ctx context.Context, userID uuid.UUID, password string) (ResetRequestResult, error) {
	st, err := s.q.GetUserPINState(ctx, userID)
	if err != nil {
		return ResetRequestResult{}, err
	}
	if VerifyPassword(st.PasswordHash, password) != nil {
		s.auditPIN(ctx, userID, "pin.reset_refused", "wrong password")
		return ResetRequestResult{}, ErrInvalidCredentials
	}
	if !st.PinHash.Valid || st.PinHash.String == "" {
		return ResetRequestResult{}, ErrPINNotSet
	}
	n, err := s.q.CountRecentOTPs(ctx, store.CountRecentOTPsParams{
		UserID: userID, Purpose: PurposePINReset, CreatedAt: ts(time.Now().Add(-time.Hour)),
	})
	if err != nil {
		return ResetRequestResult{}, err
	}
	if n >= PINResetMaxPerHour {
		s.auditPIN(ctx, userID, "pin.reset_refused", "hourly request cap reached")
		return ResetRequestResult{}, ErrPINResetRateLimited
	}
	code, err := s.RequestOTPCode(ctx, userID, PurposePINReset) // applies the 30 s cooldown
	if err != nil {
		return ResetRequestResult{}, err
	}
	if err := s.sender.SendCode(ctx, userID, PurposePINReset, code); err != nil {
		return ResetRequestResult{}, err
	}
	s.auditPIN(ctx, userID, "pin.reset_requested", "")
	out := ResetRequestResult{ExpiresIn: int(OTPTTL.Seconds())}
	if s.env == "dev" {
		out.DevCode = code
	}
	return out, nil
}

// ConfirmPINReset checks the code and sets the new PIN. A weak PIN is refused BEFORE the code
// is looked at, so a typo does not burn the code. The code is consumed in the same
// transaction that replaces the PIN, with a guarded UPDATE so that exactly one of several
// parallel confirms can win.
func (s *Service) ConfirmPINReset(ctx context.Context, userID uuid.UUID, code, newPIN string) error {
	if err := ValidatePIN(newPIN); err != nil {
		return err
	}
	if err := s.checkResetCode(ctx, userID, code); err != nil {
		return err
	}
	hash, err := HashPassword(newPIN)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	ch, err := q.GetActiveOTP(ctx, store.GetActiveOTPParams{UserID: userID, Purpose: PurposePINReset})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOTPExpired
		}
		return err
	}
	won, err := q.ConsumeOTPIfActive(ctx, ch.ID)
	if err != nil {
		return err
	}
	if won == 0 {
		return ErrOTPExpired // another confirm consumed it first
	}
	if _, err := q.ResetUserPIN(ctx, store.ResetUserPINParams{ID: userID, PinHash: pgtype.Text{String: hash, Valid: true}}); err != nil {
		return err
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: userID, ActorType: "user", Action: "pin.reset", Resource: "user " + userID.String(),
		Reason: pgtype.Text{String: "PIN reset with a one-time code", Valid: true},
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.notifyReset(ctx, userID, "Your transaction PIN was reset. If this was not you, contact support now.")
	return nil
}

// checkResetCode counts the attempt and compares the code. It does not consume it.
func (s *Service) checkResetCode(ctx context.Context, userID uuid.UUID, code string) error {
	ch, err := s.q.GetActiveOTP(ctx, store.GetActiveOTPParams{UserID: userID, Purpose: PurposePINReset})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOTPExpired
		}
		return err
	}
	inc, err := s.q.IncrementOTPAttempts(ctx, ch.ID)
	if err != nil {
		return err
	}
	key := userID.String() + ":" + PurposePINReset
	return CheckOTP(OTPChallenge{
		ID: key, CodeHash: ch.CodeHash, MaxAttempts: int(ch.MaxAttempts), ExpiresAt: ch.ExpiresAt.Time,
	}, code, int(inc.Attempts), time.Now())
}

// CheckPINCooling refuses a PIN transaction above PINCoolingCap during the cooling period
// that follows a reset.
func (s *Service) CheckPINCooling(ctx context.Context, userID uuid.UUID, amount int64) error {
	if amount <= PINCoolingCap {
		return nil
	}
	at, err := s.q.GetPINResetAt(ctx, userID)
	if err != nil {
		return err
	}
	if at.Valid && time.Since(at.Time) < PINCoolingPeriod {
		return ErrPINCooling
	}
	return nil
}

// notifyReset tells the user a reset happened. A delivery failure is logged and audited but
// does not undo the reset (the audit row and the cooling period are the safeguards).
func (s *Service) notifyReset(ctx context.Context, userID uuid.UUID, text string) {
	if err := s.sender.Notify(ctx, userID, Notice{Kind: NoticePINReset, Text: text}); err != nil {
		s.log.Warn("pin reset notice not delivered", "user", userID, "err", err)
		s.auditPIN(ctx, userID, "pin.reset_notice_failed", fmt.Sprintf("%v", err))
	}
}

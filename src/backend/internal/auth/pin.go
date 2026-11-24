package auth

// Server-side transaction PIN (ADR-0008, ADR-0014).
//
// The PIN is a second factor for money movement, separate from the login
// password. It is stored only as an argon2id hash (the same function as the
// password), every failure is counted per user with one atomic SQL update, five
// consecutive failures lock PIN use for 15 minutes (a correct PIN is refused
// while locked), and failures and locks are audited without the secret.

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
	ErrWeakPIN      = errors.New("PIN must be 6 digits and not all the same digit or a sequence")
	ErrPINNotSet    = errors.New("no PIN set; set a PIN first")
	ErrPINIncorrect = errors.New("PIN incorrect")
	ErrPINLocked    = errors.New("PIN locked after too many failed attempts; try again later")
)

// pinMaxFailures is the number of consecutive failures that locks PIN use.
// Keep in step with queries/auth.sql RecordPINFailure.
const pinMaxFailures = 5

// ValidatePIN enforces the format: exactly 6 digits, not all the same digit and
// not an ascending or descending run.
func ValidatePIN(pin string) error {
	if len(pin) != 6 {
		return ErrWeakPIN
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return ErrWeakPIN
		}
	}
	same, asc, desc := true, true, true
	for i := 1; i < len(pin); i++ {
		if pin[i] != pin[0] {
			same = false
		}
		if pin[i] != pin[i-1]+1 {
			asc = false
		}
		if pin[i] != pin[i-1]-1 {
			desc = false
		}
	}
	if same || asc || desc {
		return ErrWeakPIN
	}
	return nil
}

// SetPIN sets the first PIN. It requires the account password (step-up). If a
// PIN already exists use ChangePIN.
func (s *Service) SetPIN(ctx context.Context, userID uuid.UUID, password, newPIN string) error {
	st, err := s.q.GetUserPINState(ctx, userID)
	if err != nil {
		return err
	}
	if VerifyPassword(st.PasswordHash, password) != nil {
		s.auditPIN(ctx, userID, "pin.set_refused", "wrong password")
		return ErrInvalidCredentials
	}
	if err := ValidatePIN(newPIN); err != nil {
		return err
	}
	return s.storePIN(ctx, userID, newPIN, "pin.set")
}

// ChangePIN replaces the PIN. It requires the CURRENT PIN, which goes through
// the same counter and lockout as a payment confirmation.
func (s *Service) ChangePIN(ctx context.Context, userID uuid.UUID, currentPIN, newPIN string) error {
	if err := ValidatePIN(newPIN); err != nil {
		return err
	}
	// no rehash here: storePIN writes a fresh hash right after
	if err := s.verifyPIN(ctx, userID, currentPIN, false); err != nil {
		return err
	}
	return s.storePIN(ctx, userID, newPIN, "pin.changed")
}

func (s *Service) storePIN(ctx context.Context, userID uuid.UUID, pin, action string) error {
	h, err := s.pins.HashPIN(pin)
	if err != nil {
		return err
	}
	if err := s.q.SetUserPIN(ctx, store.SetUserPINParams{ID: userID, PinHash: pgtype.Text{String: h, Valid: true}}); err != nil {
		return err
	}
	s.auditPIN(ctx, userID, action, "")
	return nil
}

// VerifyPIN checks the PIN for a signed-in user. It is the only function that
// compares a PIN; callers must not read pin_hash themselves.
func (s *Service) VerifyPIN(ctx context.Context, userID uuid.UUID, pin string) error {
	return s.verifyPIN(ctx, userID, pin, true)
}

// verifyPIN is VerifyPIN; rehash says whether a successful check may upgrade the stored hash.
func (s *Service) verifyPIN(ctx context.Context, userID uuid.UUID, pin string, rehash bool) error {
	if s.pins == nil {
		return ErrPepperNotConfigured // fail closed: never verify a PIN without the pepper set
	}
	st, err := s.q.GetUserPINState(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			s.pins.BurnWork() // same cost as a real check: no timing oracle for unknown users
			return ErrPINNotSet
		}
		return err
	}
	if !st.PinHash.Valid || st.PinHash.String == "" {
		s.pins.BurnWork()
		return ErrPINNotSet
	}
	if st.PinLockedUntil.Valid && st.PinLockedUntil.Time.After(time.Now()) {
		return ErrPINLocked // refused even for the correct PIN
	}
	if verr := s.pins.VerifyPINHash(st.PinHash.String, pin); verr != nil {
		if errors.Is(verr, ErrUnknownPepperID) || errors.Is(verr, errMalformedHash) {
			// The client sees an ordinary wrong PIN. The log names the key id only.
			s.log.Warn("PIN hash is unusable", "user", userID, "reason", verr.Error())
		}
		res, err := s.q.RecordPINFailure(ctx, userID)
		if err != nil {
			return err
		}
		if res.PinLockedUntil.Valid && res.PinLockedUntil.Time.After(time.Now()) {
			s.auditPIN(ctx, userID, "pin.locked", fmt.Sprintf("locked after %d failed attempts", res.PinFailedAttempts))
			return ErrPINLocked
		}
		s.auditPIN(ctx, userID, "pin.failed", fmt.Sprintf("failed attempt %d of %d", res.PinFailedAttempts, pinMaxFailures))
		return ErrPINIncorrect
	}
	if st.PinFailedAttempts != 0 || st.PinLockedUntil.Valid {
		if err := s.q.ResetPINFailures(ctx, userID); err != nil {
			return err
		}
	}
	if rehash && s.pins.NeedsRehash(st.PinHash.String) {
		s.upgradePINHash(ctx, userID, st.PinHash.String, pin)
	}
	return nil
}

// upgradePINHash moves a verified legacy, old-key or old-parameter hash to the current format
// (ADR-0018). A failure is logged and ignored: the PIN was right, so the payment goes on.
func (s *Service) upgradePINHash(ctx context.Context, userID uuid.UUID, oldHash, pin string) {
	wrote, err := s.rehashPIN(ctx, userID, oldHash, pin)
	if err != nil {
		s.log.Warn("PIN hash upgrade failed", "user", userID)
		return
	}
	if wrote {
		s.log.Debug("PIN hash upgraded", "user", userID, "key", s.pins.CurrentID())
	}
}

// auditPIN writes a trail entry that never contains the PIN or a hash.
func (s *Service) auditPIN(ctx context.Context, userID uuid.UUID, action, reason string) {
	_ = s.q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: userID, ActorType: "user", Action: action,
		Resource: "user " + userID.String(),
		Reason:   pgtype.Text{String: reason, Valid: reason != ""},
	})
}

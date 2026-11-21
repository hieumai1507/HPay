package auth

// TOTP second factor for the 2fa band (ADR-0017, spec w4-2fa).
//
// RFC 6238 (HMAC-SHA1, 6 digits, 30 s step), the algorithm authenticator apps implement.
// The shared secret is stored AES-256-GCM encrypted (key from TOTP_ENCRYPTION_KEY), shown once
// at enrolment and never returned or logged. A code is accepted once: the time step is claimed
// with a guarded UPDATE, so a replay or a parallel use of the same code fails. Five wrong
// codes in a row lock TOTP use for 15 minutes. Ten single-use recovery codes (stored as
// SHA-256 hashes) cover a lost authenticator.

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

var (
	// ErrTOTPNotEnrolled: the user has no active TOTP; enrol first.
	ErrTOTPNotEnrolled = errors.New("no authenticator is set up; enrol one first")
	// ErrTOTPInvalid: wrong, expired, replayed or already used code.
	ErrTOTPInvalid = errors.New("authenticator code is not valid")
	// ErrTOTPLocked: five wrong codes in a row; TOTP use is locked for 15 minutes.
	ErrTOTPLocked = errors.New("too many wrong authenticator codes; try again later")
	// ErrTOTPAlreadyEnabled: enrolment would overwrite an active authenticator.
	ErrTOTPAlreadyEnabled = errors.New("an authenticator is already active; ask support to remove it first")
	// ErrTOTPKeyMissing: the service has no encryption key (a wiring mistake): fail closed.
	ErrTOTPKeyMissing = errors.New("authenticator encryption key is not configured")
)

const (
	totpStep        = 30 // seconds
	totpDigits      = 6
	totpDrift       = 1 // accepted steps either side
	totpMaxFailures = 5
	recoveryCount   = 10
	totpIssuer      = "HPay"
)

// totpCode computes the 6-digit code of an RFC 4226 counter (RFC 6238: counter = unix / 30).
func totpCode(secret []byte, counter int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter))
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := (uint32(sum[off])&0x7f)<<24 | uint32(sum[off+1])<<16 | uint32(sum[off+2])<<8 | uint32(sum[off+3])
	mod := uint32(1)
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, bin%mod)
}

// TOTPCode is the exported form for tests in other packages.
func TOTPCode(secret []byte, counter int64) string { return totpCode(secret, counter) }

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func encodeSecret(secret []byte) string { return b32.EncodeToString(secret) }

func decodeSecret(s string) ([]byte, error) {
	return b32.DecodeString(strings.ToUpper(strings.ReplaceAll(s, " ", "")))
}

// DecodeTOTPSecret decodes the base32 secret shown at enrolment (tests and tooling).
func DecodeTOTPSecret(s string) ([]byte, error) { return decodeSecret(s) }

func encryptSecret(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, gcm.Seal(nil, nonce, plain, nil)...), nil
}

func decryptSecret(key, enc []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(enc) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, enc[:gcm.NonceSize()], enc[gcm.NonceSize():], nil)
}

// SetTOTPKey sets the AES-256 key that protects secrets at rest. It must be 32 bytes.
func (s *Service) SetTOTPKey(key []byte) { s.totpKey = key }

func (s *Service) totpKeyOK() error {
	if len(s.totpKey) != 32 {
		return ErrTOTPKeyMissing
	}
	return nil
}

// EnrollResult is shown to the user ONCE.
type EnrollResult struct {
	Secret string // base32, for manual entry
	URI    string // otpauth:// URI for a QR code
}

// EnrollTOTP starts enrolment. It needs the account password and the user must have a PIN
// (the 2fa band needs both). The secret is not active until a first code is confirmed.
func (s *Service) EnrollTOTP(ctx context.Context, userID uuid.UUID, password string) (EnrollResult, error) {
	if err := s.totpKeyOK(); err != nil {
		return EnrollResult{}, err
	}
	st, err := s.q.GetUserPINState(ctx, userID)
	if err != nil {
		return EnrollResult{}, err
	}
	if VerifyPassword(st.PasswordHash, password) != nil {
		s.auditTOTP(ctx, userID, "totp.enroll_refused", "wrong password")
		return EnrollResult{}, ErrInvalidCredentials
	}
	if !st.PinHash.Valid || st.PinHash.String == "" {
		return EnrollResult{}, ErrPINNotSet
	}
	if cur, err := s.q.GetUserTOTP(ctx, userID); err == nil && cur.Enabled {
		return EnrollResult{}, ErrTOTPAlreadyEnabled
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return EnrollResult{}, err
	}
	secret := make([]byte, 20) // 160 bits, the RFC 4226 recommendation
	if _, err := rand.Read(secret); err != nil {
		return EnrollResult{}, err
	}
	enc, err := encryptSecret(s.totpKey, secret)
	if err != nil {
		return EnrollResult{}, err
	}
	if err := s.q.UpsertPendingTOTP(ctx, store.UpsertPendingTOTPParams{UserID: userID, SecretEnc: enc}); err != nil {
		return EnrollResult{}, err
	}
	phone, _ := s.q.GetUserPhone(ctx, userID)
	label := url.PathEscape(totpIssuer + ":" + phone)
	enc32 := encodeSecret(secret)
	uri := fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=%d&period=%d",
		label, enc32, url.QueryEscape(totpIssuer), totpDigits, totpStep)
	s.auditTOTP(ctx, userID, "totp.enroll_started", "")
	return EnrollResult{Secret: enc32, URI: uri}, nil
}

// ConfirmResult carries the recovery codes, shown once.
type ConfirmResult struct{ RecoveryCodes []string }

// ConfirmTOTP activates a pending enrolment with a first valid code and issues recovery codes.
func (s *Service) ConfirmTOTP(ctx context.Context, userID uuid.UUID, code string) (ConfirmResult, error) {
	cur, err := s.loadTOTP(ctx, userID, false)
	if err != nil {
		return ConfirmResult{}, err
	}
	if cur.Enabled {
		return ConfirmResult{}, ErrTOTPAlreadyEnabled
	}
	if err := s.checkTOTPCode(ctx, userID, cur, code, false); err != nil {
		return ConfirmResult{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ConfirmResult{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.ActivateTOTP(ctx, userID); err != nil {
		return ConfirmResult{}, err
	}
	if err := q.DeleteRecoveryCodes(ctx, userID); err != nil {
		return ConfirmResult{}, err
	}
	codes := make([]string, 0, recoveryCount)
	for i := 0; i < recoveryCount; i++ {
		c, err := newRecoveryCode()
		if err != nil {
			return ConfirmResult{}, err
		}
		if err := q.InsertRecoveryCode(ctx, store.InsertRecoveryCodeParams{UserID: userID, CodeHash: hashRecovery(c)}); err != nil {
			return ConfirmResult{}, err
		}
		codes = append(codes, c)
	}
	if err := tx.Commit(ctx); err != nil {
		return ConfirmResult{}, err
	}
	s.auditTOTP(ctx, userID, "totp.enabled", "")
	s.notifyTOTP(ctx, userID, "An authenticator was added to your account. If this was not you, contact support now.")
	return ConfirmResult{RecoveryCodes: codes}, nil
}

// TOTPStatus is what a signed-in user may read: never the secret.
type TOTPStatusResult struct {
	Enabled           bool
	RecoveryCodesLeft int
}

func (s *Service) TOTPStatus(ctx context.Context, userID uuid.UUID) (TOTPStatusResult, error) {
	cur, err := s.q.GetUserTOTP(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TOTPStatusResult{}, nil
		}
		return TOTPStatusResult{}, err
	}
	if !cur.Enabled {
		return TOTPStatusResult{}, nil
	}
	n, err := s.q.CountRecoveryCodesLeft(ctx, userID)
	if err != nil {
		return TOTPStatusResult{}, err
	}
	return TOTPStatusResult{Enabled: true, RecoveryCodesLeft: int(n)}, nil
}

// HasActiveTOTP reports whether the user can use the 2fa band.
func (s *Service) HasActiveTOTP(ctx context.Context, userID uuid.UUID) (bool, error) {
	st, err := s.TOTPStatus(ctx, userID)
	return st.Enabled, err
}

// VerifyTOTP checks a TOTP code or a recovery code for an ACTIVE authenticator. A code is
// accepted once. It is the credential check of the 2fa band.
func (s *Service) VerifyTOTP(ctx context.Context, userID uuid.UUID, code string) error {
	cur, err := s.loadTOTP(ctx, userID, true)
	if err != nil {
		return err
	}
	return s.checkTOTPCode(ctx, userID, cur, code, true)
}

func (s *Service) loadTOTP(ctx context.Context, userID uuid.UUID, mustBeActive bool) (store.GetUserTOTPRow, error) {
	if err := s.totpKeyOK(); err != nil {
		return store.GetUserTOTPRow{}, err
	}
	cur, err := s.q.GetUserTOTP(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.GetUserTOTPRow{}, ErrTOTPNotEnrolled
		}
		return store.GetUserTOTPRow{}, err
	}
	if mustBeActive && !cur.Enabled {
		return store.GetUserTOTPRow{}, ErrTOTPNotEnrolled
	}
	return cur, nil
}

// checkTOTPCode compares a candidate with the codes of the accepted window, claims the time
// step (so it cannot be used again) and records failures. allowRecovery lets a recovery code
// stand in (confirm of a pending enrolment does not accept one).
func (s *Service) checkTOTPCode(ctx context.Context, userID uuid.UUID, cur store.GetUserTOTPRow, code string, allowRecovery bool) error {
	if cur.LockedUntil.Valid && cur.LockedUntil.Time.After(time.Now()) {
		return ErrTOTPLocked
	}
	code = strings.TrimSpace(code)
	secret, err := decryptSecret(s.totpKey, cur.SecretEnc)
	if err != nil {
		return err
	}
	now := time.Now().Unix() / totpStep
	if len(code) == totpDigits && isDigits(code) {
		for off := int64(-totpDrift); off <= totpDrift; off++ {
			step := now + off
			if step <= cur.LastStep {
				continue // already used (or older than the last used step)
			}
			if hmac.Equal([]byte(totpCode(secret, step)), []byte(code)) {
				won, err := s.q.ClaimTOTPStep(ctx, store.ClaimTOTPStepParams{UserID: userID, LastStep: step})
				if err != nil {
					return err
				}
				if won == 1 {
					return nil
				}
				break // a parallel use of the same code claimed the step first
			}
		}
	} else if allowRecovery && code != "" {
		n, err := s.q.UseRecoveryCode(ctx, store.UseRecoveryCodeParams{UserID: userID, CodeHash: hashRecovery(code)})
		if err != nil {
			return err
		}
		if n == 1 {
			s.auditTOTP(ctx, userID, "totp.recovery_used", "")
			return nil
		}
	}
	res, err := s.q.RecordTOTPFailure(ctx, userID)
	if err != nil {
		return err
	}
	if res.LockedUntil.Valid && res.LockedUntil.Time.After(time.Now()) {
		s.auditTOTP(ctx, userID, "totp.locked", fmt.Sprintf("locked after %d failed attempts", res.FailedAttempts))
		return ErrTOTPLocked
	}
	s.auditTOTP(ctx, userID, "totp.failed", fmt.Sprintf("failed attempt %d of %d", res.FailedAttempts, totpMaxFailures))
	return ErrTOTPInvalid
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// newRecoveryCode returns a code like "k3f9-a82d-77qz" (random, 60 bits).
func newRecoveryCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	h := hex.EncodeToString(b)[:12]
	return h[0:4] + "-" + h[4:8] + "-" + h[8:12], nil
}

func hashRecovery(code string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}

func (s *Service) auditTOTP(ctx context.Context, userID uuid.UUID, action, reason string) {
	s.auditPIN(ctx, userID, action, reason) // same trail shape; never carries a code or a secret
}

func (s *Service) notifyTOTP(ctx context.Context, userID uuid.UUID, text string) {
	if err := s.sender.Notify(ctx, userID, Notice{Kind: NoticeTOTP, Text: text}); err != nil {
		s.log.Warn("totp notice not delivered", "user", userID, "err", err)
	}
}

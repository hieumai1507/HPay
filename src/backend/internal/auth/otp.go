package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"
)

const (
	OTPTTL         = 120 * time.Second
	OTPMaxAttempts = 3
	OTPCooldown    = 30 * time.Second
)

var (
	ErrOTPExpired     = errors.New("otp expired")
	ErrOTPMaxAttempts = errors.New("otp max attempts reached")
	ErrOTPMismatch    = errors.New("otp mismatch")
	ErrOTPCooldown    = errors.New("otp requested too soon")
)

// GenerateOTPCode returns a 6-digit code using crypto/rand.
func GenerateOTPCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashOTP binds the code to its challenge id so a hash can't be replayed
// across challenges.
func HashOTP(challengeID, code string) string {
	sum := sha256.Sum256([]byte(challengeID + ":" + code))
	return hex.EncodeToString(sum[:])
}

// OTPChallenge is the pure view of a stored challenge used by CheckOTP.
type OTPChallenge struct {
	ID          string
	CodeHash    string
	Attempts    int
	MaxAttempts int
	ExpiresAt   time.Time
}

// CheckOTP validates a candidate code. attempts is the value AFTER the current
// try has been counted (callers increment first — E11: the 3rd wrong attempt
// permanently exhausts the challenge, a later correct code must NOT pass).
func CheckOTP(ch OTPChallenge, code string, attempts int, now time.Time) error {
	if now.After(ch.ExpiresAt) {
		return ErrOTPExpired
	}
	if attempts > ch.MaxAttempts {
		return ErrOTPMaxAttempts
	}
	want := []byte(ch.CodeHash)
	got := []byte(HashOTP(ch.ID, code))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		if attempts >= ch.MaxAttempts {
			return ErrOTPMaxAttempts
		}
		return ErrOTPMismatch
	}
	if attempts > ch.MaxAttempts {
		return ErrOTPMaxAttempts
	}
	return nil
}

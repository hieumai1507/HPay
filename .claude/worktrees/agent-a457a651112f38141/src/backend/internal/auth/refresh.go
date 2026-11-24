package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

const RefreshTokenTTL = 30 * 24 * time.Hour

// NewRefreshToken returns the opaque token (given to the client once) and its
// SHA-256 hex hash (the only thing we store — fintech-review #7).
func NewRefreshToken() (token string, hash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashRefreshToken(token), nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RotationDecision classifies a stored refresh token during /auth/refresh.
type RotationDecision int

const (
	RotationOK RotationDecision = iota
	// RotationReused means used_at was already set: replay implies theft,
	// the caller must revoke the whole token family.
	RotationReused
	RotationExpired
	RotationRevoked
)

// DecideRotation is pure so the theft-detection matrix is unit-testable.
func DecideRotation(usedAt, revokedAt *time.Time, expiresAt time.Time, now time.Time) RotationDecision {
	switch {
	case revokedAt != nil:
		return RotationRevoked
	case usedAt != nil:
		return RotationReused
	case now.After(expiresAt):
		return RotationExpired
	default:
		return RotationOK
	}
}

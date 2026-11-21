package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const AccessTokenTTL = 15 * time.Minute

var (
	ErrTokenInvalid = errors.New("token invalid")
	ErrTokenExpired = errors.New("token expired")
)

// Claims carried by access tokens. Audience separates user vs admin surfaces.
type Claims struct {
	DeviceID string `json:"dev"`
	jwt.RegisteredClaims
}

type JWTIssuer struct {
	secret   []byte
	audience string
}

func NewJWTIssuer(secret, audience string) *JWTIssuer {
	return &JWTIssuer{secret: []byte(secret), audience: audience}
}

func (j *JWTIssuer) Issue(userID uuid.UUID, deviceID string, now time.Time) (string, error) {
	claims := Claims{
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{j.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

// Parse validates signature, expiry, and audience; returns user id + device.
func (j *JWTIssuer) Parse(token string) (uuid.UUID, string, error) {
	var claims Claims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		return j.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithAudience(j.audience))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return uuid.Nil, "", ErrTokenExpired
		}
		return uuid.Nil, "", ErrTokenInvalid
	}
	if !parsed.Valid {
		return uuid.Nil, "", ErrTokenInvalid
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, "", ErrTokenInvalid
	}
	return uid, claims.DeviceID, nil
}

package qr

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrQRInvalid    = errors.New("QR code is invalid")
	ErrQRExpired    = errors.New("QR code is expired")
	ErrTokenInvalid = errors.New("receiver token invalid")
	ErrTokenExpired = errors.New("receiver token expired")
)

type MsgSigner interface {
	SignMsg(msg string, exp int64) string
	VerifyMsg(msg string, exp int64, sig string) bool
}

type Service struct {
	signer MsgSigner
}

func NewService(signer MsgSigner) *Service {
	return &Service{signer: signer}
}

type QRPayload struct {
	UserID      uuid.UUID `json:"user_id"`
	NameMasked  string    `json:"name_masked"`
	PhoneMasked string    `json:"phone_masked"`
	Exp         int64     `json:"exp"`
}

type QRResult struct {
	Payload   string    `json:"payload"`
	Signature string    `json:"signature"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ResolveResult struct {
	NameMasked    string    `json:"name_masked"`
	PhoneMasked   string    `json:"phone_masked"`
	ReceiverToken string    `json:"receiver_token"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type ReceiverTokenPayload struct {
	UserID  uuid.UUID `json:"user_id"`
	Exp     int64     `json:"exp"`
	Purpose string    `json:"purpose"`
}

func (s *Service) Mint(userID uuid.UUID, nameMasked, phoneMasked string) (QRResult, error) {
	expTime := time.Now().Add(15 * time.Minute)
	exp := expTime.Unix()

	payloadObj := QRPayload{
		UserID:      userID,
		NameMasked:  nameMasked,
		PhoneMasked: phoneMasked,
		Exp:         exp,
	}

	payloadBytes, err := json.Marshal(payloadObj)
	if err != nil {
		return QRResult{}, fmt.Errorf("marshal QR payload: %w", err)
	}

	payloadStr := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := s.signer.SignMsg("qr-payload:"+payloadStr, exp)

	return QRResult{
		Payload:   payloadStr,
		Signature: signature,
		ExpiresAt: expTime,
	}, nil
}

func (s *Service) Resolve(payloadStr, signatureStr string) (ResolveResult, error) {
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadStr)
	if err != nil {
		return ResolveResult{}, ErrQRInvalid
	}

	var payload QRPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return ResolveResult{}, ErrQRInvalid
	}

	// 1. Verify signature first (never leak expiry info on forged links)
	if !s.signer.VerifyMsg("qr-payload:"+payloadStr, payload.Exp, signatureStr) {
		return ResolveResult{}, ErrQRInvalid
	}

	// 2. Verify expiry
	if time.Now().Unix() > payload.Exp {
		return ResolveResult{}, ErrQRExpired
	}

	// 3. Generate receiver_token (TTL 10 mins)
	tokenExpTime := time.Now().Add(10 * time.Minute)
	tokenExp := tokenExpTime.Unix()

	tokenPayload := ReceiverTokenPayload{
		UserID:  payload.UserID,
		Exp:     tokenExp,
		Purpose: "qr-receiver",
	}

	tokenPayloadBytes, err := json.Marshal(tokenPayload)
	if err != nil {
		return ResolveResult{}, fmt.Errorf("marshal token payload: %w", err)
	}

	tokenPayloadStr := base64.RawURLEncoding.EncodeToString(tokenPayloadBytes)
	tokenSig := s.signer.SignMsg("qr-receiver:"+tokenPayloadStr, tokenExp)

	combinedToken := tokenPayloadStr + "." + tokenSig

	return ResolveResult{
		NameMasked:    payload.NameMasked,
		PhoneMasked:   payload.PhoneMasked,
		ReceiverToken: combinedToken,
		ExpiresAt:     tokenExpTime,
	}, nil
}

func (s *Service) VerifyReceiverToken(tokenStr string) (uuid.UUID, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return uuid.Nil, ErrTokenInvalid
	}

	tokenPayloadStr := parts[0]
	tokenSig := parts[1]

	tokenPayloadBytes, err := base64.RawURLEncoding.DecodeString(tokenPayloadStr)
	if err != nil {
		return uuid.Nil, ErrTokenInvalid
	}

	var tokenPayload ReceiverTokenPayload
	if err := json.Unmarshal(tokenPayloadBytes, &tokenPayload); err != nil {
		return uuid.Nil, ErrTokenInvalid
	}

	// 1. Verify signature first
	if !s.signer.VerifyMsg("qr-receiver:"+tokenPayloadStr, tokenPayload.Exp, tokenSig) {
		return uuid.Nil, ErrTokenInvalid
	}

	// 2. Check purpose
	if tokenPayload.Purpose != "qr-receiver" {
		return uuid.Nil, ErrTokenInvalid
	}

	// 3. Verify expiry
	if time.Now().Unix() > tokenPayload.Exp {
		return uuid.Nil, ErrTokenExpired
	}

	return tokenPayload.UserID, nil
}

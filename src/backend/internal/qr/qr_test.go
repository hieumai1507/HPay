package qr

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockSigner struct {
	secret string
}

func (m *mockSigner) SignMsg(msg string, exp int64) string {
	return msg + "-signed-with-" + m.secret
}

func (m *mockSigner) VerifyMsg(msg string, exp int64, sig string) bool {
	return sig == msg+"-signed-with-"+m.secret
}

func TestQR_Flow(t *testing.T) {
	signer := &mockSigner{secret: "secret123"}
	svc := NewService(signer)

	userID := uuid.New()
	name := "Nguyen V** A*"
	phone := "091****678"

	// 1. Mint QR
	res, err := svc.Mint(userID, name, phone)
	if err != nil {
		t.Fatalf("Mint failed: %v", err)
	}

	if res.Payload == "" || res.Signature == "" {
		t.Errorf("expected non-empty payload and signature")
	}

	// 2. Resolve QR
	resolved, err := svc.Resolve(res.Payload, res.Signature)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if resolved.NameMasked != name || resolved.PhoneMasked != phone {
		t.Errorf("resolved info mismatch: got (%q, %q), want (%q, %q)", resolved.NameMasked, resolved.PhoneMasked, name, phone)
	}

	if resolved.ReceiverToken == "" {
		t.Fatalf("expected non-empty receiver token")
	}

	// 3. Verify Receiver Token
	verifiedID, err := svc.VerifyReceiverToken(resolved.ReceiverToken)
	if err != nil {
		t.Fatalf("VerifyReceiverToken failed: %v", err)
	}

	if verifiedID != userID {
		t.Errorf("verified user ID mismatch: got %v, want %v", verifiedID, userID)
	}
}

func TestQR_InvalidSignature(t *testing.T) {
	signer := &mockSigner{secret: "secret123"}
	svc := NewService(signer)

	userID := uuid.New()
	res, err := svc.Mint(userID, "Name", "Phone")
	if err != nil {
		t.Fatalf("Mint failed: %v", err)
	}

	// Try resolving with wrong signature
	_, err = svc.Resolve(res.Payload, "wrong-signature")
	if !errors.Is(err, ErrQRInvalid) {
		t.Errorf("expected ErrQRInvalid, got %v", err)
	}
}

func TestQR_Expired(t *testing.T) {
	signer := &mockSigner{secret: "secret123"}
	svc := NewService(signer)

	userID := uuid.New()
	res, err := svc.Mint(userID, "Name", "Phone")
	if err != nil {
		t.Fatalf("Mint failed: %v", err)
	}

	// Manually construct an expired payload
	payloadBytes, _ := base64.RawURLEncoding.DecodeString(res.Payload)
	var payload QRPayload
	json.Unmarshal(payloadBytes, &payload)
	payload.Exp = time.Now().Add(-1 * time.Minute).Unix() // 1 minute ago

	newPayloadBytes, _ := json.Marshal(payload)
	expiredPayloadStr := base64.RawURLEncoding.EncodeToString(newPayloadBytes)
	newSig := signer.SignMsg("qr-payload:"+expiredPayloadStr, payload.Exp)

	_, err = svc.Resolve(expiredPayloadStr, newSig)
	if !errors.Is(err, ErrQRExpired) {
		t.Errorf("expected ErrQRExpired, got %v", err)
	}
}

func TestQR_TokenInvalidAndExpired(t *testing.T) {
	signer := &mockSigner{secret: "secret123"}
	svc := NewService(signer)

	userID := uuid.New()
	res, err := svc.Mint(userID, "Name", "Phone")
	if err != nil {
		t.Fatalf("Mint failed: %v", err)
	}

	resolved, err := svc.Resolve(res.Payload, res.Signature)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// 1. Invalid token signature
	parts := strings.Split(resolved.ReceiverToken, ".")
	wrongToken := parts[0] + ".wrong-sig"
	_, err = svc.VerifyReceiverToken(wrongToken)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}

	// 2. Expired token
	tokenPayloadBytes, _ := base64.RawURLEncoding.DecodeString(parts[0])
	var tokenPayload ReceiverTokenPayload
	json.Unmarshal(tokenPayloadBytes, &tokenPayload)
	tokenPayload.Exp = time.Now().Add(-1 * time.Minute).Unix()

	expiredTokenBytes, _ := json.Marshal(tokenPayload)
	expiredTokenPayloadStr := base64.RawURLEncoding.EncodeToString(expiredTokenBytes)
	expiredSig := signer.SignMsg("qr-receiver:"+expiredTokenPayloadStr, tokenPayload.Exp)
	expiredToken := expiredTokenPayloadStr + "." + expiredSig

	_, err = svc.VerifyReceiverToken(expiredToken)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}

	// 3. Invalid purpose
	tokenPayload.Exp = time.Now().Add(10 * time.Minute).Unix()
	tokenPayload.Purpose = "wrong-purpose"
	invalidPurposeBytes, _ := json.Marshal(tokenPayload)
	invalidPurposePayloadStr := base64.RawURLEncoding.EncodeToString(invalidPurposeBytes)
	invalidPurposeSig := signer.SignMsg("qr-receiver:"+invalidPurposePayloadStr, tokenPayload.Exp)
	invalidPurposeToken := invalidPurposePayloadStr + "." + invalidPurposeSig

	_, err = svc.VerifyReceiverToken(invalidPurposeToken)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

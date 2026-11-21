package auth

import (
	"testing"
	"time"
)

func newChallenge(code string) OTPChallenge {
	return OTPChallenge{
		ID:          "user:approve_txn",
		CodeHash:    HashOTP("user:approve_txn", code),
		MaxAttempts: OTPMaxAttempts,
		ExpiresAt:   time.Now().Add(OTPTTL),
	}
}

// TestEdge_E11_OTPBruteForce: sai 3 lần thì challenge vô hiệu vĩnh viễn —
// kể cả nhập ĐÚNG ở lần 4 cũng phải bị từ chối (docs 01 §4 E11).
func TestEdge_E11_OTPBruteForce(t *testing.T) {
	ch := newChallenge("123456")
	now := time.Now()

	for attempt := 1; attempt <= 3; attempt++ {
		err := CheckOTP(ch, "000000", attempt, now)
		if attempt < 3 && err != ErrOTPMismatch {
			t.Fatalf("attempt %d: want mismatch, got %v", attempt, err)
		}
		if attempt == 3 && err != ErrOTPMaxAttempts {
			t.Fatalf("attempt 3: want max attempts, got %v", err)
		}
	}
	// attempt 4 with the CORRECT code must still fail
	if err := CheckOTP(ch, "123456", 4, now); err != ErrOTPMaxAttempts {
		t.Fatalf("correct code after exhaustion: want ErrOTPMaxAttempts, got %v", err)
	}
}

func TestOTPHappyPath(t *testing.T) {
	ch := newChallenge("654321")
	if err := CheckOTP(ch, "654321", 1, time.Now()); err != nil {
		t.Fatalf("want ok, got %v", err)
	}
}

func TestOTPExpired(t *testing.T) {
	ch := newChallenge("111111")
	ch.ExpiresAt = time.Now().Add(-time.Second)
	if err := CheckOTP(ch, "111111", 1, time.Now()); err != ErrOTPExpired {
		t.Fatalf("want expired, got %v", err)
	}
}

func TestOTPHashBoundToChallenge(t *testing.T) {
	if HashOTP("a:purpose", "123456") == HashOTP("b:purpose", "123456") {
		t.Fatal("same code on different challenges must hash differently")
	}
}

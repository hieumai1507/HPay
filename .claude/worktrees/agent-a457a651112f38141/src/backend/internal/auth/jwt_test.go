package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTRoundtrip(t *testing.T) {
	j := NewJWTIssuer("test-secret-test-secret-test-secret", "user")
	uid := uuid.New()
	tok, err := j.Issue(uid, "device-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, dev, err := j.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got != uid || dev != "device-1" {
		t.Fatalf("claims mismatch: %v %v", got, dev)
	}
}

func TestJWTExpired(t *testing.T) {
	j := NewJWTIssuer("test-secret-test-secret-test-secret", "user")
	tok, _ := j.Issue(uuid.New(), "d", time.Now().Add(-time.Hour))
	if _, _, err := j.Parse(tok); err != ErrTokenExpired {
		t.Fatalf("want ErrTokenExpired, got %v", err)
	}
}

func TestJWTWrongAudience(t *testing.T) {
	user := NewJWTIssuer("test-secret-test-secret-test-secret", "user")
	admin := NewJWTIssuer("test-secret-test-secret-test-secret", "admin")
	tok, _ := user.Issue(uuid.New(), "d", time.Now())
	if _, _, err := admin.Parse(tok); err == nil {
		t.Fatal("user token must not pass admin audience")
	}
}

package storage

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func TestLocalPutGetAndTraversalGuard(t *testing.T) {
	l, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := l.Put(ctx, "req-1/id_card.jpg", []byte("img")); err != nil {
		t.Fatal(err)
	}
	got, err := l.Get(ctx, "req-1/id_card.jpg")
	if err != nil || string(got) != "img" {
		t.Fatalf("get: %v %q", err, got)
	}
	if _, err := l.Get(ctx, "req-1/missing.jpg"); err != ErrNotFound {
		t.Fatalf("missing: want ErrNotFound, got %v", err)
	}
	for _, bad := range []string{"../etc/passwd", "/abs", "a/../../b", ""} {
		if _, err := l.Get(ctx, bad); err != ErrBadKey {
			t.Fatalf("traversal %q: want ErrBadKey, got %v", bad, err)
		}
	}
}

func TestSignerRoundTripExpiryAndTamper(t *testing.T) {
	s := NewSigner("test-secret")
	signed := s.SignURL("req-1/selfie.jpg", 5*time.Minute)
	u, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	exp, sig := u.Query().Get("exp"), u.Query().Get("sig")
	if err := s.Verify("req-1/selfie.jpg", exp, sig); err != nil {
		t.Fatalf("valid link rejected: %v", err)
	}
	if err := s.Verify("req-1/OTHER.jpg", exp, sig); err != ErrInvalidSignature {
		t.Fatalf("tampered key: want ErrInvalidSignature, got %v", err)
	}
	if err := s.Verify("req-1/selfie.jpg", "1", s.mac("req-1/selfie.jpg", 1)); err != ErrLinkExpired {
		t.Fatalf("expired: want ErrLinkExpired, got %v", err)
	}
	if err := NewSigner("other-secret").Verify("req-1/selfie.jpg", exp, sig); err != ErrInvalidSignature {
		t.Fatalf("wrong secret: want ErrInvalidSignature, got %v", err)
	}
}

package auth

import "testing"

func TestPasswordRoundtrip(t *testing.T) {
	h, err := HashPassword("S3cret!password")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(h, "S3cret!password"); err != nil {
		t.Fatalf("want match, got %v", err)
	}
	if err := VerifyPassword(h, "wrong-password"); err == nil {
		t.Fatal("want mismatch, got nil")
	}
}

func TestPasswordSaltUnique(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("two hashes of the same password must differ (random salt)")
	}
}

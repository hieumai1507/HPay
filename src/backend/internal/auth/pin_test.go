package auth

// ADR-0008 / ADR-0014 / spec w3-transaction-auth: server-side PIN.
// Pure rules (format, hashing) are tested without a database; verification,
// the attempt counter and lockout are tested against the shared test database.
// Cases: TC-AUT-004, TC-AUT-009.

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

func TestValidatePINFormat(t *testing.T) {
	good := []string{"135790", "482915", "908172"}
	for _, p := range good {
		if err := ValidatePIN(p); err != nil {
			t.Errorf("ValidatePIN(%q) = %v, want nil", p, err)
		}
	}
	bad := map[string]string{
		"":        "empty",
		"12345":   "5 digits",
		"1234567": "7 digits",
		"abcdef":  "letters",
		"12 456":  "space",
		"000000":  "all same digit",
		"111111":  "all same digit",
		"123456":  "ascending sequence",
		"654321":  "descending sequence",
		"012345":  "ascending sequence",
	}
	for p, why := range bad {
		if err := ValidatePIN(p); !errors.Is(err, ErrWeakPIN) {
			t.Errorf("ValidatePIN(%q) (%s) = %v, want ErrWeakPIN", p, why, err)
		}
	}
}

func newPinService(t *testing.T) (*Service, uuid.UUID, string) {
	t.Helper()
	pool := testutil.PoolFromEnv(t)
	log := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc := NewService(pool, NewJWTIssuer("pin-test-secret-0123456789-0123456789", "user"), log, "ci")
	svc.SetTOTPKey([]byte("0123456789abcdef0123456789abcdef"))
	svc.SetPINPepper(testPinPepperSet(t))
	userID, _ := testutil.NewUserWithWallet(t, pool)
	// testutil.NewUserWithWallet stores password_hash 'x'; give the user a real password for step-up.
	pw := "S3cret!password"
	h, err := HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, h); err != nil {
		t.Fatal(err)
	}
	return svc, userID, pw
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}

// TC-AUT-009: set requires the password, change requires the current PIN, weak PINs refused, hash stored.
func TestPin_SetAndChange(t *testing.T) {
	svc, uid, pw := newPinService(t)
	ctx := context.Background()

	if err := svc.SetPIN(ctx, uid, "wrong-password", "482915"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("set with wrong password: want ErrInvalidCredentials, got %v", err)
	}
	if err := svc.SetPIN(ctx, uid, pw, "123456"); !errors.Is(err, ErrWeakPIN) {
		t.Fatalf("set weak PIN: want ErrWeakPIN, got %v", err)
	}
	if err := svc.SetPIN(ctx, uid, pw, "482915"); err != nil {
		t.Fatalf("set PIN: %v", err)
	}
	var stored string
	if err := svc.pool.QueryRow(ctx, `SELECT pin_hash FROM users WHERE id=$1`, uid).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "482915" || !strings.HasPrefix(stored, "$argon2id$") {
		t.Fatalf("PIN must be stored as an argon2id hash, got %q", stored)
	}

	// once a PIN exists, setting again needs the CURRENT PIN, not the password
	if err := svc.ChangePIN(ctx, uid, "908172", "135790"); !errors.Is(err, ErrPINIncorrect) {
		t.Fatalf("change with wrong current PIN: want ErrPINIncorrect, got %v", err)
	}
	if err := svc.ChangePIN(ctx, uid, "482915", "135790"); err != nil {
		t.Fatalf("change PIN: %v", err)
	}
	if err := svc.VerifyPIN(ctx, uid, "482915"); !errors.Is(err, ErrPINIncorrect) {
		t.Fatalf("old PIN must stop working, got %v", err)
	}
	if err := svc.VerifyPIN(ctx, uid, "135790"); err != nil {
		t.Fatalf("new PIN must work: %v", err)
	}
}

// AC-6: a user with no PIN gets a distinct error.
func TestPin_NoPinSet(t *testing.T) {
	svc, uid, _ := newPinService(t)
	if err := svc.VerifyPIN(context.Background(), uid, "482915"); !errors.Is(err, ErrPINNotSet) {
		t.Fatalf("want ErrPINNotSet, got %v", err)
	}
}

// TC-AUT-004: five wrong PINs lock PIN use; a correct PIN is refused during the lock; it works again afterwards.
func TestPin_LockoutAfterFiveFailures(t *testing.T) {
	svc, uid, pw := newPinService(t)
	ctx := context.Background()
	if err := svc.SetPIN(ctx, uid, pw, "482915"); err != nil {
		t.Fatal(err)
	}

	for i := 1; i <= 4; i++ {
		if err := svc.VerifyPIN(ctx, uid, "000001"); !errors.Is(err, ErrPINIncorrect) {
			t.Fatalf("attempt %d: want ErrPINIncorrect, got %v", i, err)
		}
	}
	// a success resets the counter
	if err := svc.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("correct PIN before the lock: %v", err)
	}
	for i := 1; i <= 5; i++ {
		err := svc.VerifyPIN(ctx, uid, "000001")
		if i < 5 && !errors.Is(err, ErrPINIncorrect) {
			t.Fatalf("attempt %d after reset: want ErrPINIncorrect, got %v", i, err)
		}
	}
	// now locked: even the correct PIN is refused
	if err := svc.VerifyPIN(ctx, uid, "482915"); !errors.Is(err, ErrPINLocked) {
		t.Fatalf("during the lock the correct PIN must be refused with ErrPINLocked, got %v", err)
	}
	var until time.Time
	if err := svc.pool.QueryRow(ctx, `SELECT pin_locked_until FROM users WHERE id=$1`, uid).Scan(&until); err != nil {
		t.Fatal(err)
	}
	if d := time.Until(until); d < 14*time.Minute || d > 16*time.Minute {
		t.Errorf("lock lasts %v, want about 15 minutes", d)
	}
	// the lock expires
	if _, err := svc.pool.Exec(ctx, `UPDATE users SET pin_locked_until = now() - interval '1 second' WHERE id=$1`, uid); err != nil {
		t.Fatal(err)
	}
	if err := svc.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("after the lock expires the correct PIN must work: %v", err)
	}
	var attempts int
	_ = svc.pool.QueryRow(ctx, `SELECT pin_failed_attempts FROM users WHERE id=$1`, uid).Scan(&attempts)
	if attempts != 0 {
		t.Errorf("counter after success = %d, want 0", attempts)
	}
}

// Failures and lockouts are audited, and neither the PIN nor its hash is written to the audit trail.
func TestPin_FailuresAreAuditedWithoutTheSecret(t *testing.T) {
	svc, uid, pw := newPinService(t)
	ctx := context.Background()
	if err := svc.SetPIN(ctx, uid, pw, "482915"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		_ = svc.VerifyPIN(ctx, uid, "908172")
	}
	var n int
	if err := svc.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND action IN ('pin.failed','pin.locked')`, uid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 5 {
		t.Fatalf("audit rows for failures = %d, want at least 5", n)
	}
	var leaked int
	if err := svc.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND (resource LIKE '%908172%' OR resource LIKE '%482915%' OR coalesce(reason,'') LIKE '%908172%' OR coalesce(reason,'') LIKE '%482915%' OR resource LIKE '%argon2%')`, uid).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatalf("%d audit rows contain a PIN or hash", leaked)
	}
}

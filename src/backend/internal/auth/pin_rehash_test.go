package auth

// ADR-0018 / spec w5-pin-pepper: rehash-on-verify against the real database.
// Cases TC-PEP-020, TC-PEP-021, TC-PEP-027.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
)

func testPinPepperSet(t *testing.T) *PepperSet {
	t.Helper()
	return mustSet(t, PepperKey{"p1", testPepper(1)})
}

func setLegacyPIN(t *testing.T, svc *Service, uid uuid.UUID, pin string) string {
	t.Helper()
	h, err := HashPassword(pin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.pool.Exec(context.Background(), `UPDATE users SET pin_hash=$2 WHERE id=$1`, uid, h); err != nil {
		t.Fatal(err)
	}
	return h
}

func storedHash(t *testing.T, svc *Service, uid uuid.UUID) string {
	t.Helper()
	var h string
	if err := svc.pool.QueryRow(context.Background(), `SELECT pin_hash FROM users WHERE id=$1`, uid).Scan(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

// TC-PEP-020: a PIN change that lands between the verification and the rewrite is not overwritten.
func TestPinRehash_ConcurrentChangeNotOverwritten(t *testing.T) {
	svc, uid, pw := newPinService(t)
	ctx := context.Background()
	setLegacyPIN(t, svc, uid, "482915")

	changed := false
	beforeRehashWrite = func() {
		changed = true
		// the user changes the PIN in another request, after the old hash was read and verified
		if _, err := svc.pool.Exec(ctx, `UPDATE users SET pin_hash=$2 WHERE id=$1`, uid, mustHash(t, svc, "135790")); err != nil {
			t.Error(err)
		}
	}
	defer func() { beforeRehashWrite = nil }()

	if err := svc.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("the verification itself must succeed: %v", err)
	}
	if !changed {
		t.Fatal("the rehash path was not reached for a legacy hash")
	}
	beforeRehashWrite = nil
	if err := svc.VerifyPIN(ctx, uid, "135790"); err != nil {
		t.Fatalf("the PIN changed meanwhile must still be the one that works: %v", err)
	}
	if err := svc.VerifyPIN(ctx, uid, "482915"); !errors.Is(err, ErrPINIncorrect) {
		t.Fatalf("the old PIN must not have been written back, got %v", err)
	}
	_ = pw
}

func mustHash(t *testing.T, svc *Service, pin string) string {
	t.Helper()
	h, err := svc.pins.HashPIN(pin)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// TC-PEP-021: a failing rewrite must not fail the payment; the log names the user and no secret.
func TestPinRehash_FailedRewriteDoesNotFailVerify(t *testing.T) {
	svc, uid, _ := newPinService(t)
	ctx := context.Background()
	var buf bytes.Buffer
	svc.log = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	legacy := setLegacyPIN(t, svc, uid, "482915")

	fn := "pep_fail_" + strings.ReplaceAll(uid.String(), "-", "")
	if _, err := svc.pool.Exec(ctx, fmt.Sprintf(`
CREATE FUNCTION %[1]s() RETURNS trigger AS $$
BEGIN
  IF NEW.id = '%[2]s' AND NEW.pin_hash LIKE '%%,k=%%' THEN RAISE EXCEPTION 'rewrite blocked by test'; END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql`, fn, uid)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.pool.Exec(ctx, fmt.Sprintf(`CREATE TRIGGER %[1]s BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION %[1]s()`, fn)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = svc.pool.Exec(context.Background(), fmt.Sprintf(`DROP TRIGGER IF EXISTS %[1]s ON users; DROP FUNCTION IF EXISTS %[1]s()`, fn))
	})

	if err := svc.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("a failed rewrite must not fail the verification: %v", err)
	}
	if got := storedHash(t, svc, uid); got != legacy {
		t.Fatalf("the legacy hash must be untouched when the rewrite fails, got %q", got)
	}
	if err := svc.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("the legacy hash must still verify: %v", err)
	}
	logs := buf.String()
	if !strings.Contains(logs, "level=WARN") || !strings.Contains(logs, uid.String()) {
		t.Fatalf("want a warn log naming the user id, got %q", logs)
	}
	for _, secret := range []string{"482915", legacy, "rewrite blocked"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("the warn log must hold the user id only, found %q in %q", secret, logs)
		}
	}
}

// TC-PEP-027: no PIN and unknown user still spend one derivation with the new parameters.
func TestPinVerify_NoPinAndUnknownUserSpendOneDerivation(t *testing.T) {
	svc, uid, _ := newPinService(t) // no PIN set
	var calls atomic.Int32
	var last atomic.Value
	deriveTap = func(tt, m uint32, p uint8) { calls.Add(1); last.Store(fmt.Sprintf("t=%d m=%d p=%d", tt, m, p)) }
	defer func() { deriveTap = nil }()

	for name, id := range map[string]uuid.UUID{"user without a PIN": uid, "unknown user": uuid.New()} {
		calls.Store(0)
		if err := svc.VerifyPIN(context.Background(), id, "482915"); !errors.Is(err, ErrPINNotSet) {
			t.Fatalf("%s: want ErrPINNotSet, got %v", name, err)
		}
		if calls.Load() != 1 || last.Load() != "t=2 m=19456 p=1" {
			t.Fatalf("%s: want one derivation with t=2 m=19456 p=1, got %d (%v)", name, calls.Load(), last.Load())
		}
	}
}

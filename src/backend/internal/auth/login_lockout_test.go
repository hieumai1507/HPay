package auth

// BE-H8 (BUG-0016): per-account login lockout and constant work for unknown accounts.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

func newLoginRig(t *testing.T) (*Service, *AdminService, string, string, string) {
	t.Helper()
	pool := testutil.PoolFromEnv(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	issuer := NewJWTIssuer("login-test-secret-0123456789-0123456789", "user")
	svc := NewService(pool, issuer, log, "ci")
	phone := "09" + uuid.NewString()[:8]
	digits := []rune(phone)
	for i := 2; i < len(digits); i++ {
		digits[i] = '0' + rune(digits[i]%10)
	}
	phone = string(digits)
	if _, _, err := svc.Register(context.Background(), phone, "correct-horse-1", "Login Test", "dev1"); err != nil {
		t.Fatalf("register: %v", err)
	}
	admin := NewAdminService(pool, NewJWTIssuer("login-test-secret-0123456789-0123456789", "admin"))
	email := "lock-" + uuid.NewString()[:8] + "@HPay.dev"
	h, _ := HashPassword("admin-pass-123")
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO admin_users (email, password_hash, full_name, role, status) VALUES ($1,$2,'Lock Test','superadmin','active')`, email, h); err != nil {
		t.Fatal(err)
	}
	return svc, admin, phone, email, "correct-horse-1"
}

// AC-1: five wrong passwords lock the account; the correct password is then refused.
func TestLogin_LocksAfterFiveFailures(t *testing.T) {
	svc, _, phone, _, good := newLoginRig(t)
	ctx := context.Background()
	for i := 1; i <= 4; i++ {
		if _, _, err := svc.Login(ctx, phone, "wrong-password", "d"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: want ErrInvalidCredentials, got %v", i, err)
		}
	}
	if _, _, err := svc.Login(ctx, phone, "wrong-password", "d"); !errors.Is(err, ErrLoginLocked) {
		t.Fatalf("the 5th failure must lock the account, got %v", err)
	}
	if _, _, err := svc.Login(ctx, phone, good, "d"); !errors.Is(err, ErrLoginLocked) {
		t.Fatalf("the correct password must be refused while locked, got %v", err)
	}
}

// AC-2: a success resets the counter.
func TestLogin_SuccessResetsCounter(t *testing.T) {
	svc, _, phone, _, good := newLoginRig(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_, _, _ = svc.Login(ctx, phone, "wrong-password", "d")
	}
	if _, _, err := svc.Login(ctx, phone, good, "d"); err != nil {
		t.Fatalf("4 failures must not lock: %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, _, err := svc.Login(ctx, phone, "wrong-password", "d"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("after a success the counter restarts, got %v", err)
		}
	}
}

// AC-3: an expired lock lets the user in again and restarts the count.
func TestLogin_LockExpires(t *testing.T) {
	svc, _, phone, _, good := newLoginRig(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _, _ = svc.Login(ctx, phone, "wrong-password", "d")
	}
	if _, err := svc.pool.Exec(ctx, `UPDATE users SET login_locked_until = now() - interval '1 second' WHERE phone=$1`, phone); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(ctx, phone, good, "d"); err != nil {
		t.Fatalf("after the lock expired the correct password must work: %v", err)
	}
}

// AC-4: failures are audited without the password.
func TestLogin_FailuresAreAudited(t *testing.T) {
	svc, _, phone, _, _ := newLoginRig(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _, _ = svc.Login(ctx, phone, "very-secret-wrong-pw", "d")
	}
	var n int
	if err := svc.pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_logs a JOIN users u ON u.id = a.actor_id
		WHERE u.phone=$1 AND a.action IN ('auth.login_failed','auth.login_locked')
		  AND coalesce(a.reason,'') NOT LIKE '%very-secret-wrong-pw%'`, phone).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 5 {
		t.Fatalf("want 5 audit rows (4 failed + 1 locked), got %d", n)
	}
}

// AC-5: an unknown phone does the same hashing work as a known one (no timing oracle).
func TestLogin_UnknownAccountCostsAsMuchAsAWrongPassword(t *testing.T) {
	svc, _, phone, _, _ := newLoginRig(t)
	ctx := context.Background()
	timeIt := func(p string) time.Duration {
		best := time.Hour
		for i := 0; i < 3; i++ {
			s := time.Now()
			_, _, _ = svc.Login(ctx, p, "wrong-password", "d")
			if d := time.Since(s); d < best {
				best = d
			}
			// keep the real account below its lock threshold
			_, _ = svc.pool.Exec(ctx, `UPDATE users SET login_failed_attempts=0, login_locked_until=NULL WHERE phone=$1`, phone)
		}
		return best
	}
	known := timeIt(phone)
	unknown := timeIt("0999999999")
	if unknown < known/2 {
		t.Fatalf("an unknown phone answered in %v but a wrong password takes %v: the difference reveals which phones exist", unknown, known)
	}
}

// AC-6: the admin login has the same lockout, and the correct password is refused while locked.
func TestAdminLogin_LocksAfterFiveFailures(t *testing.T) {
	_, admin, _, email, _ := newLoginRig(t)
	ctx := context.Background()
	for i := 1; i <= 4; i++ {
		if _, _, err := admin.Login(ctx, email, "wrong"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, _, err := admin.Login(ctx, email, "wrong"); !errors.Is(err, ErrLoginLocked) {
		t.Fatalf("5th failure must lock, got %v", err)
	}
	if _, _, err := admin.Login(ctx, email, "admin-pass-123"); !errors.Is(err, ErrLoginLocked) {
		t.Fatalf("correct password refused while locked, got %v", err)
	}
}

// AC-7: concurrent wrong attempts cannot slip past the counter.
func TestLogin_ConcurrentFailuresStillLock(t *testing.T) {
	svc, _, phone, _, good := newLoginRig(t)
	ctx := context.Background()
	done := make(chan struct{}, 10)
	for i := 0; i < 10; i++ {
		go func() { _, _, _ = svc.Login(ctx, phone, "wrong-password", "d"); done <- struct{}{} }()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if _, _, err := svc.Login(ctx, phone, good, "d"); !errors.Is(err, ErrLoginLocked) {
		t.Fatalf("10 parallel failures must leave the account locked, got %v", err)
	}
}

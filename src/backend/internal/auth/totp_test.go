package auth

// ADR-0017 / spec w4-2fa: TOTP second factor. Cases TC-2FA-001..009 (service level, real DB).
// The RFC 6238 vectors are checked first with pure functions.

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// RFC 6238 appendix B test vectors for SHA-1 (secret "12345678901234567890"), 8 digits there;
// the 6-digit codes are the last six digits.
func TestTOTP_RFC6238Vectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"},
		{1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"},
	}
	for _, c := range cases {
		if got := totpCode(secret, c.unix/30); got != c.want {
			t.Errorf("t=%d: got %s, want %s", c.unix, got, c.want)
		}
	}
}

type totpRig struct {
	*resetRig
	secret []byte
}

func newTOTPRig(t *testing.T) *totpRig {
	t.Helper()
	r := newResetRig(t)
	r.svc.SetTOTPKey(testTOTPKey())
	return &totpRig{resetRig: r}
}

func testTOTPKey() []byte { return []byte("0123456789abcdef0123456789abcdef") }

// enrol starts enrolment and returns the secret the user would load into the app.
func (r *totpRig) enrol(t *testing.T) []byte {
	t.Helper()
	out, err := r.svc.EnrollTOTP(context.Background(), r.uid, r.pw)
	if err != nil {
		t.Fatalf("enrol: %v", err)
	}
	secret, err := decodeSecret(out.Secret)
	if err != nil {
		t.Fatalf("the returned secret must be base32: %v", err)
	}
	r.secret = secret
	return secret
}

func (r *totpRig) activate(t *testing.T) []string {
	t.Helper()
	secret := r.enrol(t)
	step := time.Now().Unix() / 30
	out, err := r.svc.ConfirmTOTP(context.Background(), r.uid, totpCode(secret, step))
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	return out.RecoveryCodes
}

// nextCode returns a valid code for a step that has not been used yet.
func (r *totpRig) codeAt(offset int64) string {
	return totpCode(r.secret, time.Now().Unix()/30+offset)
}

// TC-2FA-001
func TestTOTP_EnrollReturnsSecretOnce(t *testing.T) {
	r := newTOTPRig(t)
	out, err := r.svc.EnrollTOTP(context.Background(), r.uid, r.pw)
	if err != nil {
		t.Fatal(err)
	}
	if out.Secret == "" || !strings.HasPrefix(out.URI, "otpauth://totp/") || !strings.Contains(out.URI, "secret="+out.Secret) {
		t.Fatalf("want a secret and an otpauth URI that carries it, got %+v", out)
	}
	st, err := r.svc.TOTPStatus(context.Background(), r.uid)
	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled {
		t.Fatal("TOTP must not be active before the first code is confirmed")
	}
}

// TC-2FA-002
func TestTOTP_EnrollNeedsPassword(t *testing.T) {
	r := newTOTPRig(t)
	if _, err := r.svc.EnrollTOTP(context.Background(), r.uid, "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	var n int
	if err := r.svc.pool.QueryRow(context.Background(), `SELECT count(*) FROM user_totp WHERE user_id=$1`, r.uid).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no secret may be created for a wrong password (rows=%d err=%v)", n, err)
	}
}

// TC-2FA-003
func TestTOTP_ConfirmActivatesAndIssuesRecoveryCodes(t *testing.T) {
	r := newTOTPRig(t)
	codes := r.activate(t)
	if len(codes) != 10 {
		t.Fatalf("want 10 recovery codes, got %d", len(codes))
	}
	st, _ := r.svc.TOTPStatus(context.Background(), r.uid)
	if !st.Enabled || st.RecoveryCodesLeft != 10 {
		t.Fatalf("want enabled with 10 codes left, got %+v", st)
	}
	// stored only as hashes
	rows, err := r.svc.pool.Query(context.Background(), `SELECT code_hash FROM user_recovery_codes WHERE user_id=$1`, r.uid)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			t.Fatal(err)
		}
		for _, c := range codes {
			if h == c {
				t.Fatal("a recovery code is stored in clear")
			}
		}
		n++
	}
	if n != 10 {
		t.Fatalf("want 10 stored hashes, got %d", n)
	}
}

// TC-2FA-004
func TestTOTP_WrongCodesLockUse(t *testing.T) {
	r := newTOTPRig(t)
	ctx := context.Background()
	r.enrol(t)
	for i := 1; i <= 4; i++ {
		if _, err := r.svc.ConfirmTOTP(ctx, r.uid, "000000"); !errors.Is(err, ErrTOTPInvalid) {
			t.Fatalf("attempt %d: want ErrTOTPInvalid, got %v", i, err)
		}
	}
	if _, err := r.svc.ConfirmTOTP(ctx, r.uid, "000000"); !errors.Is(err, ErrTOTPLocked) {
		t.Fatalf("the 5th wrong attempt must lock, got %v", err)
	}
	if _, err := r.svc.ConfirmTOTP(ctx, r.uid, r.codeAt(0)); !errors.Is(err, ErrTOTPLocked) {
		t.Fatalf("the right code must be refused while locked, got %v", err)
	}
}

// TC-2FA-005
func TestTOTP_AcceptsOneStepOfDrift(t *testing.T) {
	for _, off := range []int64{-1, 0, 1} {
		r := newTOTPRig(t)
		r.activate(t)
		// activation consumed the current step; use a fresh user for each offset instead
		r2 := newTOTPRig(t)
		secret := r2.enrol(t)
		if _, err := r2.svc.ConfirmTOTP(context.Background(), r2.uid, totpCode(secret, time.Now().Unix()/30+off)); err != nil {
			t.Fatalf("offset %d must be accepted: %v", off, err)
		}
	}
	r := newTOTPRig(t)
	secret := r.enrol(t)
	for _, off := range []int64{-2, 2} {
		if _, err := r.svc.ConfirmTOTP(context.Background(), r.uid, totpCode(secret, time.Now().Unix()/30+off)); err == nil {
			t.Fatalf("offset %d must be refused", off)
		}
	}
}

// TC-2FA-006
func TestTOTP_CodeIsSingleUsePerStep(t *testing.T) {
	r := newTOTPRig(t)
	ctx := context.Background()
	r.activate(t) // consumed the current step
	if err := r.svc.VerifyTOTP(ctx, r.uid, r.codeAt(0)); !errors.Is(err, ErrTOTPInvalid) {
		t.Fatalf("the step used at activation must not be accepted again, got %v", err)
	}
	next := r.codeAt(1)
	if err := r.svc.VerifyTOTP(ctx, r.uid, next); err != nil {
		t.Fatalf("a fresh step must be accepted: %v", err)
	}
	if err := r.svc.VerifyTOTP(ctx, r.uid, next); !errors.Is(err, ErrTOTPInvalid) {
		t.Fatalf("replaying the same code must be refused, got %v", err)
	}
}

// TC-2FA-007
func TestTOTP_RecoveryCodeIsSingleUse(t *testing.T) {
	r := newTOTPRig(t)
	ctx := context.Background()
	codes := r.activate(t)
	if err := r.svc.VerifyTOTP(ctx, r.uid, codes[0]); err != nil {
		t.Fatalf("a recovery code must be accepted once: %v", err)
	}
	if err := r.svc.VerifyTOTP(ctx, r.uid, codes[0]); !errors.Is(err, ErrTOTPInvalid) {
		t.Fatalf("a used recovery code must be refused, got %v", err)
	}
	st, _ := r.svc.TOTPStatus(ctx, r.uid)
	if st.RecoveryCodesLeft != 9 {
		t.Fatalf("want 9 recovery codes left, got %d", st.RecoveryCodesLeft)
	}
}

// TC-2FA-008
func TestTOTP_SecretIsEncryptedAtRest(t *testing.T) {
	r := newTOTPRig(t)
	secret := r.enrol(t)
	var enc []byte
	if err := r.svc.pool.QueryRow(context.Background(), `SELECT secret_enc FROM user_totp WHERE user_id=$1`, r.uid).Scan(&enc); err != nil {
		t.Fatal(err)
	}
	if len(enc) == 0 || strings.Contains(string(enc), string(secret)) {
		t.Fatal("the stored secret must be ciphertext")
	}
	if _, err := decryptSecret([]byte("ffffffffffffffffffffffffffffffff"), enc); err == nil {
		t.Fatal("a wrong key must not decrypt the secret")
	}
	if got, err := decryptSecret(testTOTPKey(), enc); err != nil || string(got) != string(secret) {
		t.Fatalf("the right key must decrypt it (err=%v)", err)
	}
}

// TC-2FA-009
func TestTOTP_ConcurrentVerifyOneWins(t *testing.T) {
	r := newTOTPRig(t)
	ctx := context.Background()
	r.activate(t)
	code := r.codeAt(1)
	var ok atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.svc.VerifyTOTP(ctx, r.uid, code); err == nil {
				ok.Add(1)
			}
		}()
	}
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatalf("exactly one verification of the same code may succeed, got %d", ok.Load())
	}
}

var _ = uuid.Nil

// TC-2FA-009 (store level): the SQL guard alone lets exactly one of many parallel claims of the
// same time step win. The service-level tests above are also protected by the Go-side check of
// the last step, so this one proves the single-use guarantee on its own.
func TestTOTP_GuardedClaimHasOneWinner(t *testing.T) {
	r := newTOTPRig(t)
	ctx := context.Background()
	r.activate(t)
	step := time.Now().Unix()/30 + 5
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := r.svc.q.ClaimTOTPStep(ctx, store.ClaimTOTPStepParams{UserID: r.uid, LastStep: step})
			if err != nil {
				t.Error(err)
				return
			}
			if n == 1 {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("exactly one claim of a step may win, got %d", wins.Load())
	}
	if n, _ := r.svc.q.ClaimTOTPStep(ctx, store.ClaimTOTPStepParams{UserID: r.uid, LastStep: step}); n != 0 {
		t.Fatal("a claimed step must not be claimable again")
	}
	if n, _ := r.svc.q.ClaimTOTPStep(ctx, store.ClaimTOTPStepParams{UserID: r.uid, LastStep: step - 1}); n != 0 {
		t.Fatal("an older step must not be claimable after a newer one")
	}
}

// The secret must be stored as ciphertext: a stored value equal to the plain secret is a bug.
func TestTOTP_EncryptRoundTripAndTamper(t *testing.T) {
	key := testTOTPKey()
	plain := []byte("12345678901234567890")
	enc, err := encryptSecret(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(enc), string(plain)) {
		t.Fatal("ciphertext contains the plaintext")
	}
	if got, err := decryptSecret(key, enc); err != nil || string(got) != string(plain) {
		t.Fatalf("round trip failed: %v", err)
	}
	enc[len(enc)-1] ^= 0xff
	if _, err := decryptSecret(key, enc); err == nil {
		t.Fatal("a tampered ciphertext must not decrypt (GCM authenticates)")
	}
}

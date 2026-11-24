package authn

// ADR-0018 / spec w5-pin-pepper: PIN pepper, rehash-on-verify, rotation, leak property and logging,
// through the shared testkit and a real database. Cases TC-PEP-017 to TC-PEP-026.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

const pepperPW = "S3cret!password"

var newFormat = regexp.MustCompile(`^\$argon2id\$v=19\$m=19456,t=2,p=1,k=([a-z0-9-]+)\$[A-Za-z0-9+/]+\$[A-Za-z0-9+/]+$`)

func pepperKey(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func mustPeppers(t *testing.T, cur auth.PepperKey, old ...auth.PepperKey) *auth.PepperSet {
	t.Helper()
	ps, err := auth.NewPepperSet(cur, old...)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

// pepperUser creates a user with a known password and a LEGACY PIN hash (raw PIN, old parameters).
func pepperUser(t *testing.T, e *testkit.Env, legacyPIN string) uuid.UUID {
	t.Helper()
	uid, _, _ := e.UserWithMoney(t, 0)
	pw, err := auth.HashPassword(pepperPW)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pool.Exec(context.Background(), `UPDATE users SET password_hash=$2 WHERE id=$1`, uid, pw); err != nil {
		t.Fatal(err)
	}
	if legacyPIN != "" {
		setRaw(t, e, uid, mustLegacy(t, legacyPIN))
	}
	return uid
}

func mustLegacy(t *testing.T, pin string) string {
	t.Helper()
	h, err := auth.HashPassword(pin)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func setRaw(t *testing.T, e *testkit.Env, uid uuid.UUID, h string) {
	t.Helper()
	if _, err := e.Pool.Exec(context.Background(), `UPDATE users SET pin_hash=$2 WHERE id=$1`, uid, h); err != nil {
		t.Fatal(err)
	}
}

func pinHash(t *testing.T, e *testkit.Env, uid uuid.UUID) string {
	t.Helper()
	var h string
	if err := e.Pool.QueryRow(context.Background(), `SELECT pin_hash FROM users WHERE id=$1`, uid).Scan(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

func keyID(t *testing.T, h string) string {
	t.Helper()
	m := newFormat.FindStringSubmatch(h)
	if m == nil {
		t.Fatalf("hash is not in the ADR-0018 format: %q", h)
	}
	return m[1]
}

// TC-PEP-017
func TestPinRehash_LegacyVerifiedAndRewritten(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	uid := pepperUser(t, e, "482915")
	legacy := pinHash(t, e, uid)
	if strings.Contains(legacy, ",k=") {
		t.Fatal("setup: the starting hash must be legacy")
	}
	if err := e.Auth.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("a legacy hash must verify: %v", err)
	}
	after := pinHash(t, e, uid)
	if keyID(t, after) != "p1" {
		t.Fatalf("the row must hold the current key id after the same request, got %q", after)
	}
	if err := e.Auth.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("the rewritten hash must verify: %v", err)
	}
	if again := pinHash(t, e, uid); again != after {
		t.Fatal("a current hash must not be rewritten again")
	}
	if err := e.Auth.VerifyPIN(ctx, uid, "135790"); !errors.Is(err, auth.ErrPINIncorrect) {
		t.Fatalf("a wrong PIN must fail after the rewrite: %v", err)
	}
}

// TC-PEP-018
func TestPinRehash_WrongPinDoesNotRewrite(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	uid := pepperUser(t, e, "482915")
	legacy := pinHash(t, e, uid)
	if err := e.Auth.VerifyPIN(ctx, uid, "908172"); !errors.Is(err, auth.ErrPINIncorrect) {
		t.Fatalf("want ErrPINIncorrect, got %v", err)
	}
	if got := pinHash(t, e, uid); got != legacy {
		t.Fatalf("a failed verification must never rewrite, hash is now %q", got)
	}
	var n int
	if err := e.Pool.QueryRow(ctx, `SELECT pin_failed_attempts FROM users WHERE id=$1`, uid).Scan(&n); err != nil || n != 1 {
		t.Fatalf("the failure must be counted once, got %d (%v)", n, err)
	}
}

// TC-PEP-019
func TestPinRehash_ConcurrentVerifiesDoNotCorrupt(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	uid := pepperUser(t, e, "482915")
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- e.Auth.VerifyPIN(ctx, uid, "482915") }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("every concurrent verify must succeed: %v", err)
		}
	}
	h := pinHash(t, e, uid)
	keyID(t, h)
	if err := e.Auth.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("the row must hold one valid hash: %v", err)
	}
	if err := e.Auth.VerifyPIN(ctx, uid, "135790"); !errors.Is(err, auth.ErrPINIncorrect) {
		t.Fatalf("a wrong PIN must still fail: %v", err)
	}
}

// TC-PEP-022
func TestPinCreators_AllWriteTheNewFormat(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	uid := pepperUser(t, e, "")

	if err := e.Auth.SetPIN(ctx, uid, pepperPW, "482915"); err != nil {
		t.Fatal(err)
	}
	keyID(t, pinHash(t, e, uid))
	if err := e.Auth.ChangePIN(ctx, uid, "482915", "135790"); err != nil {
		t.Fatal(err)
	}
	keyID(t, pinHash(t, e, uid))

	// reset with a one-time code: the legacy starting point proves the reset writes the new format
	setRaw(t, e, uid, mustLegacy(t, "135790"))
	snd := &captureSender{}
	e.Auth.SetSender(snd)
	if _, err := e.Auth.RequestPINReset(ctx, uid, pepperPW); err != nil {
		t.Fatal(err)
	}
	if err := e.Auth.ConfirmPINReset(ctx, uid, snd.code(t), "908172"); err != nil {
		t.Fatal(err)
	}
	keyID(t, pinHash(t, e, uid))
	if err := e.Auth.VerifyPIN(ctx, uid, "908172"); err != nil {
		t.Fatal(err)
	}

	other, _, _ := e.UserWithMoney(t, 0)
	e.SetPIN(t, other, testkit.ValidPIN) // the shared test helper
	keyID(t, pinHash(t, e, other))
}

type captureSender struct {
	mu    sync.Mutex
	codes []string
}

func (c *captureSender) SendCode(_ context.Context, _ uuid.UUID, _, code string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.codes = append(c.codes, code)
	return nil
}
func (c *captureSender) Notify(context.Context, uuid.UUID, auth.Notice) error { return nil }
func (c *captureSender) code(t *testing.T) string {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.codes) == 0 {
		t.Fatal("no code sent")
	}
	return c.codes[len(c.codes)-1]
}

// TC-PEP-023
func TestPinRotation_OldKeyVerifiesAndRehashes(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	p1 := auth.PepperKey{ID: "p1", Key: pepperKey(1)}
	p2 := auth.PepperKey{ID: "p2", Key: pepperKey(2)}
	e.Auth.SetPINPepper(mustPeppers(t, p1))
	uid := pepperUser(t, e, "")
	if err := e.Auth.SetPIN(ctx, uid, pepperPW, "482915"); err != nil {
		t.Fatal(err)
	}
	if keyID(t, pinHash(t, e, uid)) != "p1" {
		t.Fatal("setup: hash must be under p1")
	}
	e.Auth.SetPINPepper(mustPeppers(t, p2, p1)) // rotation: p2 current, p1 kept
	if err := e.Auth.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("a hash under the old key must verify: %v", err)
	}
	if got := keyID(t, pinHash(t, e, uid)); got != "p2" {
		t.Fatalf("the row must move to p2, got %q", got)
	}
	e.Auth.SetPINPepper(mustPeppers(t, p2)) // p1 removed: the migrated user is unaffected
	if err := e.Auth.VerifyPIN(ctx, uid, "482915"); err != nil {
		t.Fatalf("a migrated user must verify after p1 is removed: %v", err)
	}
}

// TC-PEP-024
func TestPinRotation_RetiredKeyFailsCleanlyAndResetWorks(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	p1 := auth.PepperKey{ID: "p1", Key: pepperKey(1)}
	p2 := auth.PepperKey{ID: "p2", Key: pepperKey(2)}
	e.Auth.SetPINPepper(mustPeppers(t, p1))
	uid := pepperUser(t, e, "")
	if err := e.Auth.SetPIN(ctx, uid, pepperPW, "482915"); err != nil {
		t.Fatal(err)
	}
	before := pinHash(t, e, uid)
	e.Auth.SetPINPepper(mustPeppers(t, p2)) // p1 gone, user never migrated

	err := e.Auth.VerifyPIN(ctx, uid, "482915")
	if !errors.Is(err, auth.ErrPINIncorrect) {
		t.Fatalf("a retired key must look like a wrong PIN to the client, got %v", err)
	}
	if strings.Contains(err.Error(), "p1") || strings.Contains(strings.ToLower(err.Error()), "pepper") {
		t.Fatalf("the client error must not reveal why: %v", err)
	}
	if pinHash(t, e, uid) != before {
		t.Fatal("a failed verification must not rewrite")
	}

	snd := &captureSender{}
	e.Auth.SetSender(snd)
	if _, err := e.Auth.RequestPINReset(ctx, uid, pepperPW); err != nil {
		t.Fatalf("PIN reset must still work: %v", err)
	}
	if err := e.Auth.ConfirmPINReset(ctx, uid, snd.code(t), "908172"); err != nil {
		t.Fatalf("PIN reset confirm: %v", err)
	}
	if got := keyID(t, pinHash(t, e, uid)); got != "p2" {
		t.Fatalf("after the reset the row must be under p2, got %q", got)
	}
	if err := e.Auth.VerifyPIN(ctx, uid, "908172"); err != nil {
		t.Fatalf("the new PIN must verify: %v", err)
	}
}

// TC-PEP-025: a copy of the table is useless without the pepper.
func TestPinLeak_TableWithoutPepperCannotVerify(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	real := mustPeppers(t, auth.PepperKey{ID: "p1", Key: pepperKey(1)})
	e.Auth.SetPINPepper(real)
	uid := pepperUser(t, e, "")
	if err := e.Auth.SetPIN(ctx, uid, pepperPW, "482915"); err != nil {
		t.Fatal(err)
	}
	stolen := pinHash(t, e, uid) // what an attacker copies from the table

	wrong := mustPeppers(t, auth.PepperKey{ID: "p1", Key: pepperKey(9)})
	if err := wrong.VerifyPINHash(stolen, "482915"); err == nil {
		t.Fatal("the stolen hash verified the known PIN under a wrong pepper")
	}
	if err := auth.VerifyPassword(stolen, "482915"); err == nil {
		t.Fatal("the stolen hash verified the known PIN as a plain argon2 hash (no pepper)")
	}
	if err := real.VerifyPINHash(stolen, "482915"); err != nil {
		t.Fatalf("control: the real pepper must verify: %v", err)
	}
}

// TC-PEP-026: neither the PIN, the pepper nor the HMAC output reaches logs, errors or audit rows.
func TestPinLogging_NoSecretsInLogsErrorsOrAudit(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	var logs syncBuf
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	p1 := auth.PepperKey{ID: "p1", Key: pepperKey(1)}
	p2 := auth.PepperKey{ID: "p2", Key: pepperKey(2)}
	svc := auth.NewService(e.Pool, auth.NewJWTIssuer("integration-test-secret-0123456789", "user"), log, "dev")
	svc.SetTOTPKey([]byte("0123456789abcdef0123456789abcdef"))
	svc.SetPINPepper(mustPeppers(t, p1))
	snd := &captureSender{}
	svc.SetSender(snd)

	const pin, pin2, pin3 = "482915", "135790", "908172"
	uid := pepperUser(t, e, "")
	var errTexts []string
	rec := func(err error) {
		if err != nil {
			errTexts = append(errTexts, err.Error())
		}
	}

	rec(svc.SetPIN(ctx, uid, pepperPW, pin))
	rec(svc.VerifyPIN(ctx, uid, pin))      // success
	rec(svc.VerifyPIN(ctx, uid, "000001")) // failure
	// legacy rehash
	setRaw(t, e, uid, mustLegacy(t, pin))
	rec(svc.VerifyPIN(ctx, uid, pin))
	// rotation: p2 current, p1 old, then p1 removed with an unmigrated user
	svc.SetPINPepper(mustPeppers(t, p2, p1))
	rec(svc.ChangePIN(ctx, uid, pin, pin2))
	svc.SetPINPepper(mustPeppers(t, p1))
	rec(svc.SetPIN(ctx, uid, pepperPW, pin3))
	svc.SetPINPepper(mustPeppers(t, p2, p1))
	rec(svc.VerifyPIN(ctx, uid, pin3)) // old key verifies and rehashes
	svc.SetPINPepper(mustPeppers(t, p2))
	svc.SetPINPepper(mustPeppers(t, auth.PepperKey{ID: "p3", Key: pepperKey(3)}))
	rec(svc.VerifyPIN(ctx, uid, pin3)) // retired key: fails cleanly
	rec(svc.VerifyPIN(ctx, uid, pin3))
	rows, err := e.Pool.Query(ctx, `SELECT action, coalesce(reason,''), resource FROM audit_logs WHERE actor_id=$1`, uid)
	if err != nil {
		t.Fatal(err)
	}
	var audit []string
	for rows.Next() {
		var a, r, res string
		if err := rows.Scan(&a, &r, &res); err != nil {
			t.Fatal(err)
		}
		audit = append(audit, a+"|"+r+"|"+res)
	}
	rows.Close()
	if len(audit) == 0 {
		t.Fatal("expected audit rows to inspect")
	}

	haystack := logs.String() + "\n" + strings.Join(errTexts, "\n") + "\n" + strings.Join(audit, "\n")
	secrets := map[string][]byte{}
	for _, p := range []string{pin, pin2, pin3, "000001"} {
		secrets["PIN "+p] = []byte(p)
	}
	for _, k := range []auth.PepperKey{p1, p2, {ID: "p3", Key: pepperKey(3)}} {
		secrets["pepper "+k.ID] = k.Key
		for _, p := range []string{pin, pin2, pin3} {
			m := hmac.New(sha256.New, k.Key)
			m.Write([]byte(p))
			secrets[fmt.Sprintf("hmac %s %s", k.ID, p)] = m.Sum(nil)
		}
	}
	for name, raw := range secrets {
		for _, form := range []string{string(raw), hex.EncodeToString(raw), base64.StdEncoding.EncodeToString(raw),
			base64.RawStdEncoding.EncodeToString(raw), base64.URLEncoding.EncodeToString(raw)} {
			if strings.Contains(haystack, form) {
				t.Errorf("%s appears in logs, errors or audit rows (as %q)", name, form)
			}
		}
	}
	// the stored hashes themselves must not be logged either
	if strings.Contains(haystack, "$argon2id$") {
		t.Errorf("a hash appears in logs, errors or audit rows")
	}
	_ = time.Now
	_ = config.DevPINPepper
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

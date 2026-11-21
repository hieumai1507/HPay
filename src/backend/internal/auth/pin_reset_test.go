package auth

// ADR-0016 / spec w4-pin-reset: forgotten PIN reset. Cases TC-PRS-001..017, 020.
// Real database; the code delivery goes through a recording sender.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

// recordingSender captures what would be sent to the user.
type recordingSender struct {
	mu      sync.Mutex
	codes   []string
	notices []Notice
}

func (r *recordingSender) SendCode(_ context.Context, _ uuid.UUID, purpose, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codes = append(r.codes, purpose+":"+code)
	return nil
}
func (r *recordingSender) Notify(_ context.Context, _ uuid.UUID, n Notice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notices = append(r.notices, n)
	return nil
}
func (r *recordingSender) lastCode(t *testing.T) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.codes) == 0 {
		t.Fatal("no code was sent")
	}
	c := r.codes[len(r.codes)-1]
	return c[strings.Index(c, ":")+1:]
}
func (r *recordingSender) sentCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.codes)
}

type resetRig struct {
	svc    *Service
	uid    uuid.UUID
	pw     string
	sender *recordingSender
}

const startPIN = "482915"
const newPIN = "908172"

func newResetRig(t *testing.T) *resetRig {
	t.Helper()
	svc, uid, pw := newPinService(t)
	sender := &recordingSender{}
	svc.SetSender(sender)
	if err := svc.SetPIN(context.Background(), uid, pw, startPIN); err != nil {
		t.Fatal(err)
	}
	// the reset tests need to request more than once: skip the cooldown unless a test checks it
	return &resetRig{svc: svc, uid: uid, pw: pw, sender: sender}
}

func (r *resetRig) clearCooldown(t *testing.T) {
	t.Helper()
	if _, err := r.svc.pool.Exec(context.Background(), `UPDATE otp_challenges SET created_at = created_at - interval '1 minute' WHERE user_id=$1`, r.uid); err != nil {
		t.Fatal(err)
	}
}

func (r *resetRig) request(t *testing.T) string {
	t.Helper()
	if _, err := r.svc.RequestPINReset(context.Background(), r.uid, r.pw); err != nil {
		t.Fatalf("request reset: %v", err)
	}
	return r.sender.lastCode(t)
}

// TC-PRS-001
func TestPINReset_RequestSendsCodeNeverInResponse(t *testing.T) {
	r := newResetRig(t)
	out, err := r.svc.RequestPINReset(context.Background(), r.uid, r.pw)
	if err != nil {
		t.Fatal(err)
	}
	if r.sender.sentCount() != 1 {
		t.Fatalf("want exactly one code sent, got %d", r.sender.sentCount())
	}
	if out.DevCode != "" {
		t.Fatalf("the code must not be returned outside dev, got %q", out.DevCode)
	}
	if out.ExpiresIn <= 0 {
		t.Fatalf("expires_in must be positive, got %d", out.ExpiresIn)
	}
}

// TC-PRS-002
func TestPINReset_WrongPasswordSendsNothing(t *testing.T) {
	r := newResetRig(t)
	if _, err := r.svc.RequestPINReset(context.Background(), r.uid, "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	if r.sender.sentCount() != 0 {
		t.Fatal("nothing may be sent for a wrong password")
	}
	var n int
	if err := r.svc.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND action='pin.reset_refused'`, r.uid).Scan(&n); err != nil || n != 1 {
		t.Fatalf("want one pin.reset_refused audit row, got %d (%v)", n, err)
	}
}

// TC-PRS-003
func TestPINReset_ConfirmReplacesPIN(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	code := r.request(t)
	if err := r.svc.ConfirmPINReset(ctx, r.uid, code, newPIN); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := r.svc.VerifyPIN(ctx, r.uid, newPIN); err != nil {
		t.Fatalf("the new PIN must verify: %v", err)
	}
	if err := r.svc.VerifyPIN(ctx, r.uid, startPIN); !errors.Is(err, ErrPINIncorrect) {
		t.Fatalf("the old PIN must not verify, got %v", err)
	}
	var n int
	if err := r.svc.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND action='pin.reset'`, r.uid).Scan(&n); err != nil || n != 1 {
		t.Fatalf("want one pin.reset audit row, got %d (%v)", n, err)
	}
}

// TC-PRS-004
func TestPINReset_CodeIsSingleUse(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	code := r.request(t)
	if err := r.svc.ConfirmPINReset(ctx, r.uid, code, newPIN); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.ConfirmPINReset(ctx, r.uid, code, "135790"); err == nil {
		t.Fatal("a used code must not work again")
	}
	if err := r.svc.VerifyPIN(ctx, r.uid, newPIN); err != nil {
		t.Fatalf("the PIN from the first confirm must still stand: %v", err)
	}
}

// TC-PRS-005
func TestPINReset_ThreeWrongCodesExhaust(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	code := r.request(t)
	wrong := "000000"
	if code == wrong {
		wrong = "999999"
	}
	for i := 0; i < 3; i++ {
		if err := r.svc.ConfirmPINReset(ctx, r.uid, wrong, newPIN); err == nil {
			t.Fatalf("wrong code %d was accepted", i+1)
		}
	}
	if err := r.svc.ConfirmPINReset(ctx, r.uid, code, newPIN); err == nil {
		t.Fatal("the correct code must fail once three wrong attempts exhausted it")
	}
	if err := r.svc.VerifyPIN(ctx, r.uid, startPIN); err != nil {
		t.Fatalf("the PIN must be unchanged: %v", err)
	}
}

// TC-PRS-006
func TestPINReset_ExpiredCodeRefused(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	code := r.request(t)
	if _, err := r.svc.pool.Exec(ctx, `UPDATE otp_challenges SET expires_at = now() - interval '1 second' WHERE user_id=$1`, r.uid); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.ConfirmPINReset(ctx, r.uid, code, newPIN); !errors.Is(err, ErrOTPExpired) {
		t.Fatalf("want ErrOTPExpired, got %v", err)
	}
}

// TC-PRS-007
func TestPINReset_WeakPINDoesNotConsumeCode(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	code := r.request(t)
	for _, weak := range []string{"111111", "123456", "12ab56", "12345"} {
		if err := r.svc.ConfirmPINReset(ctx, r.uid, code, weak); !errors.Is(err, ErrWeakPIN) {
			t.Fatalf("weak PIN %q: want ErrWeakPIN, got %v", weak, err)
		}
	}
	if err := r.svc.ConfirmPINReset(ctx, r.uid, code, newPIN); err != nil {
		t.Fatalf("the code must still work after weak PINs were refused: %v", err)
	}
}

// TC-PRS-008
func TestPINReset_RequestCooldownAndHourlyCap(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	if _, err := r.svc.RequestPINReset(ctx, r.uid, r.pw); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.RequestPINReset(ctx, r.uid, r.pw); !errors.Is(err, ErrOTPCooldown) {
		t.Fatalf("an immediate second request: want ErrOTPCooldown, got %v", err)
	}
	for i := 0; i < 2; i++ {
		r.clearCooldown(t)
		if _, err := r.svc.RequestPINReset(ctx, r.uid, r.pw); err != nil {
			t.Fatalf("request %d: %v", i+2, err)
		}
	}
	r.clearCooldown(t)
	if _, err := r.svc.RequestPINReset(ctx, r.uid, r.pw); !errors.Is(err, ErrPINResetRateLimited) {
		t.Fatalf("the 4th request within an hour: want ErrPINResetRateLimited, got %v", err)
	}
}

// TC-PRS-009
func TestPINReset_ClearsLockout(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	for i := 0; i < pinMaxFailures; i++ {
		_ = r.svc.VerifyPIN(ctx, r.uid, "000000")
	}
	if err := r.svc.VerifyPIN(ctx, r.uid, startPIN); !errors.Is(err, ErrPINLocked) {
		t.Fatalf("setup: the PIN must be locked, got %v", err)
	}
	code := r.request(t)
	if err := r.svc.ConfirmPINReset(ctx, r.uid, code, newPIN); err != nil {
		t.Fatalf("a locked user must still be able to reset: %v", err)
	}
	if err := r.svc.VerifyPIN(ctx, r.uid, newPIN); err != nil {
		t.Fatalf("the lock must be cleared: %v", err)
	}
}

// TC-PRS-010
func TestPINReset_CoolingPeriodLimitsLargeTransactions(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	if err := r.svc.CheckPINCooling(ctx, r.uid, PINCoolingCap+1); err != nil {
		t.Fatalf("before any reset a large amount is fine: %v", err)
	}
	code := r.request(t)
	if err := r.svc.ConfirmPINReset(ctx, r.uid, code, newPIN); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.CheckPINCooling(ctx, r.uid, PINCoolingCap+1); !errors.Is(err, ErrPINCooling) {
		t.Fatalf("a large amount right after a reset: want ErrPINCooling, got %v", err)
	}
	if err := r.svc.CheckPINCooling(ctx, r.uid, PINCoolingCap); err != nil {
		t.Fatalf("an amount at the cap is allowed: %v", err)
	}
	if _, err := r.svc.pool.Exec(ctx, `UPDATE users SET pin_reset_at = now() - interval '25 hours' WHERE id=$1`, r.uid); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.CheckPINCooling(ctx, r.uid, PINCoolingCap+1); err != nil {
		t.Fatalf("after the cooling window a large amount is allowed: %v", err)
	}
}

// TC-PRS-011
func TestPINReset_UserIsNotified(t *testing.T) {
	r := newResetRig(t)
	code := r.request(t)
	if err := r.svc.ConfirmPINReset(context.Background(), r.uid, code, newPIN); err != nil {
		t.Fatal(err)
	}
	r.sender.mu.Lock()
	defer r.sender.mu.Unlock()
	if len(r.sender.notices) != 1 || r.sender.notices[0].Kind != NoticePINReset {
		t.Fatalf("want one pin_reset notice, got %+v", r.sender.notices)
	}
	for _, n := range r.sender.notices {
		if strings.Contains(n.Text, code) || strings.Contains(n.Text, newPIN) {
			t.Fatalf("the notice leaks a secret: %q", n.Text)
		}
	}
}

// TC-PRS-015
func TestPINReset_NoPINRefused(t *testing.T) {
	svc, uid, pw := newPinService(t) // no PIN set
	svc.SetSender(&recordingSender{})
	if _, err := svc.RequestPINReset(context.Background(), uid, pw); !errors.Is(err, ErrPINNotSet) {
		t.Fatalf("want ErrPINNotSet, got %v", err)
	}
}

// TC-PRS-016
func TestPINReset_ConcurrentConfirmsOneWins(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	code := r.request(t)
	var ok, fail atomic.Int32
	var wg sync.WaitGroup
	pins := []string{"908172", "135790", "246801", "357913", "468024", "579135", "680246", "791357"}
	for _, p := range pins {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			if err := r.svc.ConfirmPINReset(ctx, r.uid, code, p); err == nil {
				ok.Add(1)
			} else {
				fail.Add(1)
			}
		}(p)
	}
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatalf("exactly one confirm may win, got %d (failures %d)", ok.Load(), fail.Load())
	}
}

// TC-PRS-017
func TestPINReset_CodeBoundToUser(t *testing.T) {
	a := newResetRig(t)
	b := newResetRig(t)
	code := a.request(t)
	if err := b.svc.ConfirmPINReset(context.Background(), b.uid, code, newPIN); err == nil {
		t.Fatal("user B must not be able to use the code sent to user A")
	}
	if err := b.svc.VerifyPIN(context.Background(), b.uid, startPIN); err != nil {
		t.Fatalf("user B's PIN must be unchanged: %v", err)
	}
}

// TC-PRS-020
func TestOTPSender_ProductionRefuses(t *testing.T) {
	s := NewSender("production", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := s.SendCode(context.Background(), uuid.New(), PurposePINReset, "123456"); err == nil {
		t.Fatal("the production sender must refuse until a real provider is configured")
	}
	if err := s.Notify(context.Background(), uuid.New(), Notice{Kind: NoticePINReset, Text: "x"}); err == nil {
		t.Fatal("the production sender must refuse notices too (fail closed)")
	}
	_ = time.Second
	_ = testutil.PoolFromEnv
}

// TC-PRS-016 (store level): the guarded consume lets exactly one of many parallel callers win.
// The service-level test above is also protected by the attempt counter (8 parallel confirms
// exhaust a code that allows 3 tries), so this one proves the single-use guarantee on its own.
func TestPINReset_GuardedConsumeHasOneWinner(t *testing.T) {
	r := newResetRig(t)
	ctx := context.Background()
	r.request(t)
	ch, err := r.svc.q.GetActiveOTP(ctx, store.GetActiveOTPParams{UserID: r.uid, Purpose: PurposePINReset})
	if err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := r.svc.q.ConsumeOTPIfActive(ctx, ch.ID)
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
		t.Fatalf("exactly one consume may win, got %d", wins.Load())
	}
	if n, _ := r.svc.q.ConsumeOTPIfActive(ctx, ch.ID); n != 0 {
		t.Fatal("a consumed code must not be consumable again")
	}
}

package authn

// ADR-0016: PIN reset through the real router and the cooling cap on a real transaction.
// Cases TC-PRS-010 (end to end), 012, 013, 018, 019.

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// userWithPassword creates a user with a known password and a PIN, and a user-audience token.
func userWithPassword(t *testing.T, rig *testkit.AdminRig, pin string) (uuid.UUID, string, string) {
	t.Helper()
	uid, _, _ := rig.E.UserWithMoney(t, 0)
	pw := "S3cret!password"
	h, err := auth.HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rig.E.Pool.Exec(context.Background(), `UPDATE users SET password_hash=$2 WHERE id=$1`, uid, h); err != nil {
		t.Fatal(err)
	}
	if pin != "" {
		rig.E.SetPIN(t, uid, pin)
	}
	tok, err := auth.NewJWTIssuer(testkit.RBACSecret, "user").Issue(uid, "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return uid, pw, tok
}

func clearOTPCooldown(t *testing.T, rig *testkit.AdminRig, uid uuid.UUID) {
	t.Helper()
	if _, err := rig.E.Pool.Exec(context.Background(), `UPDATE otp_challenges SET created_at = created_at - interval '1 minute' WHERE user_id=$1`, uid); err != nil {
		t.Fatal(err)
	}
}

// TC-PRS-018
func TestPINResetHTTP_FlowAndErrorCodes(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	uid, pw, tok := userWithPassword(t, rig, testkit.ValidPIN)

	// no token
	if w := rig.Call("POST", "/v1/me/pin/reset/request", "", map[string]any{"password": pw}); w.Code != http.StatusUnauthorized {
		t.Fatalf("without a token: %d", w.Code)
	}
	// missing body
	if w := rig.Call("POST", "/v1/me/pin/reset/request", tok, map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing password: %d", w.Code)
	}
	// wrong password
	if w := rig.Call("POST", "/v1/me/pin/reset/request", tok, map[string]any{"password": "nope"}); w.Code != http.StatusUnauthorized || testkit.ErrCode(w) != "ERR_INVALID_CREDENTIALS" {
		t.Fatalf("wrong password: %d %s", w.Code, testkit.ErrCode(w))
	}
	// good request: in the ci environment the code is NOT in the body
	w := rig.Call("POST", "/v1/me/pin/reset/request", tok, map[string]any{"password": pw})
	if w.Code != http.StatusOK || testkit.Contains(w.Body.String(), "otp_dev") {
		t.Fatalf("request: %d %s (the code must not be in the response outside dev)", w.Code, w.Body.String())
	}
	// second request right away: cooldown
	if w := rig.Call("POST", "/v1/me/pin/reset/request", tok, map[string]any{"password": pw}); w.Code != http.StatusTooManyRequests || testkit.ErrCode(w) != "ERR_OTP_COOLDOWN" {
		t.Fatalf("cooldown: %d %s", w.Code, testkit.ErrCode(w))
	}
	// confirm with a wrong code
	if w := rig.Call("POST", "/v1/me/pin/reset/confirm", tok, map[string]any{"code": "000000", "new_pin": "908172"}); w.Code != http.StatusBadRequest || testkit.ErrCode(w) != "ERR_OTP_INVALID" {
		t.Fatalf("wrong code: %d %s", w.Code, testkit.ErrCode(w))
	}
	// weak PIN
	if w := rig.Call("POST", "/v1/me/pin/reset/confirm", tok, map[string]any{"code": "000000", "new_pin": "111111"}); w.Code != http.StatusBadRequest || testkit.ErrCode(w) != "ERR_PIN_WEAK" {
		t.Fatalf("weak PIN: %d %s", w.Code, testkit.ErrCode(w))
	}
	_ = uid
}

// TC-PRS-010 end to end: the cooling cap applies to a real PIN transaction after a reset.
func TestPINReset_CoolingCapAppliesToConfirm(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	e := rig.E
	sender, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, sender, "OFFER_CUST_L2")
	testkit.Fund(t, e.Pool, sender, mustWallet(t, e, sender), 40_000_000)
	e.SetPIN(t, sender, testkit.ValidPIN)
	_, _, rcv := e.UserWithMoney(t, 0)
	if _, err := e.Pool.Exec(context.Background(), `UPDATE transactions SET created_at = created_at - interval '1 hour' WHERE initiated_by=$1`, sender); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pool.Exec(context.Background(), `UPDATE users SET pin_reset_at = now() WHERE id=$1`, sender); err != nil {
		t.Fatal(err)
	}
	// a request above the cap right after a reset is refused at request time
	_, err := e.Svc.ExecuteEngineRequest(context.Background(), transfer.EngineRequestInput{
		ServiceCode: "P2P_TRANSFER", SenderUserID: sender, ReceiverPhone: rcv, Amount: auth.PINCoolingCap + 1_000, Idempotency: uuid.NewString(),
	})
	if !errors.Is(err, auth.ErrPINCooling) {
		t.Fatalf("a PIN transaction above the cap right after a reset: want ErrPINCooling, got %v", err)
	}
	// a small one goes through
	if _, err := e.Svc.ExecuteEngineRequest(context.Background(), transfer.EngineRequestInput{
		ServiceCode: "P2P_TRANSFER", SenderUserID: sender, ReceiverPhone: rcv, Amount: 100_000, Idempotency: uuid.NewString(),
	}); err != nil {
		t.Fatalf("a small PIN transaction must still work: %v", err)
	}
}

func mustWallet(t *testing.T, e *testkit.Env, user uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.Pool.QueryRow(context.Background(), `SELECT id FROM wallets WHERE owner_id=$1 AND purpose='main'`, user).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// TC-PRS-012, 019
func TestPINResetHTTP_AdminRoute(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	target, _, _ := userWithPassword(t, rig, testkit.ValidPIN)
	adminID, tok := rig.NewAdmin("superadmin", "active")
	path := "/v1/admin/users/" + target.String() + "/pin/reset"

	if w := rig.Call("POST", path, tok, map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing reason: %d", w.Code)
	}
	if w := rig.Call("POST", path, tok, map[string]any{"reason": "Identity verified by video call, ticket 4711"}); w.Code != http.StatusOK {
		t.Fatalf("clear: %d %s", w.Code, w.Body.String())
	}
	var hasPIN bool
	if err := rig.E.Pool.QueryRow(context.Background(), `SELECT pin_hash IS NOT NULL FROM users WHERE id=$1`, target).Scan(&hasPIN); err != nil || hasPIN {
		t.Fatalf("the PIN must be cleared (hasPIN=%v err=%v)", hasPIN, err)
	}
	var n int
	if err := rig.E.Pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND actor_type='admin' AND action='pin.clear' AND reason LIKE '%ticket 4711%'`, adminID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("want one pin.clear audit row with the reason, got %d (%v)", n, err)
	}
	// a second clear: the user has no PIN any more
	if w := rig.Call("POST", path, tok, map[string]any{"reason": "again for the test"}); w.Code != http.StatusForbidden && w.Code != http.StatusConflict {
		t.Fatalf("clearing a user without a PIN: %d", w.Code)
	}
	// unknown user
	if w := rig.Call("POST", "/v1/admin/users/"+uuid.NewString()+"/pin/reset", tok, map[string]any{"reason": "unknown user test"}); w.Code == http.StatusOK {
		t.Fatalf("an unknown user must not return 200")
	}
	// bad id
	if w := rig.Call("POST", "/v1/admin/users/not-a-uuid/pin/reset", tok, map[string]any{"reason": "bad id for the test"}); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", w.Code)
	}
}

// TC-PRS-013: every role except superadmin is refused.
func TestAdminClearPIN_OnlySuperadmin(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	target, _, _ := userWithPassword(t, rig, testkit.ValidPIN)
	path := "/v1/admin/users/" + target.String() + "/pin/reset"
	for _, role := range []string{"viewer", "kyc_reviewer", "risk_officer", "ops"} {
		_, tok := rig.NewAdmin(role, "active")
		w := rig.Call("POST", path, tok, map[string]any{"reason": "should be refused for " + role})
		if w.Code != http.StatusForbidden || testkit.ErrCode(w) != "ERR_FORBIDDEN_PERMISSION" {
			t.Errorf("%s: want 403 ERR_FORBIDDEN_PERMISSION, got %d %s", role, w.Code, testkit.ErrCode(w))
		}
	}
	var hasPIN bool
	if err := rig.E.Pool.QueryRow(context.Background(), `SELECT pin_hash IS NOT NULL FROM users WHERE id=$1`, target).Scan(&hasPIN); err != nil || !hasPIN {
		t.Fatalf("the PIN must be untouched after the refusals (hasPIN=%v err=%v)", hasPIN, err)
	}
}

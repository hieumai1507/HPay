package authn

// ADR-0017: TOTP routes and the operator reset through the real router. Cases TC-2FA-016..018.

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TC-2FA-018
func TestTOTPHTTP_EnrolConfirmAndStatus(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, pw, tok := userWithPassword(t, rig, testkit.ValidPIN)

	if w := rig.Call("POST", "/v1/me/totp/enroll", "", map[string]any{"password": pw}); w.Code != http.StatusUnauthorized {
		t.Fatalf("without a token: %d", w.Code)
	}
	if w := rig.Call("POST", "/v1/me/totp/enroll", tok, map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing password: %d", w.Code)
	}
	if w := rig.Call("POST", "/v1/me/totp/enroll", tok, map[string]any{"password": "nope"}); w.Code != http.StatusUnauthorized || testkit.ErrCode(w) != "ERR_INVALID_CREDENTIALS" {
		t.Fatalf("wrong password: %d %s", w.Code, testkit.ErrCode(w))
	}
	w := rig.Call("POST", "/v1/me/totp/enroll", tok, map[string]any{"password": pw})
	if w.Code != http.StatusOK || !testkit.Contains(w.Body.String(), "otpauth://totp/") {
		t.Fatalf("enrol: %d %s", w.Code, w.Body.String())
	}
	// the status endpoint never shows the secret
	st := rig.Call("GET", "/v1/me/totp", tok, nil)
	if st.Code != http.StatusOK || testkit.Contains(st.Body.String(), "secret") || testkit.Contains(st.Body.String(), "otpauth") {
		t.Fatalf("status must not leak the secret: %d %s", st.Code, st.Body.String())
	}
	if w := rig.Call("POST", "/v1/me/totp/confirm", tok, map[string]any{"code": "000000"}); w.Code != http.StatusBadRequest || testkit.ErrCode(w) != "ERR_TOTP_INVALID" {
		t.Fatalf("wrong code: %d %s", w.Code, testkit.ErrCode(w))
	}
	if w := rig.Call("POST", "/v1/me/totp/confirm", tok, map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing code: %d", w.Code)
	}
}

// TC-2FA-016, 017
func TestTOTPAdminReset_RemovesAndAudits(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	target, _, _ := userWithPassword(t, rig, testkit.ValidPIN)
	enrolTOTP(t, rig.E, target)
	adminID, tok := rig.NewAdmin("superadmin", "active")
	path := "/v1/admin/users/" + target.String() + "/totp/reset"

	if w := rig.Call("POST", path, tok, map[string]any{}); w.Code != http.StatusBadRequest {
		t.Fatalf("missing reason: %d", w.Code)
	}
	if w := rig.Call("POST", path, tok, map[string]any{"reason": "Lost the phone, identity checked by video call ticket 88"}); w.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
	var rows, codes int
	ctx := context.Background()
	_ = rig.E.Pool.QueryRow(ctx, `SELECT count(*) FROM user_totp WHERE user_id=$1`, target).Scan(&rows)
	_ = rig.E.Pool.QueryRow(ctx, `SELECT count(*) FROM user_recovery_codes WHERE user_id=$1`, target).Scan(&codes)
	if rows != 0 || codes != 0 {
		t.Fatalf("TOTP and recovery codes must be removed (rows=%d codes=%d)", rows, codes)
	}
	var n int
	if err := rig.E.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE actor_id=$1 AND actor_type='admin' AND action='totp.reset' AND reason LIKE '%ticket 88%'`, adminID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("want one totp.reset audit row with the reason, got %d (%v)", n, err)
	}
	// nothing secret in the audit trail
	// only the TOTP rows of this user and this admin: other tests write unrelated audit rows
	// (for example an AI key audit says "secret_replaced=true") into the same database
	var leaked int
	_ = rig.E.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs
		WHERE (action LIKE 'totp.%' AND (resource LIKE $2 OR actor_id = $1 OR actor_id = $3))
		  AND (resource ~* 'secret|otpauth' OR coalesce(reason,'') ~* 'secret|otpauth|[A-Z2-7]{16,}')`,
		target, "%"+target.String()+"%", adminID).Scan(&leaked)
	_ = auth.PermTOTPReset
	_ = time.Second
	_ = uuid.Nil
	if leaked != 0 {
		t.Fatal("the audit trail must not mention secrets")
	}
}

func TestTOTPAdminReset_OnlySuperadmin(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	target, _, _ := userWithPassword(t, rig, testkit.ValidPIN)
	enrolTOTP(t, rig.E, target)
	path := "/v1/admin/users/" + target.String() + "/totp/reset"
	for _, role := range []string{"viewer", "kyc_reviewer", "risk_officer", "ops"} {
		_, tok := rig.NewAdmin(role, "active")
		w := rig.Call("POST", path, tok, map[string]any{"reason": "should be refused for " + role})
		if w.Code != http.StatusForbidden || testkit.ErrCode(w) != "ERR_FORBIDDEN_PERMISSION" {
			t.Errorf("%s: want 403 ERR_FORBIDDEN_PERMISSION, got %d %s", role, w.Code, testkit.ErrCode(w))
		}
	}
	var enabled bool
	if err := rig.E.Pool.QueryRow(context.Background(), `SELECT enabled FROM user_totp WHERE user_id=$1`, target).Scan(&enabled); err != nil || !enabled {
		t.Fatalf("TOTP must be untouched after the refusals (enabled=%v err=%v)", enabled, err)
	}
}

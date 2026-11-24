package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// route is one admin endpoint and the roles that may call it.
type route struct {
	method, path string
	allowed      []string // roles allowed; everyone else must get 403
}

var all5 = []string{"viewer", "kyc_reviewer", "risk_officer", "ops", "superadmin"}

var anyPerm = []string{"kyc_reviewer", "risk_officer", "ops", "superadmin"} // every role that holds at least one permission

// The table below is the contract of docs/specs/w3-admin-rbac/spec.md.
// Paths use ids that do not exist: an allowed role then gets a 4xx other than
// 401/403 (404, 400), which proves the permission check passed.
var adminRoutes = []route{
	{"GET", "/v1/admin/kyc", []string{"kyc_reviewer", "superadmin"}},
	{"POST", "/v1/admin/kyc/" + zeroID + "/review", []string{"kyc_reviewer", "superadmin"}},
	{"GET", "/v1/admin/fraud-flags", []string{"risk_officer", "superadmin"}},
	{"POST", "/v1/admin/fraud-flags/" + zeroID + "/resolve", []string{"risk_officer", "superadmin"}},
	{"POST", "/v1/admin/reversals", []string{"risk_officer", "ops", "superadmin"}},
	{"POST", "/v1/admin/users/" + zeroID + "/pin/reset", []string{"superadmin"}},  // ADR-0016
	{"POST", "/v1/admin/users/" + zeroID + "/totp/reset", []string{"superadmin"}}, // ADR-0017
	{"GET", "/v1/admin/approvals", []string{"risk_officer", "ops", "superadmin"}},
	{"POST", "/v1/admin/approvals/" + zeroID + "/decide", []string{"risk_officer", "ops", "superadmin"}},
	{"POST", "/v1/admin/recon/run", []string{"ops", "superadmin"}},
	{"GET", "/v1/admin/recon/runs/2026-01-01", []string{"ops", "superadmin"}},
	{"GET", "/v1/admin/attention/summary", []string{"risk_officer", "ops", "superadmin"}}, // G12
	{"GET", "/v1/admin/workflows", []string{"risk_officer", "ops", "superadmin"}},
	{"GET", "/v1/admin/workflows/" + zeroID, []string{"risk_officer", "ops", "superadmin"}},
	{"GET", "/v1/admin/orgs", []string{"risk_officer", "ops", "superadmin"}},
	{"GET", "/v1/admin/orgs/" + zeroID + "/policies", []string{"risk_officer", "ops", "superadmin"}},
	{"PUT", "/v1/admin/orgs/" + zeroID + "/policies", []string{"superadmin"}},
	{"GET", "/v1/admin/services", anyPerm},
	{"GET", "/v1/admin/services/" + zeroID, anyPerm},
	{"PUT", "/v1/admin/services/" + zeroID, []string{"superadmin"}},
	{"GET", "/v1/admin/offers", anyPerm},
	{"GET", "/v1/admin/offers/" + zeroID, anyPerm},
	{"GET", "/v1/admin/audit", anyPerm},
	{"GET", "/v1/admin/ai/keys", []string{"superadmin"}},
	{"POST", "/v1/admin/ai/keys", []string{"superadmin"}},
	{"PUT", "/v1/admin/ai/keys/" + zeroID, []string{"superadmin"}},
	{"GET", "/v1/admin/ai/settings", []string{"superadmin"}},
	{"PUT", "/v1/admin/ai/settings", []string{"superadmin"}},
}

const zeroID = "00000000-0000-0000-0000-000000000000"

func roleIn(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// TC-RBA-001: a viewer holds no permission, so every admin route refuses it.
func TestAdminRBAC_ViewerIsDeniedEverywhere(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("viewer", "active")
	for _, rt := range adminRoutes {
		w := rig.Call(rt.method, rt.path, tok, map[string]any{})
		if w.Code != http.StatusForbidden || testkit.ErrCode(w) != "ERR_FORBIDDEN_PERMISSION" {
			t.Errorf("%s %s as viewer: got %d %q, want 403 ERR_FORBIDDEN_PERMISSION", rt.method, rt.path, w.Code, testkit.ErrCode(w))
		}
		if strings.Contains(w.Body.String(), "kyc.review") || strings.Contains(w.Body.String(), "superadmin") {
			t.Errorf("%s %s: the denial must not reveal the permission model: %s", rt.method, rt.path, w.Body.String())
		}
	}
}

// TC-RBA-002: for each role, 403 exactly where the table says so.
func TestAdminRBAC_RoleMatrix(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	for _, role := range all5 {
		_, tok := rig.NewAdmin(role, "active")
		for _, rt := range adminRoutes {
			w := rig.Call(rt.method, rt.path, tok, map[string]any{})
			denied := w.Code == http.StatusForbidden && testkit.ErrCode(w) == "ERR_FORBIDDEN_PERMISSION"
			if roleIn(rt.allowed, role) && denied {
				t.Errorf("%s %s as %s: denied but the role has the permission (status %d)", rt.method, rt.path, role, w.Code)
			}
			if !roleIn(rt.allowed, role) && !denied {
				t.Errorf("%s %s as %s: allowed (status %d) but the role lacks the permission", rt.method, rt.path, role, w.Code)
			}
			if w.Code == http.StatusUnauthorized {
				t.Errorf("%s %s as %s: unexpected 401", rt.method, rt.path, role)
			}
		}
	}
}

// TC-RBA-003: locking an admin takes effect on the very next request.
func TestAdminRBAC_LockedAdminRefusedImmediately(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	id, tok := rig.NewAdmin("superadmin", "active")
	if w := rig.Call("GET", "/v1/admin/audit", tok, nil); w.Code != http.StatusOK {
		t.Fatalf("setup: active superadmin should read the audit log, got %d", w.Code)
	}
	if _, err := rig.E.Pool.Exec(context.Background(), `UPDATE admin_users SET status='locked' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	w := rig.Call("GET", "/v1/admin/audit", tok, nil)
	if w.Code != http.StatusUnauthorized || testkit.ErrCode(w) != "ERR_ACCOUNT_LOCKED" {
		t.Fatalf("locked admin: got %d %q, want 401 ERR_ACCOUNT_LOCKED", w.Code, testkit.ErrCode(w))
	}
}

// TC-RBA-004: the role is read from the database, not from the token.
func TestAdminRBAC_RoleChangeTakesEffect(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	id, tok := rig.NewAdmin("ops", "active")
	if w := rig.Call("POST", "/v1/admin/recon/run", tok, map[string]any{}); w.Code == http.StatusForbidden {
		t.Fatalf("setup: ops may run recon, got 403")
	}
	if _, err := rig.E.Pool.Exec(context.Background(), `UPDATE admin_users SET role='viewer' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if w := rig.Call("POST", "/v1/admin/recon/run", tok, map[string]any{}); w.Code != http.StatusForbidden {
		t.Fatalf("after demotion to viewer: got %d, want 403", w.Code)
	}
}

// TC-RBA-005: a valid token for an admin that no longer exists is refused.
func TestAdminRBAC_DeletedAdminRefused(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	tok, err := rig.Admin.Issue(uuid.New(), "ghost", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w := rig.Call("GET", "/v1/admin/audit", tok, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown admin id: got %d, want 401", w.Code)
	}
}

// TC-RBA-008: a user-audience token still cannot call admin routes.
func TestAdminRBAC_UserTokenRefused(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	uid, _, _ := rig.E.UserWithMoney(t, 0)
	tok, err := auth.NewJWTIssuer(testkit.RBACSecret, "user").Issue(uid, "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w := rig.Call("GET", "/v1/admin/audit", tok, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("user token on an admin route: got %d, want 401", w.Code)
	}
}

// TC-RBA-010: every route the router registers under /v1/admin is in the table above,
// so a new admin route cannot ship without a permission decision.
func TestEveryAdminRouteHasAPermission(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	known := map[string]bool{}
	for _, rt := range adminRoutes {
		known[rt.method+" "+normalizeAdminPath(rt.path)] = true
	}
	for _, ri := range rig.Engine.Routes() {
		if !strings.HasPrefix(ri.Path, "/v1/admin/") || ri.Path == "/v1/admin/auth/login" {
			continue // not an admin route, or public by design (it issues the token)
		}
		if !known[ri.Method+" "+normalizeAdminPath(ri.Path)] {
			t.Errorf("admin route %s %s has no entry in the RBAC table (decide its permission)", ri.Method, ri.Path)
		}
	}
}

func normalizeAdminPath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		if strings.HasPrefix(s, ":") || s == zeroID || len(s) == 36 || strings.HasPrefix(s, "2026-") {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

// TC-RBA-006: AI key and setting writes are audited with the actor, and no key
// material ever reaches the audit trail or the response.
func TestAdminAI_WritesAreAudited(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	ctx := context.Background()
	adminID, tok := rig.NewAdmin("superadmin", "active")
	const secret = "sk-test-SECRET-key-material-0123456789"

	w := rig.Call("POST", "/v1/admin/ai/keys", tok, map[string]any{"name": "rbac key " + uuid.NewString()[:6], "provider": "anthropic", "api_key": secret})
	if w.Code != http.StatusOK {
		t.Fatalf("create key: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), secret) {
		t.Fatal("the create response contains the API key")
	}
	var created struct {
		ID uuid.UUID `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	w = rig.Call("PUT", "/v1/admin/ai/keys/"+created.ID.String(), tok, map[string]any{"name": "renamed", "is_active": true, "api_key": secret + "-v2"})
	if w.Code != http.StatusOK {
		t.Fatalf("update key: %d %s", w.Code, w.Body.String())
	}
	w = rig.Call("PUT", "/v1/admin/ai/settings", tok, map[string]any{"function_name": "fraud_advising", "provider": "mock", "model_name": "mock"})
	if w.Code != http.StatusOK {
		t.Fatalf("update setting: %d %s", w.Code, w.Body.String())
	}

	rows, err := rig.E.Pool.Query(ctx, `SELECT action, resource, coalesce(reason,''), coalesce(before::text,''), coalesce(after::text,'') FROM audit_logs WHERE actor_id=$1 AND actor_type='admin' AND action LIKE 'ai.%'`, adminID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var action, resource, reason, before, after string
		if err := rows.Scan(&action, &resource, &reason, &before, &after); err != nil {
			t.Fatal(err)
		}
		seen[action] = true
		for _, col := range []string{resource, reason, before, after} {
			if strings.Contains(col, secret) {
				t.Errorf("audit row %s contains key material", action)
			}
		}
	}
	for _, want := range []string{"ai.key.create", "ai.key.update", "ai.setting.update"} {
		if !seen[want] {
			t.Errorf("no audit row %q for this admin", want)
		}
	}
}

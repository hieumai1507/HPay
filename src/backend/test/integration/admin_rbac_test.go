package integration

// Wave 3 slice: admin RBAC (BE-H6, ADR-0015). Cases map to
// docs/specs/w3-admin-rbac/test-cases.md (TC-RBA-001..010).
//
// These tests drive the REAL router (api.NewRouter) so they exercise the actual
// middleware order and route table, not hand-built handlers.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/hieumai1507/hpay-backend/internal/api"
	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

const rbacSecret = "rbac-test-secret-0123456789-0123456789-abcdef"

type adminRig struct {
	t      *testing.T
	e      *env
	engine *gin.Engine
	router http.Handler
	admin  *auth.JWTIssuer
}

func newAdminRig(t *testing.T) *adminRig {
	t.Helper()
	pool := testutil.PoolFromEnv(t)
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := config.Config{
		Env: config.EnvCI, Role: config.RoleAll, JWTSecret: rbacSecret,
		MockBankSecret: "x", StorageProvider: "local", KYCStorageDir: t.TempDir(),
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	r := api.NewRouter(api.Deps{Cfg: cfg, Log: log, Pool: pool, Redis: rdb})
	return &adminRig{t: t, e: newEnv(t, ""), engine: r, router: r, admin: auth.NewJWTIssuer(rbacSecret, "admin")}
}

// newAdmin inserts an admin with the given role and returns its id and a token.
func (r *adminRig) newAdmin(role, status string) (uuid.UUID, string) {
	r.t.Helper()
	var id uuid.UUID
	email := "rbac-" + uuid.NewString()[:8] + "@HPay.dev"
	if err := r.e.pool.QueryRow(context.Background(),
		`INSERT INTO admin_users (email, password_hash, full_name, role, status)
		 VALUES ($1, 'x', 'RBAC Test', $2::admin_role, $3::account_status) RETURNING id`, email, role, status).Scan(&id); err != nil {
		r.t.Fatalf("insert admin: %v", err)
	}
	tok, err := r.admin.Issue(id, "rbac-test", time.Now())
	if err != nil {
		r.t.Fatal(err)
	}
	return id, tok
}

func (r *adminRig) call(method, path, token string, body any) *httptest.ResponseRecorder {
	r.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.router.ServeHTTP(w, req)
	return w
}

func errCode(w *httptest.ResponseRecorder) string {
	var b struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &b)
	return b.Code
}

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
	{"GET", "/v1/admin/approvals", []string{"risk_officer", "ops", "superadmin"}},
	{"POST", "/v1/admin/approvals/" + zeroID + "/decide", []string{"risk_officer", "ops", "superadmin"}},
	{"POST", "/v1/admin/recon/run", []string{"ops", "superadmin"}},
	{"GET", "/v1/admin/recon/runs/2026-01-01", []string{"ops", "superadmin"}},
	{"GET", "/v1/admin/workflows", []string{"risk_officer", "ops", "superadmin"}},
	{"GET", "/v1/admin/workflows/" + zeroID, []string{"risk_officer", "ops", "superadmin"}},
	{"GET", "/v1/admin/orgs", []string{"risk_officer", "ops", "superadmin"}},
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
	rig := newAdminRig(t)
	_, tok := rig.newAdmin("viewer", "active")
	for _, rt := range adminRoutes {
		w := rig.call(rt.method, rt.path, tok, map[string]any{})
		if w.Code != http.StatusForbidden || errCode(w) != "ERR_FORBIDDEN_PERMISSION" {
			t.Errorf("%s %s as viewer: got %d %q, want 403 ERR_FORBIDDEN_PERMISSION", rt.method, rt.path, w.Code, errCode(w))
		}
		if strings.Contains(w.Body.String(), "kyc.review") || strings.Contains(w.Body.String(), "superadmin") {
			t.Errorf("%s %s: the denial must not reveal the permission model: %s", rt.method, rt.path, w.Body.String())
		}
	}
}

// TC-RBA-002: for each role, 403 exactly where the table says so.
func TestAdminRBAC_RoleMatrix(t *testing.T) {
	rig := newAdminRig(t)
	for _, role := range all5 {
		_, tok := rig.newAdmin(role, "active")
		for _, rt := range adminRoutes {
			w := rig.call(rt.method, rt.path, tok, map[string]any{})
			denied := w.Code == http.StatusForbidden && errCode(w) == "ERR_FORBIDDEN_PERMISSION"
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
	rig := newAdminRig(t)
	id, tok := rig.newAdmin("superadmin", "active")
	if w := rig.call("GET", "/v1/admin/audit", tok, nil); w.Code != http.StatusOK {
		t.Fatalf("setup: active superadmin should read the audit log, got %d", w.Code)
	}
	if _, err := rig.e.pool.Exec(context.Background(), `UPDATE admin_users SET status='locked' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	w := rig.call("GET", "/v1/admin/audit", tok, nil)
	if w.Code != http.StatusUnauthorized || errCode(w) != "ERR_ACCOUNT_LOCKED" {
		t.Fatalf("locked admin: got %d %q, want 401 ERR_ACCOUNT_LOCKED", w.Code, errCode(w))
	}
}

// TC-RBA-004: the role is read from the database, not from the token.
func TestAdminRBAC_RoleChangeTakesEffect(t *testing.T) {
	rig := newAdminRig(t)
	id, tok := rig.newAdmin("ops", "active")
	if w := rig.call("POST", "/v1/admin/recon/run", tok, map[string]any{}); w.Code == http.StatusForbidden {
		t.Fatalf("setup: ops may run recon, got 403")
	}
	if _, err := rig.e.pool.Exec(context.Background(), `UPDATE admin_users SET role='viewer' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if w := rig.call("POST", "/v1/admin/recon/run", tok, map[string]any{}); w.Code != http.StatusForbidden {
		t.Fatalf("after demotion to viewer: got %d, want 403", w.Code)
	}
}

// TC-RBA-005: a valid token for an admin that no longer exists is refused.
func TestAdminRBAC_DeletedAdminRefused(t *testing.T) {
	rig := newAdminRig(t)
	tok, err := rig.admin.Issue(uuid.New(), "ghost", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w := rig.call("GET", "/v1/admin/audit", tok, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown admin id: got %d, want 401", w.Code)
	}
}

// TC-RBA-008: a user-audience token still cannot call admin routes.
func TestAdminRBAC_UserTokenRefused(t *testing.T) {
	rig := newAdminRig(t)
	uid, _, _ := rig.e.userWithMoney(t, 0)
	tok, err := auth.NewJWTIssuer(rbacSecret, "user").Issue(uid, "dev", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if w := rig.call("GET", "/v1/admin/audit", tok, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("user token on an admin route: got %d, want 401", w.Code)
	}
}

// TC-RBA-010: every route the router registers under /v1/admin is in the table above,
// so a new admin route cannot ship without a permission decision.
func TestEveryAdminRouteHasAPermission(t *testing.T) {
	rig := newAdminRig(t)
	known := map[string]bool{}
	for _, rt := range adminRoutes {
		known[rt.method+" "+normalizeAdminPath(rt.path)] = true
	}
	for _, ri := range rig.engine.Routes() {
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
	rig := newAdminRig(t)
	ctx := context.Background()
	adminID, tok := rig.newAdmin("superadmin", "active")
	const secret = "sk-test-SECRET-key-material-0123456789"

	w := rig.call("POST", "/v1/admin/ai/keys", tok, map[string]any{"name": "rbac key " + uuid.NewString()[:6], "provider": "anthropic", "api_key": secret})
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

	w = rig.call("PUT", "/v1/admin/ai/keys/"+created.ID.String(), tok, map[string]any{"name": "renamed", "is_active": true, "api_key": secret + "-v2"})
	if w.Code != http.StatusOK {
		t.Fatalf("update key: %d %s", w.Code, w.Body.String())
	}
	w = rig.call("PUT", "/v1/admin/ai/settings", tok, map[string]any{"function_name": "fraud_advising", "provider": "mock", "model_name": "mock"})
	if w.Code != http.StatusOK {
		t.Fatalf("update setting: %d %s", w.Code, w.Body.String())
	}

	rows, err := rig.e.pool.Query(ctx, `SELECT action, resource, coalesce(reason,''), coalesce(before::text,''), coalesce(after::text,'') FROM audit_logs WHERE actor_id=$1 AND actor_type='admin' AND action LIKE 'ai.%'`, adminID)
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

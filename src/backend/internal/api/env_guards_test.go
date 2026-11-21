package api

// Slice 3 (BE-H7, ADR-0012): HTTP-level guards that depend on the validated
// environment. Cases map to docs/specs/w1-environment-guards/test-cases.md
// (TC-ENV-003, 005, 008, 009, 010). They need no database or Redis.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/hieumai1507/hpay-backend/internal/platform/config"
)

func init() { gin.SetMode(gin.TestMode) }

// TC-ENV-003: the simulated bank callback must not exist in production.
func TestMockBankRouteMountedOnlyWhereAllowed(t *testing.T) {
	for _, tc := range []struct {
		env     string
		mounted bool
	}{
		{config.EnvDev, true}, {config.EnvCI, true}, {config.EnvDemo, true}, {config.EnvProduction, false},
	} {
		t.Run(tc.env, func(t *testing.T) {
			r := gin.New()
			registerMockBank(r, config.Config{Env: tc.env, MockBankSecret: "x"}, nil)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/internal/mock-bank/callback", strings.NewReader("{}"))
			r.ServeHTTP(w, req)
			got := w.Code != http.StatusNotFound
			if got != tc.mounted {
				t.Fatalf("env=%s: mounted=%v (status %d), want mounted=%v", tc.env, got, w.Code, tc.mounted)
			}
		})
	}
}

// TC-ENV-005: OTP codes are returned in a response only for APP_ENV=dev.
func TestOTPExposedOnlyInDev(t *testing.T) {
	wantDev := map[string]bool{
		config.EnvDev: true, config.EnvCI: false, config.EnvDemo: false, config.EnvProduction: false,
	}
	for env, want := range wantDev {
		cfg := config.Config{Env: env}
		if got := cfg.IsDev(); got != want {
			t.Errorf("env=%s: IsDev=%v want %v", env, got, want)
		}
	}
	body := challengeBody(config.Config{Env: config.EnvProduction}, "123456")
	if _, leaked := body["otp_dev"]; leaked {
		t.Fatal("otp_dev leaked in production")
	}
	body = challengeBody(config.Config{Env: config.EnvDemo}, "123456")
	if _, leaked := body["otp_dev"]; leaked {
		t.Fatal("otp_dev leaked in demo")
	}
	body = challengeBody(config.Config{Env: config.EnvDev}, "123456")
	if body["otp_dev"] != "123456" {
		t.Fatal("otp_dev must be present in dev")
	}
}

// TC-ENV-008: /healthz must never carry error text.
func TestHealthzHidesErrorText(t *testing.T) {
	secret := "dial tcp 10.1.2.3:5432: connect: connection refused (password=hunter2)"
	body := healthBody(config.Config{Role: config.RoleAll}, errString(secret), errString(secret))
	raw, _ := json.Marshal(body)
	for _, bad := range []string{"10.1.2.3", "hunter2", "connection refused", "dial tcp"} {
		if strings.Contains(string(raw), bad) {
			t.Fatalf("healthz body leaks %q: %s", bad, raw)
		}
	}
	if !strings.Contains(string(raw), `"down"`) {
		t.Fatalf("failed dependencies should be reported as down: %s", raw)
	}
	ok, _ := json.Marshal(healthBody(config.Config{Role: config.RoleAll}, nil, nil))
	if !strings.Contains(string(ok), `"ok"`) {
		t.Fatalf("healthy dependencies should be reported as ok: %s", ok)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// TC-ENV-009: baseline security headers; HSTS only in production.
func TestSecurityHeaders(t *testing.T) {
	for _, tc := range []struct {
		env  string
		hsts bool
	}{{config.EnvDev, false}, {config.EnvDemo, false}, {config.EnvProduction, true}} {
		t.Run(tc.env, func(t *testing.T) {
			r := gin.New()
			r.Use(securityHeaders(config.Config{Env: tc.env}))
			r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
			for k, want := range map[string]string{
				"X-Content-Type-Options": "nosniff",
				"Referrer-Policy":        "no-referrer",
				"X-Frame-Options":        "DENY",
			} {
				if got := w.Header().Get(k); got != want {
					t.Errorf("%s = %q, want %q", k, got, want)
				}
			}
			if got := w.Header().Get("Strict-Transport-Security") != ""; got != tc.hsts {
				t.Errorf("HSTS present=%v, want %v", got, tc.hsts)
			}
		})
	}
}

// TC-ENV-010: X-Forwarded-For is only honoured from a trusted proxy.
func TestTrustedProxiesControlClientIP(t *testing.T) {
	clientIP := func(trusted []string, remote string) string {
		r := gin.New()
		if err := applyTrustedProxies(r, config.Config{TrustedProxies: trusted}); err != nil {
			t.Fatalf("applyTrustedProxies: %v", err)
		}
		var got string
		r.GET("/ip", func(c *gin.Context) { got = c.ClientIP() })
		req := httptest.NewRequest(http.MethodGet, "/ip", nil)
		req.RemoteAddr = remote + ":4242"
		req.Header.Set("X-Forwarded-For", "6.6.6.6")
		r.ServeHTTP(httptest.NewRecorder(), req)
		return got
	}
	if got := clientIP(nil, "203.0.113.9"); got != "203.0.113.9" {
		t.Errorf("no trusted proxy: ClientIP = %s, want the socket address (a forged X-Forwarded-For must be ignored)", got)
	}
	if got := clientIP([]string{"203.0.113.9"}, "203.0.113.9"); got != "6.6.6.6" {
		t.Errorf("trusted proxy: ClientIP = %s, want the forwarded address", got)
	}
	if got := clientIP([]string{"198.51.100.1"}, "203.0.113.9"); got != "203.0.113.9" {
		t.Errorf("untrusted peer: ClientIP = %s, want the socket address", got)
	}
}

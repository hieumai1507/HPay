package config

// Slice 3 (BE-H7, ADR-0012): APP_ENV is required and validated; dangerous
// behavior is impossible outside the environments allowed to have it.
// Cases map to docs/specs/w1-environment-guards/test-cases.md (TC-ENV-001..004).

import (
	"strings"
	"testing"
)

// placeholderJWT is the value shipped in src/backend/.env.example.
const placeholderJWT = "dev-secret-change-me-32-bytes-min!!"

const strongSecret = "k9Qz3vB7xL1mP5tR8wY2cN6dF4gH0jA9sE3uI7oK"

func base(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("JWT_SECRET", strongSecret)
	t.Setenv("MOCK_BANK_SECRET", "a-strong-mock-bank-secret-value-1234")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("STORAGE_PROVIDER", "local")
}

// TC-ENV-001
func TestLoad_RejectsMissingOrUnknownEnv(t *testing.T) {
	for _, env := range []string{"", "prod", "Production", "production ", "staging", "test"} {
		t.Run("env="+env, func(t *testing.T) {
			base(t)
			t.Setenv("APP_ENV", env)
			_, err := Load("api")
			if err == nil {
				t.Fatalf("APP_ENV=%q: want an error, got nil", env)
			}
			for _, want := range []string{"dev", "ci", "demo", "production"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error should list the allowed value %q: %v", want, err)
				}
			}
		})
	}
}

// TC-ENV-002
func TestLoad_ProductionRejectsWeakOrDefaultSecrets(t *testing.T) {
	cases := []struct {
		name   string
		jwt    string
		mock   string
		wantIn string
	}{
		{"short jwt", "short-secret-20-bytes", "a-strong-mock-bank-secret-value-1234", "JWT_SECRET"},
		{"placeholder jwt", placeholderJWT, "a-strong-mock-bank-secret-value-1234", "JWT_SECRET"},
		{"mock secret unset", strongSecret, "", "MOCK_BANK_SECRET"},
		{"mock secret is the dev default", strongSecret, "dev-mock-bank-secret", "MOCK_BANK_SECRET"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base(t)
			t.Setenv("APP_ENV", "production")
			t.Setenv("JWT_SECRET", c.jwt)
			t.Setenv("MOCK_BANK_SECRET", c.mock)
			_, err := Load("api")
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			if !strings.Contains(err.Error(), c.wantIn) {
				t.Errorf("error should mention %s: %v", c.wantIn, err)
			}
		})
	}
}

func TestLoad_ProductionAcceptsStrongSecrets(t *testing.T) {
	base(t)
	t.Setenv("APP_ENV", "production")
	cfg, err := Load("api")
	if err != nil {
		t.Fatalf("strong production config rejected: %v", err)
	}
	if !cfg.IsProduction() || cfg.IsDev() || cfg.AllowsMockBank() {
		t.Errorf("production flags wrong: prod=%v dev=%v mockBank=%v", cfg.IsProduction(), cfg.IsDev(), cfg.AllowsMockBank())
	}
}

// TC-ENV-004: dev, ci and demo keep working with the short secrets that tests and CI use.
func TestLoad_NonProductionKeepsDevBehavior(t *testing.T) {
	for _, env := range []string{"dev", "ci"} {
		t.Run(env, func(t *testing.T) {
			base(t)
			t.Setenv("APP_ENV", env)
			t.Setenv("JWT_SECRET", "short")
			t.Setenv("MOCK_BANK_SECRET", "")
			cfg, err := Load("api")
			if err != nil {
				t.Fatalf("APP_ENV=%s with short secrets should load: %v", env, err)
			}
			if cfg.IsProduction() {
				t.Error("IsProduction must be false")
			}
			if !cfg.AllowsMockBank() {
				t.Error("mock bank must be allowed outside production")
			}
		})
	}
	t.Run("demo requires strong secrets", func(t *testing.T) {
		base(t)
		t.Setenv("APP_ENV", "demo")
		t.Setenv("JWT_SECRET", "short")
		if _, err := Load("api"); err == nil {
			t.Fatal("demo with a short JWT secret should be rejected")
		}
	})
}

func TestIsDevOnlyForDev(t *testing.T) {
	for env, want := range map[string]bool{"dev": true, "ci": false, "demo": false, "production": false} {
		c := Config{Env: env}
		if got := c.IsDev(); got != want {
			t.Errorf("IsDev(%s) = %v, want %v", env, got, want)
		}
	}
}

func TestTrustedProxiesParsing(t *testing.T) {
	base(t)
	t.Setenv("APP_ENV", "dev")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 192.168.0.0/24 ,")
	cfg, err := Load("api")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "10.0.0.1" || cfg.TrustedProxies[1] != "192.168.0.0/24" {
		t.Fatalf("TrustedProxies = %v", cfg.TrustedProxies)
	}
	t.Setenv("TRUSTED_PROXIES", "")
	cfg, _ = Load("api")
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("unset TRUSTED_PROXIES must mean none, got %v", cfg.TrustedProxies)
	}
}

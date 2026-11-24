// Package config loads 12-factor configuration from environment variables.
// Fail fast: missing required keys abort startup with a clear message.
package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Environment names (ADR-0012). APP_ENV is required and must be one of these.
const (
	EnvDev        = "dev"
	EnvCI         = "ci"
	EnvDemo       = "demo"
	EnvProduction = "production"
)

// validEnvs lists the allowed APP_ENV values in the order shown in errors.
var validEnvs = []string{EnvDev, EnvCI, EnvDemo, EnvProduction}

// Known placeholder secrets that must never be accepted outside dev and ci.
const (
	devMockBankSecret = "dev-mock-bank-secret"
	exampleJWTSecret  = "dev-secret-change-me-32-bytes-min!!"
)

// Role decides which surfaces a process serves.
type Role string

const (
	RoleAPI    Role = "api"
	RoleWorker Role = "worker"
	RoleAdmin  Role = "admin"
	RoleAll    Role = "all"
)

func (r Role) Valid() bool {
	switch r {
	case RoleAPI, RoleWorker, RoleAdmin, RoleAll:
		return true
	}
	return false
}

// ServesHTTP reports whether this role mounts an HTTP server.
func (r Role) ServesHTTP() bool { return r == RoleAPI || r == RoleAdmin || r == RoleAll }

// RunsWorkers reports whether this role consumes streams and scheduled jobs.
func (r Role) RunsWorkers() bool { return r == RoleWorker || r == RoleAll }

// MountsPublicAPI reports whether public /v1 routes are served.
func (r Role) MountsPublicAPI() bool { return r == RoleAPI || r == RoleAll }

// MountsAdminAPI reports whether /v1/admin and /v1/org routes are served.
func (r Role) MountsAdminAPI() bool { return r == RoleAdmin || r == RoleAll }

type Config struct {
	Env               string // dev | ci | demo | production (required, validated; see ADR-0012)
	Role              Role
	HTTPPort          int
	DatabaseURL       string
	RedisAddr         string
	WorkflowConsumers int
	JWTSecret         string
	// F8 KYC
	KYCStorageDir string // local image storage root
	// F10 — mock bank callback (E15); route lives under /internal/
	MockBankSecret string
	// Cloudinary configuration
	StorageProvider     string // local | cloudinary
	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
	// TrustedProxies lists proxy addresses/CIDRs whose X-Forwarded-For is trusted
	// (env TRUSTED_PROXIES, comma separated). Empty means trust none.
	TrustedProxies []string
	// TOTPKey is the 32-byte AES-256 key that protects TOTP secrets at rest (ADR-0017). Required
	// (TOTP_ENCRYPTION_KEY, base64) outside dev and ci; dev and ci get a fixed test key.
	TOTPKey []byte
	// Code delivery (ADR-0016, ADR-0017): SENDER_PROVIDER is "" (environment default) or "twilio".
	SenderProvider   string
	TwilioAccountSID string
	TwilioAPIKey     string
	TwilioAPISecret  string
	TwilioFrom       string
}

// IsDev is true only for APP_ENV=dev: the single environment that may return
// OTP codes in responses and load a local .env file.
func (c Config) IsDev() bool { return c.Env == EnvDev }

// IsProduction is true only for APP_ENV=production: the real outbound gateway,
// HSTS, and the strictest secret checks.
func (c Config) IsProduction() bool { return c.Env == EnvProduction }

// AllowsMockBank reports whether the simulated bank callback route may be mounted.
func (c Config) AllowsMockBank() bool { return c.Env != EnvProduction }

// requiresStrongSecrets is true for every environment except dev and ci.
func (c Config) requiresStrongSecrets() bool { return c.Env != EnvDev && c.Env != EnvCI }

func Load(role string) (Config, error) {
	// Dev convenience: read ./.env if present, but only when the process was
	// explicitly started as dev. APP_ENV must come from the real environment, so a
	// stray .env can never switch a process into dev. Existing variables still win.
	if os.Getenv("APP_ENV") == EnvDev {
		_ = godotenv.Load()
	}

	r := Role(role)
	if !r.Valid() {
		return Config{}, fmt.Errorf("invalid role %q (want api|worker|admin|all)", role)
	}

	cfg := Config{
		Env:         os.Getenv("APP_ENV"),
		Role:        r,
		HTTPPort:    getenvInt("HTTP_PORT", 8080),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),
		// Consumers per workflow stream (P1). Raise on bigger machines.
		WorkflowConsumers: getenvInt("WORKFLOW_CONSUMERS", 8),
		JWTSecret:         os.Getenv("JWT_SECRET"),

		KYCStorageDir: getenv("KYC_STORAGE_DIR", "./data/kyc"),

		MockBankSecret: os.Getenv("MOCK_BANK_SECRET"),

		StorageProvider:     getenv("STORAGE_PROVIDER", "local"),
		CloudinaryCloudName: os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudinaryAPIKey:    os.Getenv("CLOUDINARY_API_KEY"),
		CloudinaryAPISecret: os.Getenv("CLOUDINARY_API_SECRET"),
		TrustedProxies:      splitList(os.Getenv("TRUSTED_PROXIES")),

		SenderProvider:   strings.ToLower(strings.TrimSpace(os.Getenv("SENDER_PROVIDER"))),
		TwilioAccountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAPIKey:     os.Getenv("TWILIO_API_KEY"),
		TwilioAPISecret:  os.Getenv("TWILIO_API_SECRET"),
		TwilioFrom:       os.Getenv("TWILIO_FROM"),
	}

	if !isValidEnv(cfg.Env) {
		return Config{}, fmt.Errorf("APP_ENV %q is not valid: it is required and must be one of %s", cfg.Env, strings.Join(validEnvs, ", "))
	}
	// Outside dev and ci an unset mock bank secret is an error; inside them the
	// historical dev default keeps local runs and tests working.
	if cfg.MockBankSecret == "" && !cfg.requiresStrongSecrets() {
		cfg.MockBankSecret = devMockBankSecret
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.requiresStrongSecrets() {
		if len(cfg.JWTSecret) < 32 {
			return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 bytes when APP_ENV=%s", cfg.Env)
		}
		if cfg.JWTSecret == exampleJWTSecret {
			return Config{}, fmt.Errorf("JWT_SECRET is the placeholder from .env.example; set a random secret when APP_ENV=%s", cfg.Env)
		}
		if cfg.MockBankSecret == "" || cfg.MockBankSecret == devMockBankSecret {
			return Config{}, fmt.Errorf("MOCK_BANK_SECRET must be set to a non-default value when APP_ENV=%s", cfg.Env)
		}
	}
	key, err := loadTOTPKey(os.Getenv("TOTP_ENCRYPTION_KEY"), cfg.requiresStrongSecrets())
	if err != nil {
		return Config{}, err
	}
	cfg.TOTPKey = key
	switch cfg.SenderProvider {
	case "":
	case "twilio":
		if cfg.TwilioAccountSID == "" || cfg.TwilioAPIKey == "" || cfg.TwilioAPISecret == "" || cfg.TwilioFrom == "" {
			return Config{}, fmt.Errorf("TWILIO_ACCOUNT_SID, TWILIO_API_KEY, TWILIO_API_SECRET and TWILIO_FROM are required when SENDER_PROVIDER=twilio")
		}
	default:
		return Config{}, fmt.Errorf("SENDER_PROVIDER %q is not valid: use twilio or leave it empty", cfg.SenderProvider)
	}
	if cfg.StorageProvider == "cloudinary" {
		if cfg.CloudinaryCloudName == "" || cfg.CloudinaryAPIKey == "" || cfg.CloudinaryAPISecret == "" {
			return Config{}, fmt.Errorf("CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, and CLOUDINARY_API_SECRET are required when STORAGE_PROVIDER=cloudinary")
		}
	}
	return cfg, nil
}

// devTOTPKey is the fixed key used in dev and ci only, so local runs and tests need no setup.
// It protects nothing real: outside dev and ci a real key is required.
const devTOTPKey = "dev-only-totp-key-0123456789abcd" // exactly 32 bytes

// loadTOTPKey decodes TOTP_ENCRYPTION_KEY (base64, 32 bytes). Outside dev and ci it is required.
func loadTOTPKey(raw string, required bool) ([]byte, error) {
	if raw == "" {
		if required {
			return nil, fmt.Errorf("TOTP_ENCRYPTION_KEY is required (base64, 32 bytes) when APP_ENV is not dev or ci")
		}
		return []byte(devTOTPKey), nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("TOTP_ENCRYPTION_KEY is not valid base64")
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("TOTP_ENCRYPTION_KEY must decode to exactly 32 bytes, got %d", len(key))
	}
	return key, nil
}

func isValidEnv(e string) bool {
	for _, v := range validEnvs {
		if e == v {
			return true
		}
	}
	return false
}

// splitList splits a comma separated list, trimming spaces and dropping empties.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

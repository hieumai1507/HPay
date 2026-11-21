// Package config loads 12-factor configuration from environment variables.
// Fail fast: missing required keys abort startup with a clear message.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
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
	Env         string // dev | ci | demo
	Role        Role
	HTTPPort    int
	DatabaseURL string
	RedisAddr   string
	JWTSecret   string
	// F8 KYC
	KYCStorageDir string // local image storage root
	// F10 — mock bank callback (E15); route lives under /internal/
	MockBankSecret string
	// Cloudinary configuration
	StorageProvider     string // local | cloudinary
	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
}

func Load(role string) (Config, error) {
	// Dev convenience: read ./.env if present. Existing environment variables
	// always win (godotenv does not override), so CI/prod are unaffected.
	_ = godotenv.Load()

	r := Role(role)
	if !r.Valid() {
		return Config{}, fmt.Errorf("invalid role %q (want api|worker|admin|all)", role)
	}

	cfg := Config{
		Env:         getenv("APP_ENV", "dev"),
		Role:        r,
		HTTPPort:    getenvInt("HTTP_PORT", 8080),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:   os.Getenv("JWT_SECRET"),

		KYCStorageDir: getenv("KYC_STORAGE_DIR", "./data/kyc"),

		MockBankSecret: getenv("MOCK_BANK_SECRET", "dev-mock-bank-secret"),

		StorageProvider:     getenv("STORAGE_PROVIDER", "local"),
		CloudinaryCloudName: os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudinaryAPIKey:    os.Getenv("CLOUDINARY_API_KEY"),
		CloudinaryAPISecret: os.Getenv("CLOUDINARY_API_SECRET"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 && cfg.Env != "dev" {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 bytes outside dev")
	}
	if cfg.StorageProvider == "cloudinary" {
		if cfg.CloudinaryCloudName == "" || cfg.CloudinaryAPIKey == "" || cfg.CloudinaryAPISecret == "" {
			return Config{}, fmt.Errorf("CLOUDINARY_CLOUD_NAME, CLOUDINARY_API_KEY, and CLOUDINARY_API_SECRET are required when STORAGE_PROVIDER=cloudinary")
		}
	}
	return cfg, nil
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

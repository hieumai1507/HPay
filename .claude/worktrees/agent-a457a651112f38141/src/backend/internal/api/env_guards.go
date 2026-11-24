package api

import (
	"github.com/gin-gonic/gin"

	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

// registerMockBank mounts the simulated bank callback only in environments that
// may have it (never production; ADR-0012). walletSvc may be nil in tests that
// only check mounting.
func registerMockBank(r gin.IRoutes, cfg config.Config, walletSvc *wallet.Service) {
	if !cfg.AllowsMockBank() {
		return
	}
	h := NewMockBankHandler(walletSvc, cfg.MockBankSecret)
	r.POST("/internal/mock-bank/callback", h.Callback)
}

// challengeBody builds the approval challenge response. The OTP code is exposed
// only when APP_ENV=dev; every other environment delivers it out of band.
func challengeBody(cfg config.Config, code string) gin.H {
	body := gin.H{"status": "otp_sent"}
	if cfg.IsDev() {
		body["otp_dev"] = code
	}
	return body
}

// healthBody reports dependency health WITHOUT error text: driver errors carry
// hosts, ports and sometimes credentials, and /healthz is anonymous. The reason
// is logged by the caller.
func healthBody(cfg config.Config, pgErr, redisErr error) gin.H {
	word := func(err error) string {
		if err != nil {
			return "down"
		}
		return "ok"
	}
	status := "ok"
	if pgErr != nil || redisErr != nil {
		status = "degraded"
	}
	return gin.H{
		"status": status,
		"role":   cfg.Role,
		"checks": gin.H{"postgres": word(pgErr), "redis": word(redisErr)},
	}
}

// securityHeaders sets baseline response headers. HSTS only in production,
// where the service is served over TLS by the proxy.
func securityHeaders(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		if cfg.IsProduction() {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// applyTrustedProxies makes ClientIP trust X-Forwarded-For only from the
// configured proxies. With none configured the socket address is used, so a
// forged header cannot change the IP the rate limiter keys on.
func applyTrustedProxies(r *gin.Engine, cfg config.Config) error {
	if len(cfg.TrustedProxies) == 0 {
		return r.SetTrustedProxies(nil)
	}
	return r.SetTrustedProxies(cfg.TrustedProxies)
}

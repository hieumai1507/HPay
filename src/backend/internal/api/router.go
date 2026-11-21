// Package api wires the HTTP surface. Route groups are mounted per role:
// /v1/* (public, role api), /v1/org/* + /v1/admin/* (role admin).
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/fee"
	"github.com/hieumai1507/hpay-backend/internal/fraud"
	"github.com/hieumai1507/hpay-backend/internal/kyc"
	"github.com/hieumai1507/hpay-backend/internal/moneyrequest"
	"github.com/hieumai1507/hpay-backend/internal/org"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/internal/product"
	"github.com/hieumai1507/hpay-backend/internal/qr"
	"github.com/hieumai1507/hpay-backend/internal/recon"
	"github.com/hieumai1507/hpay-backend/internal/reversal"
	"github.com/hieumai1507/hpay-backend/internal/storage"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/vault"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

// Deps carries shared dependencies into handlers.
type Deps struct {
	Cfg   config.Config
	Log   *slog.Logger
	Pool  *pgxpool.Pool
	Redis *redis.Client
}

// NewRouter builds the Gin engine for the process role.
func NewRouter(d Deps) *gin.Engine {
	if d.Cfg.Env != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), requestID(), accessLog(d.Log))

	r.GET("/healthz", healthz(d))
	// Gin doesn't auto-answer HEAD for a GET-only route (unlike some
	// frameworks) — 404s instead. wait-on (used by hpay-admin's e2e CI job
	// and local dev scripts to block until the server is ready) sends HEAD
	// requests by default, so without this it polls until it times out even
	// though the server is actually up and healthy.
	r.HEAD("/healthz", healthz(d))

	// Shared services: the public and the org/admin surfaces need the same
	// issuer, engine, fee service and KYC storage (role=all runs them all).
	issuer := auth.NewJWTIssuer(d.Cfg.JWTSecret, "user")
	authSvc := auth.NewService(d.Pool, issuer, d.Log, d.Cfg.Env)
	engine := workflow.NewEngine(d.Pool, d.Log)
	fees := fee.NewService(d.Pool)
	walletSvc := wallet.NewService(d.Pool)
	var files storage.Store
	var err error
	if d.Cfg.StorageProvider == "cloudinary" {
		files = storage.NewCloudinary(d.Cfg.CloudinaryCloudName, d.Cfg.CloudinaryAPIKey, d.Cfg.CloudinaryAPISecret)
	} else {
		files, err = storage.NewLocal(d.Cfg.KYCStorageDir)
		if err != nil {
			d.Log.Error("kyc storage unavailable; kyc routes will fail", "err", err)
		}
	}
	signer := storage.NewSigner(d.Cfg.JWTSecret)

	if d.Cfg.Role.MountsPublicAPI() {
		// Mock bank callback (F10 E15): shared-secret header, no JWT. Under
		// /internal/ — a real deploy must block this prefix at the proxy.
		mockBankH := NewMockBankHandler(walletSvc, d.Cfg.MockBankSecret)
		r.POST("/internal/mock-bank/callback", mockBankH.Callback)

		v1 := r.Group("/v1")

		authH := NewAuthHandler(authSvc)
		authH.AfterAuth = walletSvc.EnsureMainWallet // self-heal wallet on register/login
		walletH := NewWalletHandler(walletSvc)

		transferSvc := transfer.NewService(d.Pool, engine, fees)
		qrSvc := qr.NewService(signer)
		transferH := NewTransferHandler(transferSvc, walletSvc, qrSvc)
		qrH := NewQRHandler(d.Pool, qrSvc)

		requestSvc := moneyrequest.NewService(d.Pool, engine, fees)
		requestH := NewRequestHandler(requestSvc)

		ag := v1.Group("/auth")
		ag.POST("/register", RateLimit(d.Redis, 5, time.Minute), authH.Register)
		ag.POST("/login", RateLimit(d.Redis, 5, time.Minute), authH.Login)
		ag.POST("/refresh", RateLimit(d.Redis, 30, time.Minute), authH.Refresh)
		ag.POST("/logout", authH.Logout)

		me := v1.Group("", RequireAuth(issuer))
		userH := NewUserHandler(d.Pool)
		me.GET("/users/me", userH.Me)
		me.GET("/users/me/qr", qrH.Mint)
		me.POST("/qr/resolve", RateLimitUser(d.Redis, 30, time.Minute), qrH.Resolve)
		me.GET("/users/lookup", RateLimitUser(d.Redis, 20, time.Minute), userH.Lookup)
		me.GET("/wallets", walletH.List)
		me.GET("/transactions", walletH.History)
		me.GET("/transactions/:refCode", walletH.Detail)
		me.GET("/transactions/:refCode/timeline", transferH.Timeline)
		me.GET("/fees/preview", transferH.FeePreview)

		// Service Engine Core — Unified 3-Step Execution API for ALL transaction services
		engineH := NewTransactionEngineHandler(transferSvc)
		me.POST("/transactions/request", RequireIdempotencyKey(), engineH.RequestTransaction)
		me.POST("/transactions/confirm", engineH.ConfirmTransaction)
		me.POST("/transactions/verify", engineH.VerifyTransaction)

		// Vaults: metadata only — money moves via the engine (VAULT_SAVINGS).
		vaultH := NewVaultHandler(vault.New(d.Pool))
		me.GET("/vaults", vaultH.List)
		me.POST("/vaults", vaultH.Create)
		me.PATCH("/vaults/:id/round-up-target", vaultH.SetRoundUpTarget)

		me.POST("/money-requests", requestH.Create)
		me.POST("/money-requests/split", requestH.CreateSplit)
		me.GET("/money-requests/inbox", requestH.Inbox)
		me.GET("/money-requests/sent", requestH.Sent)
		me.GET("/money-requests/:id", requestH.Detail)
		me.POST("/money-requests/:id/decline", requestH.Decline)
		me.POST("/money-requests/:id/cancel", requestH.Cancel)
		me.POST("/money-requests/:id/remind", requestH.Remind)

		kycSvc := kyc.NewService(d.Pool, engine, files)
		kycH := NewKYCHandler(kycSvc)
		me.POST("/kyc/submit", kycH.Submit)
		me.GET("/kyc/status", kycH.Status)
	}
	if d.Cfg.Role.MountsAdminAPI() {
		// Org surface (F6): business-wallet users (maker/checker) call with a
		// user JWT; membership/role checks live in the service layer queries
		// (fintech-review #8).
		orgSvc := org.NewService(d.Pool, engine, fees)
		orgH := NewOrgHandler(orgSvc)
		approvalSvc := approval.NewService(d.Pool, authSvc, d.Log)
		approvalH := NewApprovalHandler(approvalSvc, d.Cfg.Env)

		og := r.Group("/v1/org", RequireAuth(issuer))
		og.GET("", orgH.ListMyOrgs)
		og.POST("", orgH.Create)
		og.POST("/:id/members", orgH.AddMember)
		og.POST("/:id/policies", orgH.CreatePolicy)
		og.POST("/:id/transactions", RequireIdempotencyKey(), orgH.Disburse)
		og.GET("/approvals", approvalH.Inbox)
		og.GET("/approvals/by-ref/:ref_code", approvalH.DetailByRefCode)
		og.GET("/approvals/:id", approvalH.Detail)
		og.POST("/approvals/:id/challenge", RateLimit(d.Redis, 10, time.Minute), approvalH.Challenge)
		og.POST("/approvals/:id/decide", approvalH.Decide)
		og.POST("/approvals/:id/cancel", approvalH.Cancel)

		// Admin surface (F8/F9): separate credentials + aud=admin token
		// (docs 01 §3.1). Fraud review, reconciliation, reversal My Tasks.
		adminIssuer := auth.NewJWTIssuer(d.Cfg.JWTSecret, "admin")

		// The reversal executor runs inside AdminDecide's finalize tx, so it
		// must be registered on the SAME approval service this process uses.
		reversal.RegisterExecutor(approvalSvc)
		reconSvc := recon.NewService(d.Pool, files, signer)
		productSvc := product.NewService(d.Pool)

		adminH := NewAdminHandler(
			auth.NewAdminService(d.Pool, adminIssuer),
			kyc.NewAdminService(d.Pool, signer),
			fraud.NewAdminService(d.Pool),
			reconSvc,
			reversal.NewService(d.Pool),
			approvalSvc,
			productSvc,
			orgSvc,
			files, signer,
			d.Pool,
		)
		adm := r.Group("/v1/admin")
		adm.POST("/auth/login", RateLimit(d.Redis, 5, time.Minute), adminH.Login)
		adg := adm.Group("", RequireAdmin(adminIssuer))
		adg.GET("/kyc", adminH.KYCQueue)
		adg.POST("/kyc/:id/review", adminH.KYCReview)
		adg.GET("/fraud-flags", adminH.FraudFlags)
		adg.POST("/fraud-flags/:id/resolve", adminH.ResolveFraudFlag)
		adg.POST("/reversals", adminH.CreateReversal)
		adg.GET("/approvals", adminH.AdminApprovals)
		adg.POST("/approvals/:id/decide", adminH.AdminDecide)
		adg.POST("/recon/run", adminH.ReconRun)
		adg.GET("/recon/runs/:date", adminH.ReconGet)
		adg.GET("/services", adminH.ListServices)
		adg.GET("/services/:id", adminH.GetService)
		adg.PUT("/services/:id", adminH.UpdateServiceStatus)
		adg.GET("/offers", adminH.ListOffers)
		adg.GET("/offers/:id", adminH.GetOffer)
		adg.GET("/workflows", adminH.ListWorkflows)
		adg.GET("/workflows/:id", adminH.GetWorkflow)
		adg.GET("/audit", adminH.ListAuditLogs)
		adg.GET("/orgs", adminH.ListOrgs)
		adg.PUT("/orgs/:id/policies", adminH.UpdateOrgPolicy)
		adg.GET("/ai/keys", adminH.ListAIKeys)
		adg.POST("/ai/keys", adminH.CreateAIKey)
		adg.PUT("/ai/keys/:id", adminH.UpdateAIKey)
		adg.GET("/ai/settings", adminH.ListAISettings)
		adg.PUT("/ai/settings", adminH.UpdateAISetting)

		// Stored artifacts (KYC images, recon xlsx) are self-authorizing
		// (HMAC + 5' expiry) — no JWT here.
		fg := r.Group("/v1/files")
		fg.GET("/*key", adminH.ServeFile)
	}
	return r
}

// requestID tags every request; propagated into logs and (later) events.
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("request_id", id)
		c.Writer.Header().Set("X-Request-Id", id)
		c.Next()
	}
}

func accessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"dur_ms", time.Since(start).Milliseconds(),
		)
	}
}

// healthz reports readiness of postgres and redis. Degraded returns 503 with
// per-dependency detail (groundwork for E17: redis-down must be observable).
func healthz(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		status := http.StatusOK
		checks := gin.H{}

		if err := d.Pool.Ping(ctx); err != nil {
			checks["postgres"] = err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["postgres"] = "ok"
		}
		if err := d.Redis.Ping(ctx).Err(); err != nil {
			checks["redis"] = err.Error()
			status = http.StatusServiceUnavailable
		} else {
			checks["redis"] = "ok"
		}

		c.JSON(status, gin.H{"status": statusWord(status), "role": d.Cfg.Role, "checks": checks})
	}
}

func statusWord(code int) string {
	if code == http.StatusOK {
		return "ok"
	}
	return "degraded"
}

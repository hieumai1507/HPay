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
	if !d.Cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.MaxMultipartMemory = 8 << 20 // multipart parts above this go to temp files instead of memory
	if err := applyTrustedProxies(r, d.Cfg); err != nil {
		d.Log.Error("invalid TRUSTED_PROXIES; falling back to trusting none", "err", err)
		_ = r.SetTrustedProxies(nil)
	}
	r.Use(gin.Recovery(), requestID(), securityHeaders(d.Cfg), accessLog(d.Log))

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
		registerMockBank(r, d.Cfg, walletSvc)

		v1 := r.Group("/v1")

		authH := NewAuthHandler(authSvc)
		authH.AfterAuth = walletSvc.EnsureMainWallet // self-heal wallet on register/login
		walletH := NewWalletHandler(walletSvc)

		transferSvc := transfer.NewService(d.Pool, engine, fees)
		transferSvc.SetAuthenticator(authSvc) // ADR-0014: confirm verifies PIN/OTP on the server
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
		pinH := NewPINHandler(authSvc, d.Cfg)
		me.PUT("/me/pin", RateLimitUser(d.Redis, 10, time.Minute), pinH.Set)
		me.PATCH("/me/pin", RateLimitUser(d.Redis, 10, time.Minute), pinH.Change)
		// ADR-0016: forgotten PIN. Password, then a one-time code, then a new PIN.
		me.POST("/me/pin/reset/request", RateLimitUser(d.Redis, 5, time.Minute), pinH.ResetRequest)
		me.POST("/me/pin/reset/confirm", RateLimitUser(d.Redis, 10, time.Minute), pinH.ResetConfirm)
		engineH := NewTransactionEngineHandler(transferSvc, d.Cfg)
		me.POST("/transactions/request", RequireIdempotencyKey(), engineH.RequestTransaction)
		me.POST("/transactions/confirm", RateLimitUser(d.Redis, 20, time.Minute), engineH.ConfirmTransaction)
		me.POST("/transactions/challenge", RateLimitUser(d.Redis, 10, time.Minute), engineH.ChallengeTransaction)
		me.POST("/transactions/verify", engineH.VerifyTransaction)

		// Vaults: metadata only — money moves via the engine (VAULT_SAVINGS).
		vaultH := NewVaultHandler(vault.New(d.Pool))
		me.GET("/vaults", vaultH.List)
		me.POST("/vaults", vaultH.Create)
		me.PATCH("/vaults/:id/round-up-target", vaultH.SetRoundUpTarget)

		me.POST("/money-requests", RequireIdempotencyKey(), requestH.Create)
		me.POST("/money-requests/split", RequireIdempotencyKey(), requestH.CreateSplit)
		me.GET("/money-requests/inbox", requestH.Inbox)
		me.GET("/money-requests/sent", requestH.Sent)
		me.GET("/money-requests/:id", requestH.Detail)
		me.POST("/money-requests/:id/decline", requestH.Decline)
		me.POST("/money-requests/:id/cancel", requestH.Cancel)
		me.POST("/money-requests/:id/remind", requestH.Remind)

		kycSvc := kyc.NewService(d.Pool, engine, files)
		kycH := NewKYCHandler(kycSvc)
		me.POST("/kyc/submit", limitKYCBody(), kycH.Submit)
		me.GET("/kyc/status", kycH.Status)
	}
	if d.Cfg.Role.MountsAdminAPI() {
		// Org surface (F6): business-wallet users (maker/checker) call with a
		// user JWT; membership/role checks live in the service layer queries
		// (fintech-review #8).
		orgSvc := org.NewService(d.Pool, engine, fees)
		orgH := NewOrgHandler(orgSvc)
		approvalSvc := approval.NewService(d.Pool, authSvc, d.Log)
		approvalH := NewApprovalHandler(approvalSvc, d.Cfg)

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
		adg := adm.Group("", RequireAdmin(adminIssuer, d.Pool))
		// ADR-0015: every admin route names the permission it needs. A route without a
		// permission entry fails TestEveryAdminRouteHasAPermission.
		var (
			kycP      = RequirePermission(auth.PermKYCReview)
			fraudP    = RequirePermission(auth.PermFraudResolve)
			workflowP = RequirePermission(auth.PermWorkflowAction)
			reconP    = RequirePermission(auth.PermReconRun)
			orgsReadP = RequirePermission(auth.PermOrgsEdit, auth.PermWorkflowAction)
			orgsEditP = RequirePermission(auth.PermOrgsEdit)
			aiP       = RequirePermission(auth.PermAIManage)
			catalogP  = RequirePermission(auth.PermAdminsManage)
			pinResetP = RequirePermission(auth.PermPINReset)
			anyP      = RequirePermission() // any permission at all: a viewer sees nothing
		)
		adg.GET("/kyc", kycP, adminH.KYCQueue)
		adg.POST("/kyc/:id/review", kycP, adminH.KYCReview)
		adg.GET("/fraud-flags", fraudP, adminH.FraudFlags)
		adg.POST("/fraud-flags/:id/resolve", fraudP, adminH.ResolveFraudFlag)
		adg.POST("/reversals", workflowP, adminH.CreateReversal)
		adg.POST("/users/:id/pin/reset", pinResetP, adminH.ClearUserPIN) // ADR-0016 operator fallback
		adg.GET("/approvals", workflowP, adminH.AdminApprovals)
		adg.POST("/approvals/:id/decide", workflowP, adminH.AdminDecide)
		adg.POST("/recon/run", reconP, adminH.ReconRun)
		adg.GET("/recon/runs/:date", reconP, adminH.ReconGet)
		adg.GET("/services", anyP, adminH.ListServices)
		adg.GET("/services/:id", anyP, adminH.GetService)
		adg.PUT("/services/:id", catalogP, adminH.UpdateServiceStatus)
		adg.GET("/offers", anyP, adminH.ListOffers)
		adg.GET("/offers/:id", anyP, adminH.GetOffer)
		adg.GET("/workflows", workflowP, adminH.ListWorkflows)
		adg.GET("/workflows/:id", workflowP, adminH.GetWorkflow)
		adg.GET("/audit", anyP, adminH.ListAuditLogs)
		adg.GET("/orgs", orgsReadP, adminH.ListOrgs)
		adg.PUT("/orgs/:id/policies", orgsEditP, adminH.UpdateOrgPolicy)
		adg.GET("/ai/keys", aiP, adminH.ListAIKeys)
		adg.POST("/ai/keys", aiP, adminH.CreateAIKey)
		adg.PUT("/ai/keys/:id", aiP, adminH.UpdateAIKey)
		adg.GET("/ai/settings", aiP, adminH.ListAISettings)
		adg.PUT("/ai/settings", aiP, adminH.UpdateAISetting)

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

		pgErr := d.Pool.Ping(ctx)
		redisErr := d.Redis.Ping(ctx).Err()
		if pgErr != nil {
			d.Log.Warn("healthz: postgres ping failed", "err", pgErr)
			status = http.StatusServiceUnavailable
		}
		if redisErr != nil {
			d.Log.Warn("healthz: redis ping failed", "err", redisErr)
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, healthBody(d.Cfg, pgErr, redisErr))
	}
}

func statusWord(code int) string {
	if code == http.StatusOK {
		return "ok"
	}
	return "degraded"
}

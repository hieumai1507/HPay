// Command server is the single HPay binary. The --role flag decides which
// surfaces this process serves (see docs/plans/f1-foundation/solution-architecture.html §2).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hieumai1507/hpay-backend/internal/api"
	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/fee"
	"github.com/hieumai1507/hpay-backend/internal/fraud"
	"github.com/hieumai1507/hpay-backend/internal/gateway"
	"github.com/hieumai1507/hpay-backend/internal/kyc"
	"github.com/hieumai1507/hpay-backend/internal/moneyrequest"
	"github.com/hieumai1507/hpay-backend/internal/outbox"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/internal/platform/db"
	"github.com/hieumai1507/hpay-backend/internal/platform/logger"
	"github.com/hieumai1507/hpay-backend/internal/platform/redisclient"
	"github.com/hieumai1507/hpay-backend/internal/recon"
	"github.com/hieumai1507/hpay-backend/internal/storage"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
	"github.com/hieumai1507/hpay-backend/internal/workflow/handlers"
)

func main() {
	role := flag.String("role", "all", "process role: api | worker | admin | all")
	flag.Parse()

	if err := run(*role); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run(role string) error {
	cfg, err := config.Load(role)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(cfg.Env)
	log.Info("starting HPay", "role", cfg.Role, "env", cfg.Env)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	rdb, err := redisclient.Connect(ctx, cfg.RedisAddr)
	if err != nil {
		return fmt.Errorf("connect redis: %w", err)
	}
	defer rdb.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	var srv *http.Server
	errCh := make(chan error, 1)

	if cfg.Role.ServesHTTP() {
		router := api.NewRouter(api.Deps{Cfg: cfg, Log: log, Pool: pool, Redis: rdb})
		srv = &http.Server{
			Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 16,
		}
		go func() {
			log.Info("http listening", "addr", srv.Addr)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}

	if cfg.Role.RunsWorkers() {
		workerCtx, stopWorkers := context.WithCancel(context.Background())
		defer stopWorkers()

		registry := workflow.NewRegistry()
		handlers.RegisterAll(registry, pool)
		engine := workflow.NewEngine(pool, log)

		// F8: KYC steps — load dynamic configuration from the database at runtime
		var files storage.Store
		var err error
		if cfg.StorageProvider == "cloudinary" {
			files = storage.NewCloudinary(cfg.CloudinaryCloudName, cfg.CloudinaryAPIKey, cfg.CloudinaryAPISecret)
		} else {
			files, err = storage.NewLocal(cfg.KYCStorageDir)
			if err != nil {
				return fmt.Errorf("kyc storage: %w", err)
			}
		}
		kyc.RegisterHandlers(registry, pool, files)

		// F9: real fraud rule engine (replaces F5 stub) with dynamic LLM advisor.
		fraud.RegisterHandler(registry, pool, log)

		// G4: Action Pipeline — runs a service's configured 3rd-party actions.
		// Mock gateway outside production keeps dev/demo runs deterministic.
		var outbound gateway.Gateway = gateway.NewHTTP(10 * time.Second)
		if !cfg.IsProduction() {
			outbound = gateway.NewMock()
		}
		handlers.RegisterActionPipeline(registry, pool, outbound)

		// Round-up vault (docs 04 §2.2, stretch): optional ROUND_UP step in
		// p2p_transfer v2, sweeps a spend's rounding difference into a vault.
		handlers.RegisterRoundUp(registry, pool)

		// F6: parking on a human_approval step creates the approval request in
		// the same tx; the sweeper expires overdue requests (E10).
		issuer := auth.NewJWTIssuer(cfg.JWTSecret, "user")
		authSvc := auth.NewService(pool, issuer, log, cfg.Env)
		approvalSvc := approval.NewService(pool, authSvc, log)
		engine.SetPauseHook(approvalSvc)

		worker := workflow.NewWorker(pool, rdb, registry, engine, log)
		relay := outbox.NewRelay(pool, rdb, log)
		recovery := workflow.NewRecovery(pool, rdb, registry, engine, worker, log)
		sweeper := approval.NewSweeper(approvalSvc, log)

		// F7: expires overdue money requests nobody paid (the other half of E19).
		requestSvc := moneyrequest.NewService(pool, engine, fee.NewService(pool))
		requestSweeper := moneyrequest.NewSweeper(requestSvc, log)

		// F9: nightly reconciliation (00:15) of topups vs mock bank statements.
		signer := storage.NewSigner(cfg.JWTSecret)
		reconCron := recon.NewCron(recon.NewService(pool, files, signer), log)

		go relay.Run(workerCtx)
		go worker.Run(workerCtx)
		go recovery.Run(workerCtx)
		go sweeper.Run(workerCtx)
		go requestSweeper.Run(workerCtx)
		go reconCron.Run(workerCtx)
		log.Info("workflow worker running", "handlers", registry.Names())
	}

	select {
	case sig := <-stop:
		log.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	}

	if srv != nil {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown: %w", err)
		}
	}
	log.Info("bye")
	return nil
}

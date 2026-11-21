// Package integration holds cross-module, black-box integration tests.
// Unit and white-box tests stay colocated with their packages (Go convention;
// white-box tests need unexported access). See docs/plans + gen-api-test skill.
package integration

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/auth"
	"github.com/hieumai1507/hpay-backend/internal/fee"
	"github.com/hieumai1507/hpay-backend/internal/fraud"
	"github.com/hieumai1507/hpay-backend/internal/gateway"
	"github.com/hieumai1507/hpay-backend/internal/kyc"
	"github.com/hieumai1507/hpay-backend/internal/ledger"
	"github.com/hieumai1507/hpay-backend/internal/moneyrequest"
	"github.com/hieumai1507/hpay-backend/internal/org"
	"github.com/hieumai1507/hpay-backend/internal/qr"
	"github.com/hieumai1507/hpay-backend/internal/recon"
	"github.com/hieumai1507/hpay-backend/internal/reversal"
	"github.com/hieumai1507/hpay-backend/internal/storage"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
	wfhandlers "github.com/hieumai1507/hpay-backend/internal/workflow/handlers"
)

// This package pumps the shared outbox — serialize against other pumping
// packages and retire stale rows first (see testutil.RunSerialized).
func TestMain(m *testing.M) {
	os.Exit(testutil.RunSerialized(m))
}

type boom struct{}

func (boom) Name() string { return "boom" }
func (boom) Execute(_ context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	if in.Compensating {
		return workflow.StepOutput{}, nil
	}
	return workflow.StepOutput{}, fmt.Errorf("terminal boom")
}

// dialScorer lets each test choose the KYC score the SCORE step will see.
type dialScorer struct {
	mu    sync.Mutex
	score int
}

func (*dialScorer) Name() string { return "dial" }
func (d *dialScorer) Score(context.Context, []byte, []byte) (kyc.Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return kyc.Result{Score: d.score, Reasons: []string{"dialed by test"}}, nil
}

type env struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	engine *workflow.Engine
	worker *workflow.Worker
	reg    *workflow.Registry // E17 builds a second worker on a real redis
	svc    *transfer.Service
	wsvc   *wallet.Service
	asvc   *approval.Service
	osvc   *org.Service
	msvc   *moneyrequest.Service
	qrSvc  *qr.Service
	ksvc   *kyc.Service
	kadmin *kyc.AdminService
	dial   *dialScorer
	fadmin *fraud.AdminService
	rsvc   *recon.Service
	revsvc *reversal.Service
	files  storage.Store
	gw     *gateway.Mock // records Action Pipeline outbound calls (docs plan action-pipeline-wiring)
}

// seedAdmin2ID returns a second dev admin id (for maker≠checker reversal
// tests), creating it on first call.
func (e *env) seedAdmin2ID(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := e.pool.QueryRow(context.Background(),
		`INSERT INTO admin_users (email, password_hash, full_name, role)
		 VALUES ('admin2@HPay.dev', 'x', 'Dev Admin 2', 'ops')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("seed admin2: %v", err)
	}
	return id
}

func (e *env) setScore(score int) {
	e.dial.mu.Lock()
	defer e.dial.mu.Unlock()
	e.dial.score = score
}

// seedAdminID returns the migration-seeded dev admin (000014).
func (e *env) seedAdminID(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.pool.QueryRow(context.Background(),
		`SELECT id FROM admin_users WHERE email='admin@HPay.dev'`).Scan(&id); err != nil {
		t.Fatalf("seed admin missing — run migrations: %v", err)
	}
	return id
}

func newEnv(t *testing.T, defName string) *env {
	pool := testutil.PoolFromEnv(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	reg := workflow.NewRegistry()
	wfhandlers.RegisterAll(reg, pool)
	// p2p_transfer v2 (docs plan round-up-vault) appends ROUND_UP after
	// CONFIRM for every service on this workflow definition — must be
	// registered even where a test never configures a round-up target.
	wfhandlers.RegisterRoundUp(reg, pool)
	// docs plan action-pipeline-wiring: ACTIONS_BEFORE_GL/ACTIONS_AFTER_GL now
	// exist in p2p_transfer/org_disbursement — mock keeps tests deterministic,
	// matching production's non-"production" env behavior (cmd/server/main.go).
	mockGW := gateway.NewMock()
	wfhandlers.RegisterActionPipeline(reg, pool, mockGW)
	reg.Register(boom{})
	engine := workflow.NewEngine(pool, log)

	// F6: approval module — pause hook + OTP-signed checker decisions.
	// env=dev so Challenge returns the OTP code to the test.
	authSvc := auth.NewService(pool,
		auth.NewJWTIssuer("integration-test-secret-0123456789", "user"), log, "dev")
	asvc := approval.NewService(pool, authSvc, log)
	engine.SetPauseHook(asvc)

	// F8: KYC handlers with a test-dialable scorer + throwaway local storage.
	dial := &dialScorer{score: 90}
	files, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("kyc storage: %v", err)
	}
	if defName == "ai_config" {
		kyc.RegisterHandlers(reg, pool, files)
	} else {
		kyc.RegisterHandlers(reg, pool, files, dial)
	}

	// F9: real fraud handler (replaces the F5 stub) with dynamic advisor, plus
	// the reversal executor on the same approval service.
	fraud.RegisterHandler(reg, pool, log)
	reversal.RegisterExecutor(asvc)
	signer := storage.NewSigner("integration-test-secret")

	worker := workflow.NewWorker(pool, nil, reg, engine, log)
	fees := fee.NewService(pool)
	var svc *transfer.Service
	if defName == "" {
		svc = transfer.NewService(pool, engine, fees)
	} else {
		svc = transfer.NewServiceWithDefinition(pool, engine, fees, defName)
	}
	return &env{pool: pool, q: store.New(pool), engine: engine, worker: worker, reg: reg,
		svc: svc, wsvc: wallet.NewService(pool),
		asvc: asvc, osvc: org.NewService(pool, engine, fees),
		msvc:   moneyrequest.NewService(pool, engine, fees),
		qrSvc:  qr.NewService(signer),
		ksvc:   kyc.NewService(pool, engine, files),
		kadmin: kyc.NewAdminService(pool, signer),
		dial:   dial,
		fadmin: fraud.NewAdminService(pool),
		rsvc:   recon.NewService(pool, files, signer),
		revsvc: reversal.NewService(pool),
		files:  files, gw: mockGW}
}

func (e *env) drain(t *testing.T) {
	t.Helper()
	for i := 0; i < 30; i++ {
		n, err := e.worker.PumpLocal(context.Background(), 100)
		if err != nil {
			t.Fatalf("pump: %v", err)
		}
		if n == 0 {
			return
		}
	}
	t.Fatal("outbox never drained")
}

func (e *env) userWithMoney(t *testing.T, amount int64) (uuid.UUID, uuid.UUID, string) {
	t.Helper()
	userID, walletID := testutil.NewUserWithWallet(t, e.pool)
	var phone string
	if err := e.pool.QueryRow(context.Background(),
		`SELECT phone FROM users WHERE id = $1`, userID).Scan(&phone); err != nil {
		t.Fatal(err)
	}
	if amount > 0 {
		if _, err := e.wsvc.Topup(context.Background(), userID, amount, uuid.NewString()); err != nil {
			t.Fatalf("fund: %v", err)
		}
	}
	return userID, walletID, phone
}

func (e *env) txnLedgerNet(t *testing.T, refCode string, walletID uuid.UUID) int64 {
	t.Helper()
	var net int64
	err := e.pool.QueryRow(context.Background(), `
		SELECT COALESCE(SUM(CASE le.direction WHEN 'credit' THEN le.amount ELSE -le.amount END),0)
		FROM ledger_entries le JOIN transactions t ON t.id = le.transaction_id
		WHERE t.ref_code = $1 AND le.wallet_id = $2`, refCode, walletID).Scan(&net)
	if err != nil {
		t.Fatal(err)
	}
	return net
}

// postTransfer runs one raw ledger.Post inside its own transaction record —
// used by the low-level ledger scenarios (E02/E03).
func postTransfer(ctx context.Context, pool *pgxpool.Pool, initiator, from, to uuid.UUID, amount int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := store.New(tx)
	txn, err := qtx.InsertTransaction(ctx, store.InsertTransactionParams{
		RefCode: wallet.NewRefCode(), IdempotencyKey: pgtype.Text{String: uuid.NewString(), Valid: true},
		Type:           store.TxnTypeP2p,
		SenderWalletID: from, ReceiverWalletID: to,
		Amount: amount, Fee: 0,
		Note: pgtype.Text{}, InitiatedBy: initiator,
	})
	if err != nil {
		return err
	}
	if err := ledger.Post(ctx, tx, txn.ID, []ledger.Leg{
		{WalletID: from, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: to, Direction: store.EntryDirectionCredit, Amount: amount},
	}); err != nil {
		return err
	}
	if _, err := qtx.CompleteTransaction(ctx, txn.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func fund(t *testing.T, pool *pgxpool.Pool, owner, walletID uuid.UUID, amount int64) {
	t.Helper()
	cashIn := testutil.SystemWallet(t, pool, "SYSTEM_CASH_IN")
	if err := postTransfer(context.Background(), pool, owner, cashIn, walletID, amount); err != nil {
		t.Fatalf("fund: %v", err)
	}
}

func errIs(err, target error) bool {
	for e := err; e != nil; {
		if e == target {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

package testkit

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

type Env struct {
	Pool   *pgxpool.Pool
	Q      *store.Queries
	Engine *workflow.Engine
	Worker *workflow.Worker
	Reg    *workflow.Registry // E17 builds a second worker on a real redis
	Svc    *transfer.Service
	Auth   *auth.Service
	Wsvc   *wallet.Service
	Asvc   *approval.Service
	Osvc   *org.Service
	Msvc   *moneyrequest.Service
	QrSvc  *qr.Service
	Ksvc   *kyc.Service
	Kadmin *kyc.AdminService
	dial   *dialScorer
	Fadmin *fraud.AdminService
	Rsvc   *recon.Service
	Revsvc *reversal.Service
	Files  storage.Store
	Gw     *gateway.Mock // records Action Pipeline outbound calls (docs plan action-pipeline-wiring)
}

// seedAdmin2ID returns a second dev admin id (for maker≠checker reversal
// tests), creating it on first call.
func (e *Env) SeedAdmin2ID(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := e.Pool.QueryRow(context.Background(),
		`INSERT INTO admin_users (email, password_hash, full_name, role)
		 VALUES ('admin2@HPay.dev', 'x', 'Dev Admin 2', 'ops')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("seed admin2: %v", err)
	}
	return id
}

func (e *Env) SetScore(score int) {
	e.dial.mu.Lock()
	defer e.dial.mu.Unlock()
	e.dial.score = score
}

// seedAdminID returns the migration-seeded dev admin (000014).
func (e *Env) SeedAdminID(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.Pool.QueryRow(context.Background(),
		`SELECT id FROM admin_users WHERE email='admin@HPay.dev'`).Scan(&id); err != nil {
		t.Fatalf("seed admin missing — run migrations: %v", err)
	}
	return id
}

func NewEnv(t *testing.T, defName string) *Env {
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
	authSvc.SetTOTPKey([]byte("0123456789abcdef0123456789abcdef")) // ADR-0017: tests use a fixed key
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
	svc.SetAuthenticator(authSvc)
	return &Env{Pool: pool, Q: store.New(pool), Engine: engine, Worker: worker, Reg: reg,
		Svc: svc, Auth: authSvc, Wsvc: wallet.NewService(pool),
		Asvc: asvc, Osvc: org.NewService(pool, engine, fees),
		Msvc:   moneyrequest.NewService(pool, engine, fees),
		QrSvc:  qr.NewService(signer),
		Ksvc:   kyc.NewService(pool, engine, files),
		Kadmin: kyc.NewAdminService(pool, signer),
		dial:   dial,
		Fadmin: fraud.NewAdminService(pool),
		Rsvc:   recon.NewService(pool, files, signer),
		Revsvc: reversal.NewService(pool),
		Files:  files, Gw: mockGW}
}

func (e *Env) Drain(t *testing.T) {
	t.Helper()
	for i := 0; i < 30; i++ {
		n, err := e.Worker.PumpLocal(context.Background(), 100)
		if err != nil {
			t.Fatalf("pump: %v", err)
		}
		if n == 0 {
			return
		}
	}
	t.Fatal("outbox never drained")
}

func (e *Env) UserWithMoney(t *testing.T, amount int64) (uuid.UUID, uuid.UUID, string) {
	t.Helper()
	userID, walletID := testutil.NewUserWithWallet(t, e.Pool)
	var phone string
	if err := e.Pool.QueryRow(context.Background(),
		`SELECT phone FROM users WHERE id = $1`, userID).Scan(&phone); err != nil {
		t.Fatal(err)
	}
	if amount > 0 {
		if _, err := e.Wsvc.Topup(context.Background(), userID, amount, uuid.NewString()); err != nil {
			t.Fatalf("fund: %v", err)
		}
	}
	return userID, walletID, phone
}

func (e *Env) TxnLedgerNet(t *testing.T, refCode string, walletID uuid.UUID) int64 {
	t.Helper()
	var net int64
	err := e.Pool.QueryRow(context.Background(), `
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
func PostTransfer(ctx context.Context, pool *pgxpool.Pool, initiator, from, to uuid.UUID, amount int64) error {
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
	if err := ledger.Post(ctx, tx, txn.ID, ledger.KindForward, []ledger.Leg{
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

func Fund(t *testing.T, pool *pgxpool.Pool, owner, walletID uuid.UUID, amount int64) {
	t.Helper()
	cashIn := testutil.SystemWallet(t, pool, "SYSTEM_CASH_IN")
	if err := PostTransfer(context.Background(), pool, owner, cashIn, walletID, amount); err != nil {
		t.Fatalf("fund: %v", err)
	}
}

func ErrIs(err, target error) bool {
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

package money

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/hieumai1507/hpay-backend/internal/api"
	"github.com/hieumai1507/hpay-backend/internal/outbox"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestEdge_E17_RedisDownNoDataLoss: with Redis fully down the API still
// accepts a transfer — transaction, workflow instance and outbox rows land in
// postgres (the source of truth); healthz reports degraded. When Redis comes
// back the relay + consumers finish the transfer with no data loss.
func TestEdge_E17_RedisDownNoDataLoss(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// "Down" = a client pointed at a port nothing listens on.
	deadRdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond})
	defer deadRdb.Close()

	senderID, senderWallet, _ := e.UserWithMoney(t, 2_000_000)
	_, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)

	// 1. Transfer is accepted while Redis is down — DB + outbox only.
	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 1_000_000, "e17", uuid.NewString())
	if err != nil {
		t.Fatalf("transfer must not depend on redis: %v", err)
	}
	var instID uuid.UUID
	if err := e.Pool.QueryRow(ctx, `
		SELECT wi.id FROM workflow_instances wi
		JOIN transactions tx ON tx.id = wi.transaction_id
		WHERE tx.ref_code = $1`, res.RefCode).Scan(&instID); err != nil {
		t.Fatalf("workflow instance must exist: %v", err)
	}
	var pending int
	if err := e.Pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE published_at IS NULL AND payload->>'instance_id' = $1`,
		instID.String()).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending == 0 {
		t.Fatal("start event must sit in the outbox while redis is down")
	}

	// 2. The relay against dead redis publishes nothing and loses nothing.
	deadRelay := outbox.NewRelay(e.Pool, deadRdb, log)
	if n, _ := deadRelay.RelayOnce(ctx, 100); n != 0 {
		t.Fatalf("relay must publish nothing while redis is down, got %d", n)
	}
	if err := e.Pool.QueryRow(ctx,
		`SELECT count(*) FROM outbox WHERE published_at IS NULL AND payload->>'instance_id' = $1`,
		instID.String()).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending == 0 {
		t.Fatal("outbox rows must survive a failed relay pass")
	}

	// 3. healthz is degraded (503) but the process serves it.
	router := api.NewRouter(api.Deps{
		Cfg: config.Config{Env: "dev", Role: config.RoleAPI, JWTSecret: "integration-test-secret-0123456789",
			KYCStorageDir: t.TempDir(), StorageProvider: "local", MockBankSecret: "test-secret"},
		Log: log, Pool: e.Pool, Redis: deadRdb,
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("healthz with redis down: want 503, got %d", rec.Code)
	}

	// 4. Redis is back: real relay + a real-redis worker finish the transfer.
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Log("TEST_REDIS_ADDR not set — persistence verified, recovery leg skipped")
		return
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis at %s unreachable: %v", addr, err)
	}

	worker := workflow.NewWorker(e.Pool, rdb, e.Reg, e.Engine, log)
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go worker.Run(wctx)

	relay := outbox.NewRelay(e.Pool, rdb, log)
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := relay.RelayOnce(ctx, 100); err != nil {
			t.Fatalf("relay after recovery: %v", err)
		}
		var status string
		if err := e.Pool.QueryRow(ctx,
			`SELECT status FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == string(store.TxnStatusCompleted) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("transfer never completed after redis recovery; status=%s", status)
		}
		time.Sleep(200 * time.Millisecond)
	}

	wantSender := int64(2_000_000) - res.Amount - res.Fee
	if got := testutil.WalletBalance(t, e.Pool, senderWallet); got != wantSender {
		t.Fatalf("sender balance after recovery: want %d, got %d", wantSender, got)
	}
	if got := testutil.WalletBalance(t, e.Pool, receiverWallet); got != 1_000_000 {
		t.Fatalf("receiver balance after recovery: want 1000000, got %d", got)
	}
}

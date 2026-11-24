//go:build loadtest

// Load test for the P2P transfer path: request -> confirm with PIN -> workflow
// (fee, limits, ledger, outbox) against real Postgres. Not part of the normal suite.
//
//	LOAD_USERS=200 LOAD_WORKERS=50 LOAD_SECONDS=60 go test -tags loadtest ./test/loadtest -run TestP2PLoad -v -count=1
package loadtest

import (
	"context"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/hieumai1507/hpay-backend/internal/outbox"
	"github.com/hieumai1507/hpay-backend/internal/workflow"

	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func envInt(k string, d int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return d
}

type acct struct {
	user   uuid.UUID
	wallet uuid.UUID
	phone  string
}

func pct(d []time.Duration, p float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	return d[int(float64(len(d)-1)*p)]
}

func TestP2PLoad(t *testing.T) {
	users, workers, secs := envInt("LOAD_USERS", 200), envInt("LOAD_WORKERS", 50), envInt("LOAD_SECONDS", 60)
	e := testkit.NewEnv(t, "")
	ctx := context.Background()

	accts := make([]acct, users)
	for i := range accts {
		u, w, p := e.UserWithMoney(t, 8_000)
		e.SetPIN(t, u, testkit.ValidPIN)
		accts[i] = acct{u, w, p}
	}
	e.Drain(t)
	sumBalances := func() int64 {
		var s int64
		for _, a := range accts {
			var b int64
			if err := e.Pool.QueryRow(ctx, `SELECT balance FROM wallets WHERE id=$1`, a.wallet).Scan(&b); err != nil {
				t.Fatal(err)
			}
			s += b
		}
		return s
	}
	before := sumBalances()

	// Production path: outbox relay -> Redis Streams -> workflow workers (as cmd/server wires it).
	rdb := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR")})
	_ = rdb.FlushDB(ctx).Err()
	lg := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	pumpCtx, stopPump := context.WithCancel(ctx)
	var pumpWG sync.WaitGroup
	pumpWG.Add(2)
	go func() { defer pumpWG.Done(); outbox.NewRelay(e.Pool, rdb, lg).Run(pumpCtx) }()
	go func() { defer pumpWG.Done(); workflow.NewWorker(e.Pool, rdb, e.Reg, e.Engine, lg).Run(pumpCtx) }()

	var ok, fail atomic.Int64
	errKinds := sync.Map{}
	lat := make([][]time.Duration, workers)
	deadline := time.Now().Add(time.Duration(secs) * time.Second)
	start := time.Now()
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(w)))
			for time.Now().Before(deadline) {
				s := accts[r.Intn(users)]
				rc := accts[r.Intn(users)]
				if s.user == rc.user {
					continue
				}
				t0 := time.Now()
				res, err := e.Svc.ExecuteEngineRequest(ctx, transfer.EngineRequestInput{
					ServiceCode: "P2P_TRANSFER", SenderUserID: s.user, ReceiverPhone: rc.phone,
					Amount: 4 + int64(r.Intn(36)), Idempotency: wallet.NewRefCode(),
				})
				if err == nil && res.Status == "awaiting_auth" {
					_, err = e.Svc.ExecuteEngineConfirm(ctx, transfer.EngineConfirmInput{
						TransactionID: res.TransactionID, SenderUserID: s.user, PIN: testkit.ValidPIN,
					})
				}
				if err != nil {
					fail.Add(1)
					k := err.Error()
					if len(k) > 80 {
						k = k[:80]
					}
					v, _ := errKinds.LoadOrStore(k, new(atomic.Int64))
					v.(*atomic.Int64).Add(1)
					continue
				}
				lat[w] = append(lat[w], time.Since(t0))
				ok.Add(1)
			}
		}(w)
	}
	wg.Wait()
	accepted := time.Since(start)

	// Wait for every started workflow to finish.
	drainStart := time.Now()
	for {
		var pending int
		_ = e.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE type='p2p' AND created_at >= $1 AND status IN ('processing','processing_async')`, start).Scan(&pending)
		if pending == 0 || time.Since(drainStart) > 5*time.Minute {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	total := time.Since(start)
	stopPump()
	pumpWG.Wait()

	var all []time.Duration
	for _, l := range lat {
		all = append(all, l...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

	var completed, failedTx int
	_ = e.Pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='completed'), count(*) FILTER (WHERE status='failed') FROM transactions WHERE type='p2p' AND created_at >= $1`, start).Scan(&completed, &failedTx)

	// Integrity: every wallet's balance equals its ledger sum; money among test users is conserved
	// except fees moved to the system fee wallet.
	var mismatched int
	_ = e.Pool.QueryRow(ctx, `
		SELECT count(*) FROM wallets w
		WHERE w.balance <> COALESCE((SELECT SUM(CASE le.direction WHEN 'credit' THEN le.amount ELSE -le.amount END)
		                             FROM ledger_entries le WHERE le.wallet_id = w.id), 0)`).Scan(&mismatched)
	var fees int64
	_ = e.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(fee),0) FROM transactions WHERE type='p2p' AND status='completed' AND created_at >= $1`, start).Scan(&fees)
	after := sumBalances()

	t.Logf("users=%d workers=%d duration=%ds", users, workers, secs)
	t.Logf("accepted (request+PIN confirm): ok=%d fail=%d  -> %.1f req/s", ok.Load(), fail.Load(), float64(ok.Load())/accepted.Seconds())
	t.Logf("latency request+confirm: p50=%v p95=%v p99=%v max=%v", pct(all, .50), pct(all, .95), pct(all, .99), pct(all, 1))
	t.Logf("settled: completed=%d failed=%d in %.1fs -> %.1f completed transfers/s", completed, failedTx, total.Seconds(), float64(completed)/total.Seconds())
	var c50, c95, c99 float64
	_ = e.Pool.QueryRow(ctx, `SELECT COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY ms),0), COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY ms),0), COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY ms),0)
		FROM (SELECT EXTRACT(EPOCH FROM completed_at-created_at)*1000 AS ms FROM transactions WHERE type='p2p' AND status='completed' AND created_at >= $1) x`, start).Scan(&c50, &c95, &c99)
	var stuck int
	_ = e.Pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE type='p2p' AND created_at >= $1 AND status IN ('processing','processing_async')`, start).Scan(&stuck)
	t.Logf("end-to-end (created -> completed): p50=%.0fms p95=%.0fms p99=%.0fms; still processing=%d", c50, c95, c99, stuck)
	t.Logf("integrity: wallets with balance != ledger sum = %d; user money before=%d after=%d fees=%d delta=%d",
		mismatched, before, after, fees, before-after-fees)
	errKinds.Range(func(k, v any) bool { t.Logf("error %q x%d", k, v.(*atomic.Int64).Load()); return true })

	if mismatched != 0 {
		t.Errorf("ledger mismatch on %d wallets", mismatched)
	}
	if before-after != fees {
		t.Errorf("money not conserved: before-after=%d fees=%d", before-after, fees)
	}
}

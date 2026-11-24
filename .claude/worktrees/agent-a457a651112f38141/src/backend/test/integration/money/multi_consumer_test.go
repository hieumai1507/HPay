package money

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/hieumai1507/hpay-backend/internal/outbox"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestP1_ManyConsumersFinishEveryTransferOnce runs two workers with several
// consumers each on the same Redis streams (real relay) and checks that every
// transfer completes once: money moves exactly once, ledger sums match, and
// each step has a single execution row per attempt.
func TestP1_ManyConsumersFinishEveryTransferOnce(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 9}) // own DB: stray consumers must not touch other tests' streams
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unreachable: %v", err)
	}
	_ = rdb.FlushDB(ctx).Err()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	const nTransfers = 40
	sender, senderWallet, _ := e.UserWithMoney(t, 2_000_000)
	_, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)

	refs := make([]string, 0, nTransfers)
	for i := 0; i < nTransfers; i++ {
		res, err := e.Svc.Transfer(ctx, sender, receiverPhone, 10_000, "p1", uuid.NewString())
		if err != nil {
			t.Fatalf("transfer %d: %v", i, err)
		}
		refs = append(refs, res.RefCode)
	}

	wctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for i := 0; i < 2; i++ {
		w := workflow.NewWorker(e.Pool, rdb, e.Reg, e.Engine, log)
		w.SetConcurrency(4)
		go w.Run(wctx)
	}
	go outbox.NewRelay(e.Pool, rdb, log).Run(wctx)

	deadline := time.Now().Add(90 * time.Second)
	for {
		var done int
		if err := e.Pool.QueryRow(ctx,
			`SELECT count(*) FROM transactions WHERE ref_code = ANY($1) AND status='completed'`, refs).Scan(&done); err != nil {
			t.Fatal(err)
		}
		if done == nTransfers {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d/%d transfers completed", done, nTransfers)
		}
		time.Sleep(200 * time.Millisecond)
	}

	if got := testutil.WalletBalance(t, e.Pool, receiverWallet); got != int64(nTransfers)*10_000 {
		t.Fatalf("receiver balance: want %d, got %d (money moved more or less than once)", nTransfers*10_000, got)
	}
	var mismatched int
	if err := e.Pool.QueryRow(ctx, `
		SELECT count(*) FROM wallets w WHERE w.id = ANY($1) AND w.balance <>
		COALESCE((SELECT SUM(CASE le.direction WHEN 'credit' THEN le.amount ELSE -le.amount END)
		          FROM ledger_entries le WHERE le.wallet_id = w.id), 0)`,
		[]uuid.UUID{senderWallet, receiverWallet}).Scan(&mismatched); err != nil {
		t.Fatal(err)
	}
	if mismatched != 0 {
		t.Fatalf("%d wallets differ from their ledger sum", mismatched)
	}
	var dup int
	if err := e.Pool.QueryRow(ctx, `
		SELECT count(*) FROM (SELECT instance_id, step_name, attempt FROM step_executions se
		  JOIN workflow_instances wi ON wi.id = se.instance_id
		  JOIN transactions tx ON tx.id = wi.transaction_id
		  WHERE tx.ref_code = ANY($1)
		  GROUP BY 1,2,3 HAVING count(*) > 1) d`, refs).Scan(&dup); err != nil {
		t.Fatal(err)
	}
	if dup != 0 {
		t.Fatalf("%d step executions ran twice", dup)
	}
}

package money

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestEdge_E02_ConcurrentOverdraw: wallet of 100k, 2 goroutines both withdraw 70k —
// exactly one succeeds, final balance 30k, never negative.
func TestEdge_E02_ConcurrentOverdraw(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	ctx := context.Background()
	userA, walletA := testutil.NewUserWithWallet(t, pool)
	_, walletB := testutil.NewUserWithWallet(t, pool)
	testkit.Fund(t, pool, userA, walletA, 400)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = testkit.PostTransfer(ctx, pool, userA, walletA, walletB, 280)
		}(i)
	}
	wg.Wait()

	var fails int
	for _, err := range errs {
		if err != nil {
			fails++
			if !testkit.ErrIs(err, ledger.ErrInsufficientBalance) {
				t.Fatalf("unexpected error: %v", err)
			}
		}
	}
	if fails != 1 {
		t.Fatalf("want exactly 1 failure, got %d (errs=%v)", fails, errs)
	}
	if got := testutil.WalletBalance(t, pool, walletA); got != 120 {
		t.Fatalf("wallet A: want 120, got %d", got)
	}
	if got := testutil.WalletBalance(t, pool, walletB); got != 280 {
		t.Fatalf("wallet B: want 280, got %d", got)
	}
	if testutil.WalletBalance(t, pool, walletA) != testutil.LedgerSum(t, pool, walletA) {
		t.Fatal("wallet A: balance != ledger sum")
	}
}

// TestEdge_E03_CrossTransferNoDeadlock: A→B and B→A concurrently — lock ordering
// ensures both complete, no deadlock.
func TestEdge_E03_CrossTransferNoDeadlock(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	ctx := context.Background()
	userA, walletA := testutil.NewUserWithWallet(t, pool)
	userB, walletB := testutil.NewUserWithWallet(t, pool)
	testkit.Fund(t, pool, userA, walletA, 200)
	testkit.Fund(t, pool, userB, walletB, 200)

	done := make(chan error, 2)
	go func() { done <- testkit.PostTransfer(ctx, pool, userA, walletA, walletB, 40) }()
	go func() { done <- testkit.PostTransfer(ctx, pool, userB, walletB, walletA, 40) }()

	timeout := time.After(10 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("transfer %d failed: %v", i, err)
			}
		case <-timeout:
			t.Fatal("deadlock suspected: transfers did not finish in 10s")
		}
	}
	if a, b := testutil.WalletBalance(t, pool, walletA), testutil.WalletBalance(t, pool, walletB); a != 200 || b != 200 {
		t.Fatalf("balances after cross transfer: A=%d B=%d, want 200/200", a, b)
	}
}

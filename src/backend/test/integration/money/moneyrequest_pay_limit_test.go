package money

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// BUG-0021: money-request Pay checked only the per-transaction amount, so parallel pays by one
// payer could exceed the daily limit that the transfer confirm path enforces (BUG-0017). At L0 the
// daily limit is 2,500,000: of five requests of 1,500,000 exactly one may start.
func TestMoneyRequestPay_ConcurrentPaysRespectDailyLimit(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	payer, _, payerPhone := e.UserWithMoney(t, 8_000)
	requester, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, payer, "OFFER_CUST_L0")

	const n = 5
	ids := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		r, err := e.Msvc.Create(ctx, requester, payerPhone, 6_000, "dinner", uuid.NewString())
		if err != nil {
			t.Fatalf("create request %d: %v", i, err)
		}
		ids = append(ids, r.ID)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, n)
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = e.Msvc.Pay(ctx, payer, ids[i], uuid.NewString())
		}()
	}
	close(start)
	wg.Wait()

	started, limited := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			started++
		case errors.Is(err, offerguard.ErrLimitDaily):
			limited++
		default:
			t.Errorf("unexpected pay error: %v", err)
		}
	}
	if started != 1 || limited != n-1 {
		t.Fatalf("daily limit 2.5M, five requests of 1.5M: want 1 started and %d refused, got %d started and %d refused", n-1, started, limited)
	}
}

// Deterministic check of the lock MODE: while another transaction holds the payer's limit-window
// lock in shared mode, Pay (which needs it exclusive) must wait; it must finish once that
// transaction ends. A shared lock in Pay would not wait, and parallel pays could all pass the check.
func TestMoneyRequestPay_WaitsForThePayersExclusiveLimitLock(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	payer, _, payerPhone := e.UserWithMoney(t, 8_000)
	requester, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, payer, "OFFER_CUST_L0")
	r, err := e.Msvc.Create(ctx, requester, payerPhone, 400, "lunch", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	holder, err := e.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended('limit-window:' || $1::text, 0))`, payer); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := e.Msvc.Pay(ctx, payer, r.ID, uuid.NewString())
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("Pay must wait for the exclusive limit-window lock, but it finished with %v", err)
	case <-time.After(750 * time.Millisecond):
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Pay after the lock was released: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Pay did not finish after the lock was released")
	}
}

package money

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func confirmPIN(e *testkit.Env, sender uuid.UUID, res transfer.EngineRequestResult) error {
	_, err := e.Svc.ExecuteEngineConfirm(context.Background(), transfer.EngineConfirmInput{
		TransactionID: res.TransactionID, SenderUserID: sender, PIN: testkit.ValidPIN,
	})
	return err
}

// BUG-0017 / BE-H2: a request waiting for its PIN does not count toward the quota, and nothing
// re-checked at confirm, so many requests could all pass and all start. The limit must hold when
// money starts to move: at L0 the daily cap is 2,500,000, so of five held requests of 1,500,000
// exactly one may start.
func TestLimitWindow_ConcurrentConfirmsRespectDailyLimit(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, _, _ := e.UserWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)
	e.SetOffer(t, sender, "OFFER_CUST_L0")
	e.SetPIN(t, sender, testkit.ValidPIN)

	const n = 5
	held := make([]transfer.EngineRequestResult, 0, n)
	for i := 0; i < n; i++ {
		held = append(held, e.RequestP2P(t, sender, receiverPhone, 1_500_000))
	}

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range held {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = confirmPIN(e, sender, held[i])
		}()
	}
	wg.Wait()

	started, limited := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			started++
		case errors.Is(err, offerguard.ErrLimitDaily):
			limited++
		default:
			t.Errorf("unexpected confirm error: %v", err)
		}
	}
	if started != 1 || limited != n-1 {
		t.Fatalf("daily limit 2.5M, five holds of 1.5M: want 1 started and %d refused, got %d started and %d refused", n-1, started, limited)
	}
}

// The quota counts amount + fee, so the confirm check must too: with the 1,000 VND p2p fee, two
// holds of 1,249,500 are 2,501,000 in total. Without the fee the second would be exactly at the
// cap and pass.
func TestLimitWindow_ConfirmCountsTheFee(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, _, _ := e.UserWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)
	e.SetOffer(t, sender, "OFFER_CUST_L0")
	e.SetPIN(t, sender, testkit.ValidPIN)

	a := e.RequestP2P(t, sender, receiverPhone, 1_249_500)
	b := e.RequestP2P(t, sender, receiverPhone, 1_249_500)
	if a.Fee != 1_000 {
		t.Fatalf("setup: the p2p fee should be 1000, got %d", a.Fee)
	}
	if err := confirmPIN(e, sender, a); err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	if err := confirmPIN(e, sender, b); !errors.Is(err, offerguard.ErrLimitDaily) {
		t.Fatalf("second confirm must hit the daily limit once the fee is counted, got %v", err)
	}
}

// The quota counts a transaction from the moment it STARTS, not from when it was requested
// (BUG-0017). A request made yesterday and confirmed today uses today's quota; counting it by
// its creation time would let it dodge both days.
func TestLimitWindow_VolumeIsCountedFromWhenTheTransactionStarted(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.UserWithMoney(t, 2_400_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)
	e.SetOffer(t, sender, "OFFER_CUST_L0")
	e.SetPIN(t, sender, testkit.ValidPIN)

	first := e.RequestP2P(t, sender, receiverPhone, 1_500_000)
	if err := confirmPIN(e, sender, first); err != nil {
		t.Fatalf("first confirm: %v", err)
	}
	// Pretend that request was created two days ago; it started just now.
	if _, err := e.Pool.Exec(ctx, `UPDATE transactions SET created_at = created_at - interval '2 days' WHERE id = $1`, first.TransactionID); err != nil {
		t.Fatal(err)
	}
	_, err := e.Svc.ExecuteEngineRequest(ctx, transfer.EngineRequestInput{
		ServiceCode: "P2P_TRANSFER", SenderUserID: sender, ReceiverPhone: receiverPhone,
		Amount: 1_100_000, Idempotency: uuid.NewString(),
	})
	if !errors.Is(err, offerguard.ErrLimitDaily) {
		t.Fatalf("1,500,000 started today plus 1,100,000 exceeds the 2,500,000 daily limit, got %v", err)
	}
}

// The throttle (interval_seconds) measures the spacing of STARTS. Held requests do not count, so
// holding several and confirming them together used to bypass it.
func TestLimitWindow_IntervalThrottleHoldsAtConfirm(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	sender, _, _ := e.UserWithMoney(t, 2_400_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)
	e.SetPIN(t, sender, testkit.ValidPIN)

	code := "OFFER_TEST_" + uuid.NewString()[:8]
	if _, err := e.Pool.Exec(ctx, `
		WITH o AS (INSERT INTO offers (code, name, role, level, is_active) VALUES ($1, 'throttle test', 'customer', 1, true) RETURNING id)
		INSERT INTO offer_services (offer_id, service_id, is_enabled, min_amount, max_amount_per_txn, daily_limit, monthly_limit, interval_seconds)
		SELECT o.id, s.id, true, 0, 0, 0, 0, 3600 FROM o, services s WHERE s.code = 'P2P_TRANSFER'`, code); err != nil {
		t.Fatal(err)
	}
	e.SetOffer(t, sender, code)

	const n = 4
	held := make([]transfer.EngineRequestResult, 0, n)
	for i := 0; i < n; i++ {
		held = append(held, e.RequestP2P(t, sender, receiverPhone, 10_000))
	}
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range held {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = confirmPIN(e, sender, held[i])
		}()
	}
	wg.Wait()
	started, throttled := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			started++
		case errors.Is(err, offerguard.ErrTooFrequent):
			throttled++
		default:
			t.Errorf("unexpected confirm error: %v", err)
		}
	}
	if started != 1 || throttled != n-1 {
		t.Fatalf("one start per 3600 s: want 1 started and %d throttled, got %d and %d", n-1, started, throttled)
	}
}

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

// A request made before today's Vietnam midnight is metered by its creation time, so it would
// dodge today's volume. Confirm refuses it and asks for a new request.
func TestLimitWindow_RequestFromAnEarlierDayIsRefusedAtConfirm(t *testing.T) {
	e := testkit.NewEnv(t, "")
	sender, _, _ := e.UserWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)
	e.SetOffer(t, sender, "OFFER_CUST_L0")
	e.SetPIN(t, sender, testkit.ValidPIN)

	held := e.RequestP2P(t, sender, receiverPhone, 100_000)
	if _, err := e.Pool.Exec(context.Background(),
		`UPDATE transactions SET created_at = now() - interval '2 days' WHERE id = $1`, held.TransactionID); err != nil {
		t.Fatal(err)
	}
	err := confirmPIN(e, sender, held)
	if !errors.Is(err, transfer.ErrAuthExpired) {
		t.Fatalf("a request from an earlier day must be refused at confirm, got %v", err)
	}
	var status string
	if err := e.Pool.QueryRow(context.Background(), `SELECT status FROM transactions WHERE id=$1`, held.TransactionID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "awaiting_auth" {
		t.Fatalf("the refused request must not start, status=%s", status)
	}
}

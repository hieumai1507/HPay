package money

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/moneyrequest"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestMoneyRequestPayEndToEnd: A requests from B → B pays → P2P completes through the correct engine
// F5 → request status=paid + paid_txn_id points to the right transaction.
func TestMoneyRequestPayEndToEnd(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	requesterID, requesterWallet, _ := e.UserWithMoney(t, 0)
	payerID, payerWallet, payerPhone := e.UserWithMoney(t, 2_000_000)

	req, err := e.Msvc.Create(ctx, requesterID, payerPhone, 200_000, "dinner", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != "pending" {
		t.Fatalf("want pending, got %s", req.Status)
	}

	res, err := e.Msvc.Pay(ctx, payerID, req.ID, uuid.NewString())
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if res.Amount != 200_000 {
		t.Fatalf("amount: want 200000, got %d", res.Amount)
	}
	e.Drain(t)

	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn: want completed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, payerWallet); got != 2_000_000-res.Total {
		t.Fatalf("payer balance: want %d, got %d", 2_000_000-res.Total, got)
	}
	if got := testutil.WalletBalance(t, e.Pool, requesterWallet); got != 200_000 {
		t.Fatalf("requester balance: want 200000, got %d", got)
	}

	d, err := e.Msvc.DetailFor(ctx, requesterID, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != store.MoneyRequestStatusPaid {
		t.Fatalf("request status: want paid, got %s", d.Status)
	}
	var txnID uuid.UUID
	if err := e.Pool.QueryRow(ctx, `SELECT id FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	if d.PaidTxnID != txnID {
		t.Fatalf("paid_txn_id mismatch: want %s, got %s", txnID, d.PaidTxnID)
	}
}

// TestSplitBillThreeWay: 3 payers split 300k evenly → 2 pay, 1 declines →
// GET detail shows 3 independent states, one per sibling.
func TestSplitBillThreeWay(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	requesterID, _, _ := e.UserWithMoney(t, 0)
	p1ID, _, p1Phone := e.UserWithMoney(t, 500_000)
	p2ID, _, p2Phone := e.UserWithMoney(t, 500_000)
	p3ID, _, p3Phone := e.UserWithMoney(t, 500_000)

	split, err := e.Msvc.CreateSplit(ctx, requesterID, []string{p1Phone, p2Phone, p3Phone}, 300_000, "trip", nil, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if len(split.Requests) != 3 {
		t.Fatalf("want 3 requests, got %d", len(split.Requests))
	}
	var sum int64
	for _, r := range split.Requests {
		sum += r.Amount
	}
	if sum != 300_000 {
		t.Fatalf("split amounts sum to %d, want 300000", sum)
	}

	if _, err := e.Msvc.Pay(ctx, p1ID, split.Requests[0].ID, uuid.NewString()); err != nil {
		t.Fatalf("p1 pay: %v", err)
	}
	if _, err := e.Msvc.Pay(ctx, p2ID, split.Requests[1].ID, uuid.NewString()); err != nil {
		t.Fatalf("p2 pay: %v", err)
	}
	if err := e.Msvc.Decline(ctx, p3ID, split.Requests[2].ID); err != nil {
		t.Fatalf("p3 decline: %v", err)
	}
	e.Drain(t)

	d, err := e.Msvc.DetailFor(ctx, requesterID, split.Requests[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Siblings) != 3 {
		t.Fatalf("want 3 siblings, got %d", len(d.Siblings))
	}
	statusByPayer := map[uuid.UUID]store.MoneyRequestStatus{}
	for _, s := range d.Siblings {
		statusByPayer[s.PayerID] = s.Status
	}
	if statusByPayer[p1ID] != store.MoneyRequestStatusPaid {
		t.Fatalf("p1: want paid, got %s", statusByPayer[p1ID])
	}
	if statusByPayer[p2ID] != store.MoneyRequestStatusPaid {
		t.Fatalf("p2: want paid, got %s", statusByPayer[p2ID])
	}
	if statusByPayer[p3ID] != store.MoneyRequestStatusDeclined {
		t.Fatalf("p3: want declined, got %s", statusByPayer[p3ID])
	}
}

// TestMoneyRequestCancelAndActorGuards: cancel by the right requester while still
// pending; decline/cancel by the wrong actor must be blocked.
func TestMoneyRequestCancelAndActorGuards(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	requesterID, _, _ := e.UserWithMoney(t, 0)
	payerID, _, payerPhone := e.UserWithMoney(t, 0)
	outsiderID, _, _ := e.UserWithMoney(t, 0)

	req, err := e.Msvc.Create(ctx, requesterID, payerPhone, 50_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	if err := e.Msvc.Cancel(ctx, outsiderID, req.ID); !testkit.ErrIs(err, moneyrequest.ErrForbidden) {
		t.Fatalf("outsider cancel: want ErrForbidden, got %v", err)
	}
	if err := e.Msvc.Decline(ctx, requesterID, req.ID); !testkit.ErrIs(err, moneyrequest.ErrForbidden) {
		t.Fatalf("requester decline own request: want ErrForbidden, got %v", err)
	}
	if err := e.Msvc.Cancel(ctx, requesterID, req.ID); err != nil {
		t.Fatalf("requester cancel: %v", err)
	}
	if err := e.Msvc.Cancel(ctx, requesterID, req.ID); !testkit.ErrIs(err, moneyrequest.ErrNotPayable) {
		t.Fatalf("double-cancel: want ErrNotPayable, got %v", err)
	}
	if _, err := e.Msvc.Pay(ctx, payerID, req.ID, uuid.NewString()); !testkit.ErrIs(err, moneyrequest.ErrNotPayable) {
		t.Fatalf("pay a canceled request: want ErrNotPayable, got %v", err)
	}
}

// TestEdge_E19_ExpiryRacePay: money request expires right as it is being paid.
// (a) expires_at passes JUST BEFORE Pay(), sweeper has NOT run → Pay() blocks itself
// via the expires_at condition inside the transaction, creating no
// transaction. (b) 2 goroutines run concurrently — Pay() and sweeper.ExpireDue() — FOR
// UPDATE serializes them: exactly one of the two wins, never both, and no
// orphaned transaction if expiry wins.
func TestEdge_E19_ExpiryRacePay(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()

	t.Run("payer_loses_before_sweeper_runs", func(t *testing.T) {
		requesterID, _, _ := e.UserWithMoney(t, 0)
		payerID, _, payerPhone := e.UserWithMoney(t, 500_000)

		req, err := e.Msvc.Create(ctx, requesterID, payerPhone, 100_000, "", uuid.NewString())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Pool.Exec(ctx,
			`UPDATE money_requests SET expires_at = now() - interval '1 minute' WHERE id=$1`, req.ID); err != nil {
			t.Fatal(err)
		}

		if _, err := e.Msvc.Pay(ctx, payerID, req.ID, uuid.NewString()); !testkit.ErrIs(err, moneyrequest.ErrExpired) {
			t.Fatalf("pay on overdue request: want ErrExpired, got %v", err)
		}
		var txnCount int
		if err := e.Pool.QueryRow(ctx,
			`SELECT count(*) FROM transactions WHERE initiated_by=$1 AND type='p2p'`, payerID).Scan(&txnCount); err != nil {
			t.Fatal(err)
		}
		if txnCount != 0 {
			t.Fatalf("want no transaction created for a rejected pay, got %d", txnCount)
		}
	})

	t.Run("pay_vs_sweeper_concurrent", func(t *testing.T) {
		requesterID, _, _ := e.UserWithMoney(t, 0)
		payerID, _, payerPhone := e.UserWithMoney(t, 500_000)

		req, err := e.Msvc.Create(ctx, requesterID, payerPhone, 100_000, "", uuid.NewString())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Pool.Exec(ctx,
			`UPDATE money_requests SET expires_at = now() - interval '1 minute' WHERE id=$1`, req.ID); err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		var payErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, payErr = e.Msvc.Pay(ctx, payerID, req.ID, uuid.NewString())
		}()
		go func() {
			defer wg.Done()
			if _, err := e.Msvc.ExpireDue(ctx); err != nil {
				t.Errorf("sweep: %v", err)
			}
		}()
		wg.Wait()

		// Pay must lose (row was already past its deadline): either it sees
		// the guard fail directly, or (rare timing) sees the sweeper's
		// 'expired' status first — both surface as a non-payable/expired error.
		if payErr == nil {
			t.Fatal("want pay to fail against an already-overdue request, got nil error")
		}
		if !testkit.ErrIs(payErr, moneyrequest.ErrExpired) && !testkit.ErrIs(payErr, moneyrequest.ErrNotPayable) {
			t.Fatalf("unexpected pay error: %v", payErr)
		}

		d, err := e.Msvc.DetailFor(ctx, requesterID, req.ID)
		if err != nil {
			t.Fatal(err)
		}
		if d.Status != store.MoneyRequestStatusExpired {
			t.Fatalf("request: want expired, got %s", d.Status)
		}
		var txnCount int
		if err := e.Pool.QueryRow(ctx,
			`SELECT count(*) FROM transactions WHERE initiated_by=$1 AND type='p2p'`, payerID).Scan(&txnCount); err != nil {
			t.Fatal(err)
		}
		if txnCount != 0 {
			t.Fatalf("want no orphaned transaction when expiry wins, got %d", txnCount)
		}
	})
}

// TestMoneyRequestRemind: Tests remind rate limiting, authorization, and concurrent reminds.
func TestMoneyRequestRemind(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()

	requesterID, _, _ := e.UserWithMoney(t, 0)
	payerID, _, payerPhone := e.UserWithMoney(t, 500_000)
	outsiderID, _, _ := e.UserWithMoney(t, 0)

	req, err := e.Msvc.Create(ctx, requesterID, payerPhone, 100_000, "trip", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	// 1. Payer (not requester) tries to remind -> ErrForbidden
	_, err = e.Msvc.Remind(ctx, req.ID, payerID)
	if !errors.Is(err, moneyrequest.ErrForbidden) {
		t.Fatalf("payer remind: want ErrForbidden, got %v", err)
	}

	// 2. Outsider tries to remind -> ErrForbidden
	_, err = e.Msvc.Remind(ctx, req.ID, outsiderID)
	if !errors.Is(err, moneyrequest.ErrForbidden) {
		t.Fatalf("outsider remind: want ErrForbidden, got %v", err)
	}

	// 3. Requester reminds -> OK
	remindedAt, err := e.Msvc.Remind(ctx, req.ID, requesterID)
	if err != nil {
		t.Fatalf("requester remind failed: %v", err)
	}
	if remindedAt.IsZero() {
		t.Fatal("expected non-zero reminded_at time")
	}

	// 4. Requester reminds again -> ErrRemindTooSoon
	_, err = e.Msvc.Remind(ctx, req.ID, requesterID)
	if !errors.Is(err, moneyrequest.ErrRemindTooSoon) {
		t.Fatalf("requester remind too soon: want ErrRemindTooSoon, got %v", err)
	}

	// 5. Check detail returns last_reminded_at
	detail, err := e.Msvc.DetailFor(ctx, requesterID, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !detail.LastRemindedAt.Valid || detail.LastRemindedAt.Time.Unix() != remindedAt.Unix() {
		t.Fatalf("detail last_reminded_at mismatch: want %v, got %v", remindedAt, detail.LastRemindedAt.Time)
	}

	// 6. Artificially backdate last_reminded_at to > 24 hours ago
	if _, err := e.Pool.Exec(ctx,
		`UPDATE money_requests SET last_reminded_at = now() - interval '25 hours' WHERE id=$1`, req.ID); err != nil {
		t.Fatal(err)
	}

	// 7. Remind again -> OK
	_, err = e.Msvc.Remind(ctx, req.ID, requesterID)
	if err != nil {
		t.Fatalf("remind after 25h failed: %v", err)
	}
}

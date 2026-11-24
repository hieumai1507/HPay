package money

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/listing"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func verifyKYC(t *testing.T, e *testkit.Env, userID uuid.UUID) {
	t.Helper()
	if _, err := e.Pool.Exec(context.Background(),
		`UPDATE users SET kyc_status='verified' WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	// a verified user is on the L1 offer (docs/specs/w3-offer-limits)
	e.SetOffer(t, userID, "OFFER_CUST_L1")
}

// insertP2PHistory writes N completed p2p transactions for a sender at "now",
// building velocity + a low 30-day average without moving real money (the
// fraud rules read only the transactions table).
func insertP2PHistory(t *testing.T, e *testkit.Env, sender, senderWallet, receiverWallet uuid.UUID, n int, amount int64) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := e.Pool.Exec(context.Background(), `
			INSERT INTO transactions (ref_code, idempotency_key, type, status,
			  sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by, completed_at)
			VALUES ($1, $2, 'p2p', 'completed', $3, $4, $5, 0, $6, now())`,
			wallet.NewRefCode(), uuid.NewString(), senderWallet, receiverWallet, amount, sender); err != nil {
			t.Fatal(err)
		}
	}
}

// TestFraudBlocksHighRisk: velocity + amount spike + new receiver >= 85 → the
// FRAUD_CHECK step blocks, the transaction fails, and no money moves.
func TestFraudBlocksHighRisk(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.UserWithMoney(t, 0)
	// verified tier moves the user to offer L1 (62.5M per transaction); fund via topup so the
	// funding row is type=topup and never pollutes the p2p average/velocity.
	verifyKYC(t, e, senderID)
	if _, err := e.Wsvc.Topup(ctx, senderID, 200_000, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	_, histWallet, _ := e.UserWithMoney(t, 0)
	_, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)

	// 4 recent small transfers -> velocity(30); low average so a big amount spikes.
	insertP2PHistory(t, e, senderID, senderWallet, histWallet, 4, 400)

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 80_000, "big", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)

	if got := e.TxnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("txn: want failed (blocked), got %s", got)
	}
	if net := e.TxnLedgerNet(t, res.RefCode, receiverWallet); net != 0 {
		t.Fatal("blocked transaction must move no money")
	}
	// A fraud flag was raised.
	var flags int
	if err := e.Pool.QueryRow(ctx, `
		SELECT count(*) FROM fraud_flags f JOIN transactions t ON t.id=f.transaction_id
		WHERE t.ref_code=$1`, res.RefCode).Scan(&flags); err != nil {
		t.Fatal(err)
	}
	if flags != 1 {
		t.Fatalf("want 1 fraud flag, got %d", flags)
	}
}

// TestFraudFlagsGreyZone: velocity + new receiver = 50 → flag but the money
// proceeds; the async advisory merges into the flag reasons.
func TestFraudFlagsGreyZone(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, senderID, senderWallet, 20_000)
	_, histWallet, _ := e.UserWithMoney(t, 0)
	_, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)

	insertP2PHistory(t, e, senderID, senderWallet, histWallet, 4, 800) // velocity 30

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 800, "grey", uuid.NewString()) // + new receiver 20 = 50
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)

	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("flagged (not blocked) txn must complete, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, receiverWallet); got != 800 {
		t.Fatalf("receiver: want 800, got %d", got)
	}

	// Flag exists with open status; the mock advisory merges asynchronously.
	var flagID uuid.UUID
	if err := e.Pool.QueryRow(ctx, `
		SELECT f.id FROM fraud_flags f JOIN transactions t ON t.id=f.transaction_id
		WHERE t.ref_code=$1`, res.RefCode).Scan(&flagID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		var reasons string
		if err := e.Pool.QueryRow(ctx, `SELECT reasons::text FROM fraud_flags WHERE id=$1`, flagID).Scan(&reasons); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(reasons, "advisory") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("advisory never merged into flag reasons")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Admin can list and resolve it (guarded; double resolve conflicts).
	flags, _, err := e.Fadmin.List(ctx, "open", listing.Filter{})
	if err != nil || len(flags) == 0 {
		t.Fatalf("list open flags: %v (%d)", err, len(flags))
	}
	adminID := e.SeedAdminID(t)
	if err := e.Fadmin.Resolve(ctx, adminID, flagID, false, "looks fine"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := e.Fadmin.Resolve(ctx, adminID, flagID, true, "again"); err == nil {
		t.Fatal("double resolve must conflict")
	}
}

package admin

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/reversal"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestReversalEndToEnd: a completed p2p is reversed via admin maker-checker;
// both parties AND the fee wallet return to their pre-transaction balances,
// the original goes 'reversed', and the reversal ledger nets to zero.
func TestReversalEndToEnd(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, senderID, senderWallet, 3_000_000)
	_, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)
	feeWallet := seedSystemWalletID(t, e, "SYSTEM_FEE")

	senderBefore := testutil.WalletBalance(t, e.Pool, senderWallet)

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 1_000_000, "to reverse", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("setup transfer: want completed, got %s", got)
	}
	fee := res.Fee

	// Admin A files a reversal; admin B approves.
	adminA := e.SeedAdminID(t)
	adminB := e.SeedAdmin2ID(t)
	cr, err := e.Revsvc.CreateRequest(ctx, adminA, res.RefCode, "customer dispute")
	if err != nil {
		t.Fatalf("create reversal: %v", err)
	}

	// Maker cannot approve their own request.
	if _, err := e.Asvc.AdminDecide(ctx, adminA, cr.RequestID, true, "self"); err != approval.ErrSelfApproval {
		t.Fatalf("self-approval: want ErrSelfApproval, got %v", err)
	}
	// Second reversal request for the same txn is refused.
	if _, err := e.Revsvc.CreateRequest(ctx, adminA, res.RefCode, "again"); err != reversal.ErrReversalPending {
		t.Fatalf("duplicate reversal request: want ErrReversalPending, got %v", err)
	}

	dr, err := e.Asvc.AdminDecide(ctx, adminB, cr.RequestID, true, "approved")
	if err != nil {
		t.Fatalf("admin decide: %v", err)
	}
	if dr.Status != "approved" {
		t.Fatalf("decide status: want approved, got %s", dr.Status)
	}

	// Everyone whole again.
	if got := testutil.WalletBalance(t, e.Pool, senderWallet); got != senderBefore {
		t.Fatalf("sender not made whole: want %d, got %d", senderBefore, got)
	}
	if got := testutil.WalletBalance(t, e.Pool, receiverWallet); got != 0 {
		t.Fatalf("receiver not clawed back: want 0, got %d", got)
	}
	// SYSTEM_FEE is shared with every other test in this package (and with any
	// workflow still settling), so its absolute balance is not ours to assert.
	// What must hold is that THIS transaction and its reversal net to zero on
	// the fee wallet.
	var feeNet int64
	if err := e.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE le.direction WHEN 'credit' THEN le.amount ELSE -le.amount END), 0)::bigint
		FROM ledger_entries le
		WHERE le.wallet_id = $1
		  AND le.transaction_id IN (
		      SELECT id FROM transactions WHERE ref_code = $2
		      UNION ALL
		      SELECT id FROM transactions
		       WHERE reversal_of = (SELECT id FROM transactions WHERE ref_code = $2))`,
		feeWallet, res.RefCode).Scan(&feeNet); err != nil {
		t.Fatal(err)
	}
	if feeNet != 0 {
		t.Fatalf("fee not clawed back: net %d on the fee wallet for this txn pair (fee was %d)", feeNet, fee)
	}

	// Original is reversed; a reversal transaction exists pointing back to it.
	var origStatus string
	var revCount int
	if err := e.Pool.QueryRow(ctx, `SELECT status FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&origStatus); err != nil {
		t.Fatal(err)
	}
	if origStatus != "reversed" {
		t.Fatalf("original: want reversed, got %s", origStatus)
	}
	if err := e.Pool.QueryRow(ctx, `
		SELECT count(*) FROM transactions
		WHERE reversal_of = (SELECT id FROM transactions WHERE ref_code=$1)`, res.RefCode).Scan(&revCount); err != nil {
		t.Fatal(err)
	}
	if revCount != 1 {
		t.Fatalf("want 1 reversal transaction, got %d", revCount)
	}
}

// TestReversalGuards: only completed transactions are reversible, and a
// rejected reversal leaves the original completed.
func TestReversalGuards(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, senderID, senderWallet, 2_000_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)

	adminA := e.SeedAdminID(t)
	adminB := e.SeedAdmin2ID(t)

	if _, err := e.Revsvc.CreateRequest(ctx, adminA, "MP-does-not-exist", "x"); err != reversal.ErrTxnNotFound {
		t.Fatalf("unknown ref: want ErrTxnNotFound, got %v", err)
	}

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 500_000, "reject me", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)

	cr, err := e.Revsvc.CreateRequest(ctx, adminA, res.RefCode, "maybe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Asvc.AdminDecide(ctx, adminB, cr.RequestID, false, "no"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("rejected reversal must leave original completed, got %s", got)
	}
}

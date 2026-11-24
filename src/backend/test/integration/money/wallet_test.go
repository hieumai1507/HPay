package money

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestEdge_E01_IdempotentTopup: two top-ups with the same Idempotency-Key —
// exactly one transaction, funds credited exactly once.
func TestEdge_E01_IdempotentTopup(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	ctx := context.Background()
	userID, walletID := testutil.NewUserWithWallet(t, pool)

	svc := wallet.NewService(pool)
	key := uuid.NewString()

	first, err := svc.Topup(ctx, userID, 2_000, key)
	if err != nil {
		t.Fatalf("first topup: %v", err)
	}
	if first.Duplicate {
		t.Fatal("first call must not be duplicate")
	}
	second, err := svc.Topup(ctx, userID, 2_000, key)
	if err != nil {
		t.Fatalf("second topup: %v", err)
	}
	if !second.Duplicate || second.RefCode != first.RefCode {
		t.Fatalf("second call must return the SAME transaction: %+v vs %+v", second, first)
	}
	if got := testutil.WalletBalance(t, pool, walletID); got != 2_000 {
		t.Fatalf("balance credited more than once: want 2000, got %d", got)
	}
	if testutil.WalletBalance(t, pool, walletID) != testutil.LedgerSum(t, pool, walletID) {
		t.Fatal("balance != ledger sum")
	}
}

// TestEdge_E15_DuplicateBankCallback: the mock bank delivers the same topup
// callback twice — money credits exactly once, both deliveries book a
// bank_statements row (the second flagged duplicate) so recon sees the repeat.
func TestEdge_E15_DuplicateBankCallback(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	ctx := context.Background()
	userID, walletID := testutil.NewUserWithWallet(t, pool)
	svc := wallet.NewService(pool)

	pending, err := svc.CreatePendingTopup(ctx, userID, 3_000, uuid.NewString())
	if err != nil {
		t.Fatalf("create pending topup: %v", err)
	}
	if pending.Status != "created" {
		t.Fatalf("pending topup must park at created, got %s", pending.Status)
	}
	if got := testutil.WalletBalance(t, pool, walletID); got != 0 {
		t.Fatalf("no money before the callback: got %d", got)
	}

	first, err := svc.BankCallback(ctx, pending.RefCode, 3_000, "BANK-REF-1")
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	if first.Duplicate || first.Status != "completed" {
		t.Fatalf("first callback must complete the topup: %+v", first)
	}

	second, err := svc.BankCallback(ctx, pending.RefCode, 3_000, "BANK-REF-1-RETRY")
	if err != nil {
		t.Fatalf("second callback: %v", err)
	}
	if !second.Duplicate {
		t.Fatalf("second callback must be flagged duplicate: %+v", second)
	}

	if got := testutil.WalletBalance(t, pool, walletID); got != 3_000 {
		t.Fatalf("credited more than once: want 3000, got %d", got)
	}
	if testutil.WalletBalance(t, pool, walletID) != testutil.LedgerSum(t, pool, walletID) {
		t.Fatal("balance != ledger sum")
	}
	var entries int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ledger_entries le
		JOIN transactions tx ON tx.id = le.transaction_id
		WHERE tx.ref_code = $1`, pending.RefCode).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if entries != 2 { // exactly one debit+credit pair
		t.Fatalf("want exactly 2 ledger entries, got %d", entries)
	}
	var stmts, dups int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE (raw->>'duplicate')::bool)
		FROM bank_statements WHERE ref_code = $1`, pending.RefCode).Scan(&stmts, &dups); err != nil {
		t.Fatal(err)
	}
	if stmts != 2 || dups != 1 {
		t.Fatalf("want 2 statements with 1 duplicate for recon, got %d/%d", stmts, dups)
	}

	// Amount mismatch on a pending topup: no money moves, statement still booked.
	p2, err := svc.CreatePendingTopup(ctx, userID, 400, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BankCallback(ctx, p2.RefCode, 999_999, "BANK-BAD"); !testkit.ErrIs(err, wallet.ErrCallbackAmountMismatch) {
		t.Fatalf("want ErrCallbackAmountMismatch, got %v", err)
	}
	if got := testutil.WalletBalance(t, pool, walletID); got != 3_000 {
		t.Fatalf("mismatched callback must move no money: got %d", got)
	}
}

package ledger_test

// ADR-0010 / BE-C1: a ledger posting happens at most once per (transaction, kind).
// Cases map to docs/specs/w1-ledger-and-idempotency/test-cases.md (TC-LED-001..008).

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

type fixture struct {
	pool             *pgxpool.Pool
	user             uuid.UUID
	sender, receiver uuid.UUID
}

// newFixture creates two wallets and funds the sender with 1,000 from the system cash-in wallet.
func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := testutil.PoolFromEnv(t)
	user, sender := testutil.NewUserWithWallet(t, pool)
	_, receiver := testutil.NewUserWithWallet(t, pool)
	cash := testutil.SystemWallet(t, pool, "SYSTEM_CASH_IN")
	fundTxn := insertTxn(t, pool, user, cash, sender, 1_000)
	if err := post(t, pool, fundTxn, []ledger.Leg{
		{WalletID: cash, Direction: store.EntryDirectionDebit, Amount: 1_000},
		{WalletID: sender, Direction: store.EntryDirectionCredit, Amount: 1_000},
	}); err != nil {
		t.Fatalf("fund sender: %v", err)
	}
	return fixture{pool: pool, user: user, sender: sender, receiver: receiver}
}

func (f fixture) legs(amount int64) []ledger.Leg {
	return []ledger.Leg{
		{WalletID: f.sender, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: f.receiver, Direction: store.EntryDirectionCredit, Amount: amount},
	}
}

func (f fixture) reverse(amount int64) []ledger.Leg {
	return []ledger.Leg{
		{WalletID: f.receiver, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: f.sender, Direction: store.EntryDirectionCredit, Amount: amount},
	}
}

func postKind(t *testing.T, pool *pgxpool.Pool, txnID uuid.UUID, kind ledger.Kind, legs []ledger.Leg) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := ledger.Post(ctx, tx, txnID, kind, legs); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return nil
}

func entryCount(t *testing.T, pool *pgxpool.Pool, txnID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM ledger_entries WHERE transaction_id = $1`, txnID).Scan(&n); err != nil {
		t.Fatalf("count entries: %v", err)
	}
	return n
}

// TC-LED-001: the second post of the same (txn, kind) is rejected and changes nothing.
func TestPost_SecondCallSameKindIsRejected(t *testing.T) {
	f := newFixture(t)
	txn := insertTxn(t, f.pool, f.user, f.sender, f.receiver, 100)

	if err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(100)); err != nil {
		t.Fatalf("first post: %v", err)
	}
	entries := entryCount(t, f.pool, txn)
	senderBal := testutil.WalletBalance(t, f.pool, f.sender)
	receiverBal := testutil.WalletBalance(t, f.pool, f.receiver)

	err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(100))
	if !errors.Is(err, ledger.ErrAlreadyPosted) {
		t.Fatalf("second post: want ErrAlreadyPosted, got %v", err)
	}
	if got := entryCount(t, f.pool, txn); got != entries {
		t.Errorf("entries changed %d -> %d", entries, got)
	}
	if got := testutil.WalletBalance(t, f.pool, f.sender); got != senderBal {
		t.Errorf("sender balance changed %d -> %d", senderBal, got)
	}
	if got := testutil.WalletBalance(t, f.pool, f.receiver); got != receiverBal {
		t.Errorf("receiver balance changed %d -> %d", receiverBal, got)
	}
	// recon invariant still holds
	if d, l := testutil.WalletBalance(t, f.pool, f.sender), testutil.LedgerSum(t, f.pool, f.sender); d != l {
		t.Errorf("sender balance %d != ledger sum %d", d, l)
	}
}

// Same (txn, kind) with a DIFFERENT total is a conflict, never a replay.
func TestPost_SameKindDifferentAmountIsAConflict(t *testing.T) {
	f := newFixture(t)
	txn := insertTxn(t, f.pool, f.user, f.sender, f.receiver, 100)
	if err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(100)); err != nil {
		t.Fatalf("first post: %v", err)
	}
	before := testutil.WalletBalance(t, f.pool, f.sender)

	err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(40))
	if !errors.Is(err, ledger.ErrPostingConflict) {
		t.Fatalf("want ErrPostingConflict, got %v", err)
	}
	if got := testutil.WalletBalance(t, f.pool, f.sender); got != before {
		t.Errorf("sender balance changed %d -> %d", before, got)
	}
}

// TC-LED-002: two concurrent posts of the same (txn, kind): exactly one effect.
func TestPost_ConcurrentSameKindOnce(t *testing.T) {
	f := newFixture(t)
	txn := insertTxn(t, f.pool, f.user, f.sender, f.receiver, 100)

	const workers = 8
	errs := make([]error, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = postKind(t, f.pool, txn, ledger.KindForward, f.legs(100))
		}(i)
	}
	close(start)
	wg.Wait()

	ok, dup := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ledger.ErrAlreadyPosted):
			dup++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || dup != workers-1 {
		t.Fatalf("want 1 success and %d ErrAlreadyPosted, got %d and %d", workers-1, ok, dup)
	}
	if got := testutil.WalletBalance(t, f.pool, f.sender); got != 900 {
		t.Errorf("sender balance = %d, want 900 (moved once)", got)
	}
	if got := entryCount(t, f.pool, txn); got != 2 {
		t.Errorf("entries = %d, want 2", got)
	}
}

// TC-LED-003: compensation is independent of the forward posting and happens once.
func TestPost_CompensateIndependentAndOnce(t *testing.T) {
	f := newFixture(t)
	txn := insertTxn(t, f.pool, f.user, f.sender, f.receiver, 100)

	if err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(100)); err != nil {
		t.Fatalf("forward: %v", err)
	}
	if err := postKind(t, f.pool, txn, ledger.KindCompensate, f.reverse(100)); err != nil {
		t.Fatalf("compensate: %v", err)
	}
	if err := postKind(t, f.pool, txn, ledger.KindCompensate, f.reverse(100)); !errors.Is(err, ledger.ErrAlreadyPosted) {
		t.Fatalf("second compensate: want ErrAlreadyPosted, got %v", err)
	}
	if got := testutil.WalletBalance(t, f.pool, f.sender); got != 1_000 {
		t.Errorf("sender balance = %d, want 1000 (forward and compensation net to zero)", got)
	}
	if got := entryCount(t, f.pool, txn); got != 4 {
		t.Errorf("entries = %d, want 4 (2 forward + 2 compensate)", got)
	}
}

// TC-LED-004 (partial, at the ledger level): a round-up posts once on top of an existing forward posting.
func TestPost_RoundUpAfterForwardOnce(t *testing.T) {
	f := newFixture(t)
	txn := insertTxn(t, f.pool, f.user, f.sender, f.receiver, 100)
	if err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(100)); err != nil {
		t.Fatalf("forward: %v", err)
	}
	if err := postKind(t, f.pool, txn, ledger.KindRoundUp, f.legs(7)); err != nil {
		t.Fatalf("round-up: %v", err)
	}
	if err := postKind(t, f.pool, txn, ledger.KindRoundUp, f.legs(7)); !errors.Is(err, ledger.ErrAlreadyPosted) {
		t.Fatalf("second round-up: want ErrAlreadyPosted, got %v", err)
	}
	if got := testutil.WalletBalance(t, f.pool, f.sender); got != 893 {
		t.Errorf("sender balance = %d, want 893", got)
	}
}

// TC-LED-008: a posting that fails leaves no guard row behind, so a retry can post.
func TestPost_FailedPostingLeavesNoGuard(t *testing.T) {
	f := newFixture(t)
	txn := insertTxn(t, f.pool, f.user, f.sender, f.receiver, 5_000) // more than the sender holds

	err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(5_000))
	if !errors.Is(err, ledger.ErrInsufficientBalance) {
		t.Fatalf("want ErrInsufficientBalance, got %v", err)
	}
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM ledger_postings WHERE transaction_id = $1`, txn).Scan(&n); err != nil {
		t.Fatalf("count postings: %v", err)
	}
	if n != 0 {
		t.Fatalf("guard rows after a failed posting = %d, want 0", n)
	}
	// the same transaction can still be posted with a payload that works
	if err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(100)); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
}

// TC-LED-006 (data part): the back-filled guard rows exist for transactions posted before migration 000014.
func TestMigration_BackfillCoversEveryPostedTransaction(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	var missing int
	err := pool.QueryRow(context.Background(), `
		SELECT count(*) FROM (
		  SELECT DISTINCT transaction_id FROM ledger_entries
		  EXCEPT
		  SELECT transaction_id FROM ledger_postings
		) x`).Scan(&missing)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if missing != 0 {
		t.Fatalf("%d transactions have ledger entries but no ledger_postings row", missing)
	}
}

// TC-LED-007: the ledger stays append-only.
func TestLedgerEntriesStayAppendOnly(t *testing.T) {
	f := newFixture(t)
	txn := insertTxn(t, f.pool, f.user, f.sender, f.receiver, 100)
	if err := postKind(t, f.pool, txn, ledger.KindForward, f.legs(100)); err != nil {
		t.Fatalf("post: %v", err)
	}
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE ledger_entries SET amount = amount + 1 WHERE transaction_id = $1`, txn); err == nil {
		t.Error("UPDATE on ledger_entries succeeded; the append-only trigger is not holding")
	}
	if _, err := f.pool.Exec(ctx, `DELETE FROM ledger_entries WHERE transaction_id = $1`, txn); err == nil {
		t.Error("DELETE on ledger_entries succeeded; the append-only trigger is not holding")
	}
}

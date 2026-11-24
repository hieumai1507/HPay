package ledger_test

// Red test for BE-C1 against the CURRENT signature Post(ctx, tx, txnID, legs).
// It proves the defect before any fix: posting the same transaction twice
// moves the money twice. After ADR-0010 lands the signature gains a `kind`
// argument and this file is replaced by posting_guard_test.go.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

func insertTxn(t *testing.T, pool *pgxpool.Pool, initiator, from, to uuid.UUID, amount int64) uuid.UUID {
	t.Helper()
	row, err := store.New(pool).InsertTransaction(context.Background(), store.InsertTransactionParams{
		RefCode:          wallet.NewRefCode(),
		IdempotencyKey:   pgtype.Text{String: uuid.NewString(), Valid: true},
		Type:             store.TxnTypeP2p,
		SenderWalletID:   from,
		ReceiverWalletID: to,
		Amount:           amount,
		Fee:              0,
		Note:             pgtype.Text{},
		InitiatedBy:      initiator,
	})
	if err != nil {
		t.Fatalf("insert transaction: %v", err)
	}
	return row.ID
}

// post runs ledger.Post in its own transaction, like the MOVE_FUNDS handler does.
func post(t *testing.T, pool *pgxpool.Pool, txnID uuid.UUID, legs []ledger.Leg) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	if err := ledger.Post(ctx, tx, txnID, ledger.KindForward, legs); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return nil
}

func TestBEC1_SecondPostOfSameTransactionMovesMoneyTwice(t *testing.T) {
	pool := testutil.PoolFromEnv(t)

	senderUser, sender := testutil.NewUserWithWallet(t, pool)
	_, receiver := testutil.NewUserWithWallet(t, pool)

	// Fund the sender from the system cash-in wallet so balances are legitimate.
	cash := testutil.SystemWallet(t, pool, "SYSTEM_CASH_IN")
	fundTxn := insertTxn(t, pool, senderUser, cash, sender, 1_000)
	if err := post(t, pool, fundTxn, []ledger.Leg{
		{WalletID: cash, Direction: store.EntryDirectionDebit, Amount: 1_000},
		{WalletID: sender, Direction: store.EntryDirectionCredit, Amount: 1_000},
	}); err != nil {
		t.Fatalf("fund sender: %v", err)
	}

	txnID := insertTxn(t, pool, senderUser, sender, receiver, 100)
	legs := []ledger.Leg{
		{WalletID: sender, Direction: store.EntryDirectionDebit, Amount: 100},
		{WalletID: receiver, Direction: store.EntryDirectionCredit, Amount: 100},
	}
	if err := post(t, pool, txnID, legs); err != nil {
		t.Fatalf("first post: %v", err)
	}
	afterFirst := testutil.WalletBalance(t, pool, sender)
	if afterFirst != 900 {
		t.Fatalf("setup wrong: sender balance after first post = %d, want 900", afterFirst)
	}

	// A redelivered MOVE_FUNDS step runs the same posting again.
	_ = post(t, pool, txnID, legs) // today this succeeds; after ADR-0010 it must be rejected

	afterSecond := testutil.WalletBalance(t, pool, sender)
	if afterSecond != afterFirst {
		t.Fatalf("BE-C1 reproduced: sender balance went %d -> %d after a second post of the same transaction (expected unchanged)",
			afterFirst, afterSecond)
	}
}

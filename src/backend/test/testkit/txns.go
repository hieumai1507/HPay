package testkit

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func (e *Env) CountTxnsByKey(t *testing.T, userID uuid.UUID, key string) int {
	t.Helper()
	var n int
	if err := e.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM transactions WHERE initiated_by=$1 AND idempotency_key=$2`, userID, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *Env) CountOrgTxnsByKey(t *testing.T, key string) int {
	t.Helper()
	var n int
	if err := e.Pool.QueryRow(context.Background(), `SELECT count(*) FROM transactions WHERE idempotency_key=$1`, key).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// completedTransfer funds a sender, sends amount to a new receiver and waits for completion.
func (e *Env) CompletedTransfer(t *testing.T, amount int64) (refCode string, txnID uuid.UUID, senderWallet, receiverWallet uuid.UUID, receiverUser uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	senderID, sw, _ := e.UserWithMoney(t, 0)
	Fund(t, e.Pool, senderID, sw, amount*2)
	rUser, rw, rPhone := e.UserWithMoney(t, 0)
	res, err := e.Svc.Transfer(ctx, senderID, rPhone, amount, "setup", uuid.NewString())
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	e.Drain(t)
	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("setup transfer: want completed, got %s", got)
	}
	if err := e.Pool.QueryRow(ctx, `SELECT id FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	return res.RefCode, txnID, sw, rw, rUser
}

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

// TestEdge_E06_DuplicateStepNoDoublePost (TC-LED-004, BE-C1, ADR-0010).
//
// The worker commits MOVE_FUNDS' money movement in one transaction and records
// the step in a second one. If the process dies between the two, or the outbox
// redelivers the event, the handler runs again for the same transaction. This
// test simulates that by completing a transfer and then invoking the registered
// MOVE_FUNDS handler a second time with the same input: balances must not move
// again and the replayed step must report success.
func TestEdge_E06_DuplicateStepNoDoublePost(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.userWithMoney(t, 2_000_000)
	_, receiverWallet, receiverPhone := e.userWithMoney(t, 0)

	res, err := e.svc.Transfer(ctx, senderID, receiverPhone, 1_000_000, "replay", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)

	var txnID uuid.UUID
	if err := e.pool.QueryRow(ctx, `SELECT id FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	senderBefore := testutil.WalletBalance(t, e.pool, senderWallet)
	receiverBefore := testutil.WalletBalance(t, e.pool, receiverWallet)
	feeWallet := testutil.SystemWallet(t, e.pool, "SYSTEM_FEE")
	feeBefore := testutil.WalletBalance(t, e.pool, feeWallet)

	var instanceID uuid.UUID
	if err := e.pool.QueryRow(ctx, `SELECT id FROM workflow_instances WHERE transaction_id=$1`, txnID).Scan(&instanceID); err != nil {
		t.Fatal(err)
	}
	h, ok := e.reg.Get("wallet")
	if !ok {
		t.Fatal("wallet handler (MOVE_FUNDS) is not registered")
	}

	// Redelivery: same instance, same step, a new attempt number.
	out, err := h.Execute(ctx, workflow.StepInput{
		InstanceID:    instanceID,
		TransactionID: txnID,
		Step:          workflow.StepDef{Name: "MOVE_FUNDS", Handler: "wallet"},
		Attempt:       2,
	})
	if err != nil {
		t.Fatalf("replayed MOVE_FUNDS must succeed (idempotent), got error: %v", err)
	}
	if out.Context["funds_moved"] != true {
		t.Errorf("replayed step output funds_moved = %v, want true", out.Context["funds_moved"])
	}

	if got := testutil.WalletBalance(t, e.pool, senderWallet); got != senderBefore {
		t.Errorf("sender balance moved on replay: %d -> %d", senderBefore, got)
	}
	if got := testutil.WalletBalance(t, e.pool, receiverWallet); got != receiverBefore {
		t.Errorf("receiver balance moved on replay: %d -> %d", receiverBefore, got)
	}
	if got := testutil.WalletBalance(t, e.pool, feeWallet); got != feeBefore {
		t.Errorf("fee wallet moved on replay: %d -> %d", feeBefore, got)
	}
	// recon invariant: stored balance == ledger sum for every wallet touched
	for name, id := range map[string]uuid.UUID{"sender": senderWallet, "receiver": receiverWallet} {
		if d, l := testutil.WalletBalance(t, e.pool, id), testutil.LedgerSum(t, e.pool, id); d != l {
			t.Errorf("%s: balance %d != ledger sum %d", name, d, l)
		}
	}
}

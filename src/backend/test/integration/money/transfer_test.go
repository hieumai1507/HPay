package money

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/fee"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func TestP2PTransferEndToEnd(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.UserWithMoney(t, 8_000)
	_, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 4_000, "test p2p", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if res.Fee != 25 { // the p2p fee is a fixed 25 cents
		t.Fatalf("fee: want 25, got %d", res.Fee)
	}
	e.Drain(t)

	var status string
	if err := e.Pool.QueryRow(ctx, `SELECT status FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("txn status: want completed, got %s", status)
	}
	if got := testutil.WalletBalance(t, e.Pool, senderWallet); got != 8_000-4_000-25 {
		t.Fatalf("sender balance: want 3975, got %d", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, receiverWallet); got != 4_000 {
		t.Fatalf("receiver balance: want 4000, got %d", got)
	}
	feeWallet := testutil.SystemWallet(t, e.Pool, "SYSTEM_FEE")
	if net := e.TxnLedgerNet(t, res.RefCode, feeWallet); net != 25 {
		t.Fatalf("SYSTEM_FEE net for txn: want 25, got %d", net)
	}

	// timeline has all 7 steps completed: v3 (docs plan action-pipeline-wiring)
	// added ACTIONS_BEFORE_GL/ACTIONS_AFTER_GL around MOVE_FUNDS, and v2 (docs
	// plan round-up-vault) appended ROUND_UP after CONFIRM. This transfer has
	// no round-up target configured, so ROUND_UP is a no-op; e.Svc.Transfer's
	// test definition has no service_code in context, so both ACTIONS_* steps
	// also no-op (no service to resolve actions from).
	var txnID uuid.UUID
	if err := e.Pool.QueryRow(ctx, `SELECT id FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	tl, err := e.Svc.TimelineFor(ctx, txnID)
	if err != nil {
		t.Fatal(err)
	}
	if tl.InstanceStatus != "completed" || len(tl.Steps) != 7 {
		t.Fatalf("timeline: status=%s steps=%d, want completed/7", tl.InstanceStatus, len(tl.Steps))
	}
}

// TestEdge_E07_CompensationReversesMovedFunds: a step fails terminally AFTER the money
// has moved → the engine compensates: balances restored, ledger net 0, instance compensated.
func TestEdge_E07_CompensationReversesMovedFunds(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	defName := "p2p_comp_" + uuid.NewString()[:8]
	_, err := e.Q.InsertWorkflowDefinition(ctx, store.InsertWorkflowDefinitionParams{
		Name: defName, Version: 1, IsActive: true,
		Steps: []byte(`[
		 {"name":"VALIDATE","type":"auto","handler":"validator","retry":2,"timeout_ms":5000,"on_failure":"fail"},
		 {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":2,"timeout_ms":10000,"on_failure":"fail","config":{"compensatable":true}},
		 {"name":"EXPLODE","type":"auto","handler":"boom","retry":1,"timeout_ms":2000,"on_failure":"compensate"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.Svc = transfer.NewServiceWithDefinition(e.Pool, e.Engine, fee.NewService(e.Pool), defName)

	senderID, senderWallet, _ := e.UserWithMoney(t, 8_000)
	_, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 2_000, "comp test", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)

	var instStatus, txnStatus string
	if err := e.Pool.QueryRow(ctx, `
		SELECT wi.status, t.status FROM workflow_instances wi
		JOIN transactions t ON t.id = wi.transaction_id
		WHERE t.ref_code = $1`, res.RefCode).Scan(&instStatus, &txnStatus); err != nil {
		t.Fatal(err)
	}
	if instStatus != "compensated" || txnStatus != "failed" {
		t.Fatalf("want compensated/failed, got %s/%s", instStatus, txnStatus)
	}
	if got := testutil.WalletBalance(t, e.Pool, senderWallet); got != 8_000 {
		t.Fatalf("sender must be made whole: want 8000, got %d", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, receiverWallet); got != 0 {
		t.Fatalf("receiver must be reversed: want 0, got %d", got)
	}
	for _, w := range []uuid.UUID{senderWallet, receiverWallet} {
		if net := e.TxnLedgerNet(t, res.RefCode, w); net != 0 {
			t.Fatalf("ledger net for wallet must be 0 after compensation, got %d", net)
		}
	}
	var entries int
	if err := e.Pool.QueryRow(ctx, `
		SELECT count(*) FROM ledger_entries le JOIN transactions t ON t.id=le.transaction_id
		WHERE t.ref_code=$1`, res.RefCode).Scan(&entries); err != nil {
		t.Fatal(err)
	}
	if entries != 6 { // 3 forward + 3 reverse (fee > 0 at tier none)
		t.Fatalf("want 6 ledger entries (forward+reverse), got %d", entries)
	}
}

// TestEdge_E12_LockedUserFailsMidWorkflow: user is locked after the request was created,
// before the money moves → the workflow fails in a controlled way, NO ledger entries are written.
func TestEdge_E12_LockedUserFailsMidWorkflow(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.UserWithMoney(t, 8_000)
	receiverID, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 1_200, "lock test", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	// lock the receiver BEFORE the worker runs any step
	if _, err := e.Pool.Exec(ctx, `UPDATE users SET status='locked' WHERE id=$1`, receiverID); err != nil {
		t.Fatal(err)
	}
	e.Drain(t)

	var txnStatus string
	if err := e.Pool.QueryRow(ctx, `SELECT status FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnStatus); err != nil {
		t.Fatal(err)
	}
	if txnStatus != "failed" {
		t.Fatalf("want failed, got %s", txnStatus)
	}
	if got := testutil.WalletBalance(t, e.Pool, senderWallet); got != 8_000 {
		t.Fatalf("sender untouched: want 8000, got %d", got)
	}
	if net := e.TxnLedgerNet(t, res.RefCode, senderWallet); net != 0 {
		t.Fatal("no ledger entries expected for sender")
	}
	if net := e.TxnLedgerNet(t, res.RefCode, receiverWallet); net != 0 {
		t.Fatal("no ledger entries expected for receiver")
	}
}

// AC-6: the P2P fee is a flat 25 cents, whatever the amount.
func TestP2PFixedFeeIs25Cents(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, _, _ := e.UserWithMoney(t, 5_000)
	for _, amount := range []int64{1_000, 2_000} {
		receiverID, _, _ := e.UserWithMoney(t, 0)
		res, err := e.Svc.TransferToUser(ctx, senderID, receiverID, amount, "fee", uuid.NewString())
		if err != nil {
			t.Fatalf("transfer %d: %v", amount, err)
		}
		if res.Fee != 25 {
			t.Errorf("amount %d: fee = %d, want 25", amount, res.Fee)
		}
	}
}

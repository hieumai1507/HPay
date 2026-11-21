package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/fee"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
)

func TestP2PTransferEndToEnd(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.userWithMoney(t, 2_000_000)
	_, receiverWallet, receiverPhone := e.userWithMoney(t, 0)

	res, err := e.svc.Transfer(ctx, senderID, receiverPhone, 1_000_000, "test p2p", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if res.Fee != 1_000 { // tier none: 0.1% of 1M
		t.Fatalf("fee: want 1000, got %d", res.Fee)
	}
	e.drain(t)

	var status string
	if err := e.pool.QueryRow(ctx, `SELECT status FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "completed" {
		t.Fatalf("txn status: want completed, got %s", status)
	}
	if got := testutil.WalletBalance(t, e.pool, senderWallet); got != 2_000_000-1_001_000 {
		t.Fatalf("sender balance: want 999000, got %d", got)
	}
	if got := testutil.WalletBalance(t, e.pool, receiverWallet); got != 1_000_000 {
		t.Fatalf("receiver balance: want 1000000, got %d", got)
	}
	feeWallet := testutil.SystemWallet(t, e.pool, "SYSTEM_FEE")
	if net := e.txnLedgerNet(t, res.RefCode, feeWallet); net != 1_000 {
		t.Fatalf("SYSTEM_FEE net for txn: want 1000, got %d", net)
	}

	// timeline has all 7 steps completed: v3 (docs plan action-pipeline-wiring)
	// added ACTIONS_BEFORE_GL/ACTIONS_AFTER_GL around MOVE_FUNDS, and v2 (docs
	// plan round-up-vault) appended ROUND_UP after CONFIRM. This transfer has
	// no round-up target configured, so ROUND_UP is a no-op; e.svc.Transfer's
	// test definition has no service_code in context, so both ACTIONS_* steps
	// also no-op (no service to resolve actions from).
	var txnID uuid.UUID
	if err := e.pool.QueryRow(ctx, `SELECT id FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	tl, err := e.svc.TimelineFor(ctx, txnID)
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
	e := newEnv(t, "")
	ctx := context.Background()
	defName := "p2p_comp_" + uuid.NewString()[:8]
	_, err := e.q.InsertWorkflowDefinition(ctx, store.InsertWorkflowDefinitionParams{
		Name: defName, Version: 1, IsActive: true,
		Steps: []byte(`[
		 {"name":"VALIDATE","type":"auto","handler":"validator","retry":2,"timeout_ms":5000,"on_failure":"fail"},
		 {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":2,"timeout_ms":10000,"on_failure":"fail","config":{"compensatable":true}},
		 {"name":"EXPLODE","type":"auto","handler":"boom","retry":1,"timeout_ms":2000,"on_failure":"compensate"}]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = transfer.NewServiceWithDefinition(e.pool, e.engine, fee.NewService(e.pool), defName)

	senderID, senderWallet, _ := e.userWithMoney(t, 2_000_000)
	_, receiverWallet, receiverPhone := e.userWithMoney(t, 0)

	res, err := e.svc.Transfer(ctx, senderID, receiverPhone, 500_000, "comp test", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)

	var instStatus, txnStatus string
	if err := e.pool.QueryRow(ctx, `
		SELECT wi.status, t.status FROM workflow_instances wi
		JOIN transactions t ON t.id = wi.transaction_id
		WHERE t.ref_code = $1`, res.RefCode).Scan(&instStatus, &txnStatus); err != nil {
		t.Fatal(err)
	}
	if instStatus != "compensated" || txnStatus != "failed" {
		t.Fatalf("want compensated/failed, got %s/%s", instStatus, txnStatus)
	}
	if got := testutil.WalletBalance(t, e.pool, senderWallet); got != 2_000_000 {
		t.Fatalf("sender must be made whole: want 2000000, got %d", got)
	}
	if got := testutil.WalletBalance(t, e.pool, receiverWallet); got != 0 {
		t.Fatalf("receiver must be reversed: want 0, got %d", got)
	}
	for _, w := range []uuid.UUID{senderWallet, receiverWallet} {
		if net := e.txnLedgerNet(t, res.RefCode, w); net != 0 {
			t.Fatalf("ledger net for wallet must be 0 after compensation, got %d", net)
		}
	}
	var entries int
	if err := e.pool.QueryRow(ctx, `
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
	e := newEnv(t, "")
	ctx := context.Background()
	senderID, senderWallet, _ := e.userWithMoney(t, 2_000_000)
	receiverID, receiverWallet, receiverPhone := e.userWithMoney(t, 0)

	res, err := e.svc.Transfer(ctx, senderID, receiverPhone, 300_000, "lock test", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	// lock the receiver BEFORE the worker runs any step
	if _, err := e.pool.Exec(ctx, `UPDATE users SET status='locked' WHERE id=$1`, receiverID); err != nil {
		t.Fatal(err)
	}
	e.drain(t)

	var txnStatus string
	if err := e.pool.QueryRow(ctx, `SELECT status FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnStatus); err != nil {
		t.Fatal(err)
	}
	if txnStatus != "failed" {
		t.Fatalf("want failed, got %s", txnStatus)
	}
	if got := testutil.WalletBalance(t, e.pool, senderWallet); got != 2_000_000 {
		t.Fatalf("sender untouched: want 2000000, got %d", got)
	}
	if net := e.txnLedgerNet(t, res.RefCode, senderWallet); net != 0 {
		t.Fatal("no ledger entries expected for sender")
	}
	if net := e.txnLedgerNet(t, res.RefCode, receiverWallet); net != 0 {
		t.Fatal("no ledger entries expected for receiver")
	}
}

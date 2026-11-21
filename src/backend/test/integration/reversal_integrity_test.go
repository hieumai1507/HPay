package integration

// Wave 2 slice: reversal integrity (BE-H3, BE-H4, ADR-0013, ADR-0009).
// Cases map to docs/specs/w2-reversal-integrity/test-cases.md (TC-REV-001..008).

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/reversal"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

// completedTransfer funds a sender, sends amount to a new receiver and waits for completion.
func (e *env) completedTransfer(t *testing.T, amount int64) (refCode string, txnID uuid.UUID, senderWallet, receiverWallet uuid.UUID, receiverUser uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	senderID, sw, _ := e.userWithMoney(t, 0)
	fund(t, e.pool, senderID, sw, amount*2)
	rUser, rw, rPhone := e.userWithMoney(t, 0)
	res, err := e.svc.Transfer(ctx, senderID, rPhone, amount, "setup", uuid.NewString())
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("setup transfer: want completed, got %s", got)
	}
	if err := e.pool.QueryRow(ctx, `SELECT id FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	return res.RefCode, txnID, sw, rw, rUser
}

// TC-REV-001: concurrent requests for one transaction create exactly one request.
func TestReversal_ConcurrentRequestsOnlyOne(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	ref, _, _, _, _ := e.completedTransfer(t, 500_000)
	admin := e.seedAdminID(t)

	const workers = 8
	errs := make([]error, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = e.revsvc.CreateRequest(ctx, admin, ref, "race")
		}(i)
	}
	close(start)
	wg.Wait()

	ok, pending := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, reversal.ErrReversalPending):
			pending++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 || pending != workers-1 {
		t.Fatalf("want 1 created and %d ErrReversalPending, got %d and %d", workers-1, ok, pending)
	}
}

// TC-REV-002: the database refuses a second reversal row for one original.
func TestReversal_SecondReversalRowRejectedByDB(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	ref, txnID, sw, rw, _ := e.completedTransfer(t, 300_000)
	a, b := e.seedAdminID(t), e.seedAdmin2ID(t)
	cr, err := e.revsvc.CreateRequest(ctx, a, ref, "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.asvc.AdminDecide(ctx, b, cr.RequestID, true, "ok"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// An attempt to insert another reversal of the same original must violate the unique index.
	_, err = e.pool.Exec(ctx, `
		INSERT INTO transactions (ref_code, idempotency_key, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by, reversal_of, completed_at)
		SELECT 'RV-'||substr(md5(random()::text),1,10), 'rev2-'||gen_random_uuid()::text, 'reversal', 'completed', $2, $3, amount, fee, initiated_by, $1, now()
		FROM transactions WHERE id = $1`, txnID, rw, sw)
	if err == nil {
		t.Fatal("a second reversal row for the same original was accepted; want a unique violation")
	}
}

// TC-REV-003: a reversal transaction cannot itself be reversed.
func TestReversal_ReversalOfReversalRejected(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	ref, txnID, _, _, _ := e.completedTransfer(t, 200_000)
	a, b := e.seedAdminID(t), e.seedAdmin2ID(t)
	cr, err := e.revsvc.CreateRequest(ctx, a, ref, "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.asvc.AdminDecide(ctx, b, cr.RequestID, true, "ok"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var revRef string
	if err := e.pool.QueryRow(ctx, `SELECT ref_code FROM transactions WHERE reversal_of = $1`, txnID).Scan(&revRef); err != nil {
		t.Fatalf("reversal row missing: %v", err)
	}
	if _, err := e.revsvc.CreateRequest(ctx, a, revRef, "reverse the reversal"); !errors.Is(err, reversal.ErrNotReversible) {
		t.Fatalf("reversal of a reversal: want ErrNotReversible, got %v", err)
	}
}

// TC-REV-004: the executor requires the original to still be completed.
//
// Two layers enforce this today: the explicit status check right after the row
// lock, and the row count of MarkTransactionReversed (guarded completed -> reversed).
// Removing only the first layer leaves this test green because the second layer
// rolls everything back with the same error, so the test pins the BEHAVIOR (no
// money moves, no reversal row), not which layer caught it.
func TestReversal_ExecutorRequiresCompletedOriginal(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	ref, txnID, sw, rw, _ := e.completedTransfer(t, 250_000)
	a, b := e.seedAdminID(t), e.seedAdmin2ID(t)
	cr, err := e.revsvc.CreateRequest(ctx, a, ref, "stale")
	if err != nil {
		t.Fatal(err)
	}
	senderBefore := testutil.WalletBalance(t, e.pool, sw)
	receiverBefore := testutil.WalletBalance(t, e.pool, rw)
	// Something else moved the original out of 'completed' between request and approval.
	if _, err := e.pool.Exec(ctx, `UPDATE transactions SET status = 'failed' WHERE id = $1`, txnID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.asvc.AdminDecide(ctx, b, cr.RequestID, true, "approve stale"); err == nil {
		t.Fatal("approving a reversal of a no-longer-completed transaction must fail")
	}
	if got := testutil.WalletBalance(t, e.pool, sw); got != senderBefore {
		t.Errorf("sender balance changed %d -> %d", senderBefore, got)
	}
	if got := testutil.WalletBalance(t, e.pool, rw); got != receiverBefore {
		t.Errorf("receiver balance changed %d -> %d", receiverBefore, got)
	}
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE reversal_of = $1`, txnID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("a reversal row was created for a non-completed original: %d", n)
	}
}

// TC-REV-005: AdminDecide honours the request deadline.
func TestAdminDecide_ExpiredRequestRejected(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	ref, txnID, sw, _, _ := e.completedTransfer(t, 150_000)
	a, b := e.seedAdminID(t), e.seedAdmin2ID(t)
	cr, err := e.revsvc.CreateRequest(ctx, a, ref, "late")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE approval_requests SET expires_at = now() - interval '1 minute' WHERE id = $1`, cr.RequestID); err != nil {
		t.Fatal(err)
	}
	before := testutil.WalletBalance(t, e.pool, sw)
	_, err = e.asvc.AdminDecide(ctx, b, cr.RequestID, true, "too late")
	if !errors.Is(err, approval.ErrExpired) {
		t.Fatalf("want ErrExpired, got %v", err)
	}
	if got := testutil.WalletBalance(t, e.pool, sw); got != before {
		t.Errorf("sender balance changed on an expired decision: %d -> %d", before, got)
	}
	var n int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE reversal_of = $1`, txnID).Scan(&n)
	if n != 0 {
		t.Errorf("reversal executed for an expired request")
	}
}

// TC-REV-006 and TC-REV-008: the receiver spent the money, so the reversal cannot complete.
func TestReversal_ReceiverWithoutFundsGoesToNeedsManual(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	ref, txnID, sw, rw, rUser := e.completedTransfer(t, 400_000)
	a, b := e.seedAdminID(t), e.seedAdmin2ID(t)
	cr, err := e.revsvc.CreateRequest(ctx, a, ref, "receiver spent it")
	if err != nil {
		t.Fatal(err)
	}
	// The receiver moves everything away to another wallet.
	_, spendWallet, spendPhone := e.userWithMoney(t, 0)
	_ = spendWallet
	if _, err := e.svc.Transfer(ctx, rUser, spendPhone, testutil.WalletBalance(t, e.pool, rw)-0, "spend", uuid.NewString()); err != nil {
		// fee may make the full balance too large; retry with a smaller amount that still empties the wallet below the reversal amount
		bal := testutil.WalletBalance(t, e.pool, rw)
		if _, err2 := e.svc.Transfer(ctx, rUser, spendPhone, bal-bal/100-1, "spend", uuid.NewString()); err2 != nil {
			t.Fatalf("receiver spend: %v / %v", err, err2)
		}
	}
	e.drain(t)
	if testutil.WalletBalance(t, e.pool, rw) >= 400_000 {
		t.Fatal("setup: receiver still holds enough to be reversed")
	}

	senderBefore := testutil.WalletBalance(t, e.pool, sw)
	receiverBefore := testutil.WalletBalance(t, e.pool, rw)
	var ledgerBefore int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM ledger_entries`).Scan(&ledgerBefore)

	res, err := e.asvc.AdminDecide(ctx, b, cr.RequestID, true, "approve")
	if err != nil {
		t.Fatalf("the approver's decision must be recorded, not lost: %v", err)
	}
	if res.Status != "needs_manual" {
		t.Fatalf("decision status = %q, want needs_manual", res.Status)
	}

	var status string
	if err := e.pool.QueryRow(ctx, `SELECT status::text FROM approval_requests WHERE id = $1`, cr.RequestID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "needs_manual" {
		t.Errorf("approval_requests.status = %q, want needs_manual", status)
	}
	var actions int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM approval_actions WHERE request_id = $1`, cr.RequestID).Scan(&actions)
	if actions != 1 {
		t.Errorf("approver's action rows = %d, want 1 (the decision must survive)", actions)
	}
	var audits int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'reversal.needs_manual' AND resource LIKE '%'||$1||'%'`, ref).Scan(&audits)
	if audits < 1 {
		t.Errorf("no audit row reversal.needs_manual for %s", ref)
	}
	var revRows int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM transactions WHERE reversal_of = $1`, txnID).Scan(&revRows)
	if revRows != 0 {
		t.Errorf("a reversal transaction was created: %d", revRows)
	}
	if got := e.txnStatus(t, ref); got != "completed" {
		t.Errorf("original status = %s, want completed", got)
	}
	var ledgerAfter int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM ledger_entries`).Scan(&ledgerAfter)
	if ledgerAfter < ledgerBefore { // other tests may add entries; never fewer
		t.Errorf("ledger entries decreased")
	}
	if got := testutil.WalletBalance(t, e.pool, sw); got != senderBefore {
		t.Errorf("sender balance changed %d -> %d", senderBefore, got)
	}
	if got := testutil.WalletBalance(t, e.pool, rw); got != receiverBefore {
		t.Errorf("receiver balance changed %d -> %d", receiverBefore, got)
	}
	for name, id := range map[string]uuid.UUID{"sender": sw, "receiver": rw} {
		if d, l := testutil.WalletBalance(t, e.pool, id), testutil.LedgerSum(t, e.pool, id); d != l {
			t.Errorf("%s: balance %d != ledger sum %d", name, d, l)
		}
	}

	// TC-REV-007: a needs_manual request cannot be decided again.
	if _, err := e.asvc.AdminDecide(ctx, e.seedAdminID(t), cr.RequestID, true, "again"); !errors.Is(err, approval.ErrAlreadyFinalized) && !errors.Is(err, approval.ErrSelfApproval) {
		t.Errorf("second decision on a needs_manual request: want ErrAlreadyFinalized, got %v", err)
	}
}

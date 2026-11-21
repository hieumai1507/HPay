package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

// TestApprovalCancel_MakerCancelsPending: maker hủy lệnh pending của mình →
// txn failed, không bút toán — cùng hành vi với nhánh reject hiện có.
func TestApprovalCancel_MakerCancelsPending(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	if err := e.asvc.Cancel(ctx, f.makerID, reqID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := e.approvalStatus(t, reqID); got != "cancelled" {
		t.Fatalf("approval status: want cancelled, got %s", got)
	}
	e.drain(t)

	if got := e.txnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("txn: want failed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.pool, f.orgWallet); got != 2_000_000 {
		t.Fatalf("org wallet untouched: want 2000000, got %d", got)
	}
	if net := e.txnLedgerNet(t, res.RefCode, f.orgWallet); net != 0 {
		t.Fatal("no ledger entries expected on cancel")
	}
}

// TestApprovalCancel_CheckerCannotCancel: checker of the org is not the
// maker — cancelling someone else's request must be refused (404, not 403).
func TestApprovalCancel_CheckerCannotCancel(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	if err := e.asvc.Cancel(ctx, f.checkers[0], reqID); !errIs(err, approval.ErrNotFound) {
		t.Fatalf("checker cancel: want ErrNotFound, got %v", err)
	}
	if got := e.approvalStatus(t, reqID); got != "pending" {
		t.Fatalf("approval status must be untouched: want pending, got %s", got)
	}
}

// TestApprovalCancel_OutsiderGetsNotFound: a user with no relation to the
// org/request must not learn it exists.
func TestApprovalCancel_OutsiderGetsNotFound(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	outsider, _, _ := e.userWithMoney(t, 0)
	if err := e.asvc.Cancel(ctx, outsider, reqID); !errIs(err, approval.ErrNotFound) {
		t.Fatalf("outsider cancel: want ErrNotFound, got %v", err)
	}
}

// TestApprovalCancel_AlreadyFinalizedRejected: cancel is refused once the
// request has already left the pending state (approved/rejected/expired).
func TestApprovalCancel_AlreadyFinalizedRejected(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	if _, err := e.decide(t, f.checkers[0], reqID, true); err != nil {
		t.Fatalf("decide: %v", err)
	}
	e.drain(t)

	if err := e.asvc.Cancel(ctx, f.makerID, reqID); !errIs(err, approval.ErrAlreadyFinalized) {
		t.Fatalf("cancel after approved: want ErrAlreadyFinalized, got %v", err)
	}
}

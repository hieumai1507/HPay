package org

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestApprovalCancel_MakerCancelsPending: maker cancels their own pending request →
// txn failed, no ledger entries — same behavior as the existing reject branch.
func TestApprovalCancel_MakerCancelsPending(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	if err := e.Asvc.Cancel(ctx, f.MakerID, reqID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got := e.ApprovalStatus(t, reqID); got != "cancelled" {
		t.Fatalf("approval status: want cancelled, got %s", got)
	}
	e.Drain(t)

	if got := e.TxnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("txn: want failed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, f.OrgWallet); got != 2_000_000 {
		t.Fatalf("org wallet untouched: want 2000000, got %d", got)
	}
	if net := e.TxnLedgerNet(t, res.RefCode, f.OrgWallet); net != 0 {
		t.Fatal("no ledger entries expected on cancel")
	}
}

// TestApprovalCancel_CheckerCannotCancel: checker of the org is not the
// maker — cancelling someone else's request must be refused (404, not 403).
func TestApprovalCancel_CheckerCannotCancel(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	if err := e.Asvc.Cancel(ctx, f.Checkers[0], reqID); !testkit.ErrIs(err, approval.ErrNotFound) {
		t.Fatalf("checker cancel: want ErrNotFound, got %v", err)
	}
	if got := e.ApprovalStatus(t, reqID); got != "pending" {
		t.Fatalf("approval status must be untouched: want pending, got %s", got)
	}
}

// TestApprovalCancel_OutsiderGetsNotFound: a user with no relation to the
// org/request must not learn it exists.
func TestApprovalCancel_OutsiderGetsNotFound(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	outsider, _, _ := e.UserWithMoney(t, 0)
	if err := e.Asvc.Cancel(ctx, outsider, reqID); !testkit.ErrIs(err, approval.ErrNotFound) {
		t.Fatalf("outsider cancel: want ErrNotFound, got %v", err)
	}
}

// TestApprovalCancel_AlreadyFinalizedRejected: cancel is refused once the
// request has already left the pending state (approved/rejected/expired).
func TestApprovalCancel_AlreadyFinalizedRejected(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	if _, err := e.Decide(t, f.Checkers[0], reqID, true); err != nil {
		t.Fatalf("decide: %v", err)
	}
	e.Drain(t)

	if err := e.Asvc.Cancel(ctx, f.MakerID, reqID); !testkit.ErrIs(err, approval.ErrAlreadyFinalized) {
		t.Fatalf("cancel after approved: want ErrAlreadyFinalized, got %v", err)
	}
}

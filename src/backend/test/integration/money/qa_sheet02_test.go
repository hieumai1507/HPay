package money

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/org"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TC-BIZ-008: amount 0 and negative are both rejected at validation, no
// disbursement created.
func TestQA_DisbursementZeroOrNegativeAmountRejected(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	if _, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 0, "", uuid.NewString()); !testkit.ErrIs(err, wallet.ErrAmountInvalid) {
		t.Fatalf("amount=0: want ErrAmountInvalid, got %v", err)
	}
	if _, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, -100_000, "", uuid.NewString()); !testkit.ErrIs(err, wallet.ErrAmountInvalid) {
		t.Fatalf("amount=-100000: want ErrAmountInvalid, got %v", err)
	}
}

// TC-BIZ-009: a receiver phone that doesn't belong to any user is rejected
// clearly, no disbursement left hanging for an unresolvable recipient.
func TestQA_DisbursementUnknownRecipientRejected(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	_, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, "0999999999", 100_000, "", uuid.NewString())
	if !testkit.ErrIs(err, org.ErrReceiverNotFound) {
		t.Fatalf("unknown recipient: want ErrReceiverNotFound, got %v", err)
	}
}

// TC-BIZ-021: a wrong OTP code does not finalize the request — it stays
// pending exactly as it was before the attempt.
func TestQA_DecideWrongOTPDoesNotFinalize(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 500_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	// A real challenge must exist first (otherwise Decide would reject for a
	// different reason — no active challenge — rather than "wrong code").
	if _, err := e.Asvc.Challenge(ctx, f.Checkers[0], reqID); err != nil {
		t.Fatalf("challenge: %v", err)
	}

	if _, err := e.Asvc.Decide(ctx, f.Checkers[0], reqID, true, "000000-wrong"); err == nil {
		t.Fatal("wrong OTP: want an error, got nil")
	}

	req, err := e.Q.GetApprovalRequest(ctx, reqID)
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != "pending" {
		t.Fatalf("request must stay pending after a wrong-OTP attempt, got %s", req.Status)
	}
}

// TC-BIZ-026: checker A decides first and wins; checker B deciding the same
// (already-finalized) request seconds later gets a clear "already handled"
// error, not a generic failure — same guard as the true-race case (E08),
// exercised here as a plain sequential call instead of a race.
func TestQA_SecondCheckerAfterFinalizeGetsClearError(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1, 1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 500_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	if _, err := e.Decide(t, f.Checkers[0], reqID, true); err != nil {
		t.Fatalf("checker A decide: %v", err)
	}
	e.Drain(t)

	_, err = e.Decide(t, f.Checkers[1], reqID, true)
	if !testkit.ErrIs(err, approval.ErrAlreadyFinalized) {
		t.Fatalf("checker B after finalize: want ErrAlreadyFinalized, got %v", err)
	}
}

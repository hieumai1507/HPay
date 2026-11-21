package integration

// f6-qa-sheet02: real execution of the remaining AUTO-API test cases in
// docs/qa/test-cases/02-business-maker-checker.xlsx that had no
// existing automated coverage (TC-BIZ-008, 009, 021, 026).

import (
	"context"
	"testing"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/org"
	"github.com/hieumai1507/hpay-backend/internal/wallet"

	"github.com/google/uuid"
)

// TC-BIZ-008: amount 0 and negative are both rejected at validation, no
// disbursement created.
func TestQA_DisbursementZeroOrNegativeAmountRejected(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	if _, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 0, "", uuid.NewString()); !errIs(err, wallet.ErrAmountInvalid) {
		t.Fatalf("amount=0: want ErrAmountInvalid, got %v", err)
	}
	if _, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, -100_000, "", uuid.NewString()); !errIs(err, wallet.ErrAmountInvalid) {
		t.Fatalf("amount=-100000: want ErrAmountInvalid, got %v", err)
	}
}

// TC-BIZ-009: a receiver phone that doesn't belong to any user is rejected
// clearly, no disbursement left hanging for an unresolvable recipient.
func TestQA_DisbursementUnknownRecipientRejected(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	_, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, "0999999999", 100_000, "", uuid.NewString())
	if !errIs(err, org.ErrReceiverNotFound) {
		t.Fatalf("unknown recipient: want ErrReceiverNotFound, got %v", err)
	}
}

// TC-BIZ-021: a wrong OTP code does not finalize the request — it stays
// pending exactly as it was before the attempt.
func TestQA_DecideWrongOTPDoesNotFinalize(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 500_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	// A real challenge must exist first (otherwise Decide would reject for a
	// different reason — no active challenge — rather than "wrong code").
	if _, err := e.asvc.Challenge(ctx, f.checkers[0], reqID); err != nil {
		t.Fatalf("challenge: %v", err)
	}

	if _, err := e.asvc.Decide(ctx, f.checkers[0], reqID, true, "000000-wrong"); err == nil {
		t.Fatal("wrong OTP: want an error, got nil")
	}

	req, err := e.q.GetApprovalRequest(ctx, reqID)
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
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1, 1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 500_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	if _, err := e.decide(t, f.checkers[0], reqID, true); err != nil {
		t.Fatalf("checker A decide: %v", err)
	}
	e.drain(t)

	_, err = e.decide(t, f.checkers[1], reqID, true)
	if !errIs(err, approval.ErrAlreadyFinalized) {
		t.Fatalf("checker B after finalize: want ErrAlreadyFinalized, got %v", err)
	}
}

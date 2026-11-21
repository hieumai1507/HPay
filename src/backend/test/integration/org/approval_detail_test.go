package org

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestApprovalDetail_MakerCanView: maker can view their own request even though
// they are not a checker/org member matching the current level.
func TestApprovalDetail_MakerCanView(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "salary", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	d, err := e.Asvc.Detail(ctx, f.MakerID, reqID)
	if err != nil {
		t.Fatalf("maker detail: %v", err)
	}
	if d.Status != "pending" || d.CurrentLevel != 1 {
		t.Fatalf("detail: want pending level 1, got %+v", d)
	}
	if d.MakerID != f.MakerID || d.RefCode != res.RefCode || d.Amount != 1_000_000 {
		t.Fatalf("detail money facts wrong: %+v", d)
	}
	if d.SignaturesRequired != 1 {
		t.Fatalf("signatures_required: want 1, got %d", d.SignaturesRequired)
	}
	if len(d.Signatures) != 0 {
		t.Fatalf("no one has signed yet: want 0 signatures, got %d", len(d.Signatures))
	}
}

// TestApprovalDetail_EligibleCheckerCanView: an org checker can view it, even
// when their level does not match current_level (viewing is not approving).
func TestApprovalDetail_EligibleCheckerCanView(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1, 2}, []approval.Tier{
		{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 1},
	})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	// L2 checker views while current tier is still 1 — allowed to view, would
	// not be allowed to Decide (ErrWrongLevel), which this test does not call.
	d, err := e.Asvc.Detail(ctx, f.Checkers[1], reqID)
	if err != nil {
		t.Fatalf("L2 detail while tier=1: %v", err)
	}
	if d.CurrentLevel != 1 {
		t.Fatalf("detail current_level: want 1, got %d", d.CurrentLevel)
	}
}

// TestApprovalDetail_UnrelatedUserGetsNotFound: a user outside the org gets 404
// (ErrNotFound), not 403 — does not leak the request's existence.
func TestApprovalDetail_UnrelatedUserGetsNotFound(t *testing.T) {
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
	if _, err := e.Asvc.Detail(ctx, outsider, reqID); !testkit.ErrIs(err, approval.ErrNotFound) {
		t.Fatalf("outsider detail: want ErrNotFound, got %v", err)
	}

	// A random id that doesn't exist at all behaves the same way.
	if _, err := e.Asvc.Detail(ctx, f.MakerID, uuid.New()); !testkit.ErrIs(err, approval.ErrNotFound) {
		t.Fatalf("nonexistent request: want ErrNotFound, got %v", err)
	}
}

// TestApprovalDetail_SignatureHistoryOrdered: 2 checkers sign at 2 different
// levels — the response returns the correct order, names and actions.
func TestApprovalDetail_SignatureHistoryOrdered(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 3_000_000, []int{1, 2}, []approval.Tier{
		{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 1},
	})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	if _, err := e.Decide(t, f.Checkers[0], reqID, true); err != nil {
		t.Fatalf("L1 decide: %v", err)
	}
	if _, err := e.Decide(t, f.Checkers[1], reqID, true); err != nil {
		t.Fatalf("L2 decide: %v", err)
	}
	e.Drain(t)

	d, err := e.Asvc.Detail(ctx, f.MakerID, reqID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.Status != "approved" {
		t.Fatalf("status: want approved, got %s", d.Status)
	}
	if len(d.Signatures) != 2 {
		t.Fatalf("signatures: want 2, got %d", len(d.Signatures))
	}
	if d.Signatures[0].Level != 1 || d.Signatures[0].CheckerID != f.Checkers[0] || d.Signatures[0].Action != "approve" {
		t.Fatalf("signature[0]: want L1 checker approve, got %+v", d.Signatures[0])
	}
	if d.Signatures[1].Level != 2 || d.Signatures[1].CheckerID != f.Checkers[1] || d.Signatures[1].Action != "approve" {
		t.Fatalf("signature[1]: want L2 checker approve, got %+v", d.Signatures[1])
	}
	if d.Signatures[0].CheckerName == "" || d.Signatures[1].CheckerName == "" {
		t.Fatal("checker names must be resolved for the mobile timeline")
	}
	if d.FinalizedAt == nil {
		t.Fatal("finalized_at must be set once approved")
	}
}

// TestApprovalDetailByRefCode_MakerResolvesJustCreatedRequest: mirrors the
// real mobile flow — CreateDisbursement returns only ref_code (the
// approval_request doesn't exist until the pause hook runs), so the maker
// must be able to resolve the same Detail via ref_code once it's drained.
func TestApprovalDetailByRefCode_MakerResolvesJustCreatedRequest(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}

	// Before the worker drains, the approval_request doesn't exist yet.
	if _, err := e.Asvc.DetailByRefCode(ctx, f.MakerID, res.RefCode); !testkit.ErrIs(err, approval.ErrNotFound) {
		t.Fatalf("before drain: want ErrNotFound, got %v", err)
	}

	e.Drain(t)
	d, err := e.Asvc.DetailByRefCode(ctx, f.MakerID, res.RefCode)
	if err != nil {
		t.Fatalf("after drain: %v", err)
	}
	if d.RefCode != res.RefCode || d.Status != "pending" {
		t.Fatalf("detail by ref_code: want ref_code=%s status=pending, got %+v", res.RefCode, d)
	}

	// Same access rule applies: outsider still gets ErrNotFound.
	outsider, _, _ := e.UserWithMoney(t, 0)
	if _, err := e.Asvc.DetailByRefCode(ctx, outsider, res.RefCode); !testkit.ErrIs(err, approval.ErrNotFound) {
		t.Fatalf("outsider by ref_code: want ErrNotFound, got %v", err)
	}
}

// TestApprovalDetail_SignaturesRequiredReflectsPolicySnapshot: signatures
// required must come from the request's frozen snapshot, not a later policy
// edit (E09 — same invariant Decide already relies on).
func TestApprovalDetail_SignaturesRequiredReflectsPolicySnapshot(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	// Admin upgrades the policy to 2 tiers (3 signatures total) while pending.
	if _, err := e.Osvc.CreatePolicy(ctx, f.AdminID, f.OrgID, "default", 0, nil,
		[]approval.Tier{{Level: 1, RequiredApprovals: 2}, {Level: 2, RequiredApprovals: 1}}); err != nil {
		t.Fatal(err)
	}

	d, err := e.Asvc.Detail(ctx, f.MakerID, reqID)
	if err != nil {
		t.Fatal(err)
	}
	if d.SignaturesRequired != 1 {
		t.Fatalf("signatures_required must stay from the v1 snapshot: want 1, got %d", d.SignaturesRequired)
	}
}

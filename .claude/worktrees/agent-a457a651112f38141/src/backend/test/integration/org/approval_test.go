package org

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// TestOrgDisbursementTwoTierEndToEnd: maker → WAIT_APPROVAL → L1 signs → L2 signs →
// engine resume → money leaves the org wallet → completed; timeline contains WAIT_APPROVAL.
func TestOrgDisbursementTwoTierEndToEnd(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 10_000_000, []int{1, 2}, []approval.Tier{
		{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 1},
	})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 5_000_000, "salary", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t) // VALIDATE → FRAUD_CHECK → pause at WAIT_APPROVAL

	if got := e.TxnStatus(t, res.RefCode); got != "pending_approval" {
		t.Fatalf("txn status after pause: want pending_approval, got %s", got)
	}
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	// Inbox: L1 sees the task, L2 does not (current tier is 1).
	inbox1, err := e.Asvc.Inbox(ctx, f.Checkers[0])
	if err != nil || len(inbox1) != 1 {
		t.Fatalf("L1 inbox: want 1 task, got %d (err=%v)", len(inbox1), err)
	}
	inbox2, _ := e.Asvc.Inbox(ctx, f.Checkers[1])
	if len(inbox2) != 0 {
		t.Fatalf("L2 inbox before L1 signs: want 0, got %d", len(inbox2))
	}

	d1, err := e.Decide(t, f.Checkers[0], reqID, true)
	if err != nil {
		t.Fatalf("L1 decide: %v", err)
	}
	if d1.Status != "pending" || d1.CurrentLevel != 2 {
		t.Fatalf("after L1: want pending level 2, got %+v", d1)
	}

	inbox2, _ = e.Asvc.Inbox(ctx, f.Checkers[1])
	if len(inbox2) != 1 {
		t.Fatalf("L2 inbox after L1 signs: want 1, got %d", len(inbox2))
	}

	d2, err := e.Decide(t, f.Checkers[1], reqID, true)
	if err != nil {
		t.Fatalf("L2 decide: %v", err)
	}
	if d2.Status != "approved" {
		t.Fatalf("after L2: want approved, got %+v", d2)
	}
	e.Drain(t) // resume → MOVE_FUNDS → CONFIRM

	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn final status: want completed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, f.OrgWallet); got != 5_000_000 {
		t.Fatalf("org wallet: want 5000000 left, got %d", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, f.ReceiverWallet); got != 5_000_000 {
		t.Fatalf("receiver: want 5000000, got %d", got)
	}

	// Timeline shows the human gate as a completed step.
	var gateCount int
	if err := e.Pool.QueryRow(ctx, `
		SELECT count(*) FROM step_executions se
		JOIN workflow_instances wi ON wi.id = se.instance_id
		JOIN transactions t ON t.id = wi.transaction_id
		WHERE t.ref_code=$1 AND se.step_name='WAIT_APPROVAL' AND se.status='completed'`,
		res.RefCode).Scan(&gateCount); err != nil {
		t.Fatal(err)
	}
	if gateCount != 1 {
		t.Fatalf("WAIT_APPROVAL step_execution: want 1, got %d", gateCount)
	}
}

// TestOrgDisbursementRejectFailsClean: checker reject → txn failed, NO ledger entries.
func TestOrgDisbursementRejectFailsClean(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	d, err := e.Decide(t, f.Checkers[0], reqID, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "rejected" {
		t.Fatalf("want rejected, got %+v", d)
	}
	e.Drain(t)

	if got := e.TxnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("txn: want failed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, f.OrgWallet); got != 2_000_000 {
		t.Fatalf("org wallet untouched: want 2000000, got %d", got)
	}
	if net := e.TxnLedgerNet(t, res.RefCode, f.OrgWallet); net != 0 {
		t.Fatal("no ledger entries expected on reject")
	}
}

// TestEdge_E08_CheckerRaceSingleFinalize: 2 checkers at the same level approve concurrently
// on a tier needing 1 signature → FOR UPDATE serializes them: exactly one finalizes,
// the other gets ERR_ALREADY_FINALIZED; the engine resumes exactly once.
func TestEdge_E08_CheckerRaceSingleFinalize(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 3_000_000, []int{1, 1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "race", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	// Both checkers pass eligibility and get their own OTP while pending.
	codes := make([]string, 2)
	for i, c := range f.Checkers {
		code, err := e.Asvc.Challenge(ctx, c, reqID)
		if err != nil {
			t.Fatalf("challenge %d: %v", i, err)
		}
		codes[i] = code
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.Asvc.Decide(ctx, f.Checkers[i], reqID, true, codes[i])
		}(i)
	}
	wg.Wait()

	var ok, alreadyFinalized int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case testkit.ErrIs(err, approval.ErrAlreadyFinalized):
			alreadyFinalized++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || alreadyFinalized != 1 {
		t.Fatalf("want exactly 1 success + 1 ERR_ALREADY_FINALIZED, got ok=%d already=%d", ok, alreadyFinalized)
	}

	var actionCount int
	if err := e.Pool.QueryRow(ctx,
		`SELECT count(*) FROM approval_actions WHERE request_id=$1`, reqID).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 1 {
		t.Fatalf("approval_actions: want 1, got %d", actionCount)
	}

	e.Drain(t)
	if got := e.TxnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn: want completed, got %s", got)
	}
	// Money moved exactly once.
	if got := testutil.WalletBalance(t, e.Pool, f.ReceiverWallet); got != 1_000_000 {
		t.Fatalf("receiver: want 1000000 exactly once, got %d", got)
	}
}

// TestEdge_E09_PolicySnapshotSurvivesPolicyChange: a request created under policy v1
// (1 signature) is still approved under v1 after the admin switches to v2 (2 tiers);
// a NEW request created afterwards follows v2.
func TestEdge_E09_PolicySnapshotSurvivesPolicyChange(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 10_000_000, []int{1, 2}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	resA, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "under v1", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqA := testkit.ApprovalRequestFor(t, e, resA.RefCode)

	// Admin upgrades the policy to 2 tiers while A is pending.
	if _, err := e.Osvc.CreatePolicy(ctx, f.AdminID, f.OrgID, "default", 0, nil,
		[]approval.Tier{{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 1}}); err != nil {
		t.Fatal(err)
	}

	// A still finalizes with ONE signature — snapshot v1 (E09).
	dA, err := e.Decide(t, f.Checkers[0], reqA, true)
	if err != nil {
		t.Fatal(err)
	}
	if dA.Status != "approved" {
		t.Fatalf("request under old policy: want approved with 1 signature, got %+v", dA)
	}
	e.Drain(t)
	if got := e.TxnStatus(t, resA.RefCode); got != "completed" {
		t.Fatalf("txn A: want completed, got %s", got)
	}

	// A NEW disbursement snapshots v2 and needs the second tier.
	resB, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "under v2", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqB := testkit.ApprovalRequestFor(t, e, resB.RefCode)

	dB, err := e.Decide(t, f.Checkers[0], reqB, true)
	if err != nil {
		t.Fatal(err)
	}
	if dB.Status != "pending" || dB.CurrentLevel != 2 {
		t.Fatalf("request under new policy: want pending level 2 after L1, got %+v", dB)
	}
	if got := e.TxnStatus(t, resB.RefCode); got != "pending_approval" {
		t.Fatalf("txn B still gated: want pending_approval, got %s", got)
	}
}

// TestEdge_E10_ExpiredApprovalFailsClean: expired request → sweeper expires it →
// instance failed, txn failed, NO ledger entries; 2 parallel sweepers do not double-fire.
func TestEdge_E10_ExpiredApprovalFailsClean(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	f := testkit.NewOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.Osvc.CreateDisbursement(ctx, f.MakerID, f.OrgID, f.ReceiverPhone, 1_000_000, "expire me", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	reqID := testkit.ApprovalRequestFor(t, e, res.RefCode)

	if _, err := e.Pool.Exec(ctx,
		`UPDATE approval_requests SET expires_at = now() - interval '1 hour' WHERE id=$1`, reqID); err != nil {
		t.Fatal(err)
	}

	// Expired requests must refuse decisions even before the sweeper runs.
	if _, err := e.Decide(t, f.Checkers[0], reqID, true); !testkit.ErrIs(err, approval.ErrExpired) {
		t.Fatalf("decide on overdue request: want ErrExpired, got %v", err)
	}

	// Two concurrent sweepers: the guarded UPDATE means only one fires.
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.Asvc.ExpireDue(ctx); err != nil {
				t.Errorf("sweep: %v", err)
			}
		}()
	}
	wg.Wait()
	e.Drain(t)

	if got := e.ApprovalStatus(t, reqID); got != "expired" {
		t.Fatalf("request: want expired, got %s", got)
	}
	if got := e.TxnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("txn: want failed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.Pool, f.OrgWallet); got != 2_000_000 {
		t.Fatalf("org wallet untouched: want 2000000, got %d", got)
	}
	if net := e.TxnLedgerNet(t, res.RefCode, f.OrgWallet); net != 0 {
		t.Fatal("no ledger entries expected on expiry")
	}
	// Exactly one recorded WAIT_APPROVAL execution despite two sweepers.
	var gateCount int
	if err := e.Pool.QueryRow(ctx, `
		SELECT count(*) FROM step_executions se
		JOIN workflow_instances wi ON wi.id = se.instance_id
		JOIN transactions t ON t.id = wi.transaction_id
		WHERE t.ref_code=$1 AND se.step_name='WAIT_APPROVAL'`, res.RefCode).Scan(&gateCount); err != nil {
		t.Fatal(err)
	}
	if gateCount != 1 {
		t.Fatalf("WAIT_APPROVAL executions: want 1, got %d", gateCount)
	}
}

// TestApprovalEligibility: a maker cannot approve their own request; a user outside the org
// cannot see that the request exists; a checker at the wrong level is blocked.
func TestApprovalEligibility(t *testing.T) {
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

	// Maker ≠ Checker: maker's role blocks signing their own order.
	if _, err := e.Asvc.Challenge(ctx, f.MakerID, reqID); !testkit.ErrIs(err, approval.ErrNotChecker) {
		t.Fatalf("maker challenge: want ErrNotChecker, got %v", err)
	}
	// A user outside the org must not learn the request exists.
	outsider, _, _ := e.UserWithMoney(t, 0)
	if _, err := e.Asvc.Challenge(ctx, outsider, reqID); !testkit.ErrIs(err, approval.ErrNotFound) {
		t.Fatalf("outsider challenge: want ErrNotFound, got %v", err)
	}
	// L2 cannot sign while the current tier is 1.
	if _, err := e.Asvc.Challenge(ctx, f.Checkers[1], reqID); !testkit.ErrIs(err, approval.ErrWrongLevel) {
		t.Fatalf("L2 at tier 1: want ErrWrongLevel, got %v", err)
	}
	// No matching policy → maker gets an immediate error on a NEW org.
	f2 := testkit.NewOrgFixture(t, e, 1_000_000, []int{1}, nil) // no policy installed
	if _, err := e.Osvc.CreateDisbursement(ctx, f2.MakerID, f2.OrgID, f2.ReceiverPhone, 500_000, "", uuid.NewString()); !testkit.ErrIs(err, approval.ErrNoPolicy) {
		t.Fatalf("no policy: want ErrNoPolicy, got %v", err)
	}
}

func TestIntegration_ListUserOrgs(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()

	// Create org with 10M funding
	f := testkit.NewOrgFixture(t, e, 10_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	// 1. Admin checks their list
	orgsAdmin, err := e.Osvc.ListUserOrganizations(ctx, f.AdminID)
	if err != nil {
		t.Fatalf("ListUserOrganizations admin: %v", err)
	}
	if len(orgsAdmin) != 1 {
		t.Fatalf("Admin org count: want 1, got %d", len(orgsAdmin))
	}
	if orgsAdmin[0].OrgID != f.OrgID {
		t.Fatalf("Admin org ID: want %s, got %s", f.OrgID, orgsAdmin[0].OrgID)
	}
	if orgsAdmin[0].Role != "admin" {
		t.Fatalf("Admin role: want admin, got %s", orgsAdmin[0].Role)
	}
	if orgsAdmin[0].Balance != 10_000_000 {
		t.Fatalf("Admin org balance: want 10000000, got %d", orgsAdmin[0].Balance)
	}

	// 2. Maker checks their list
	orgsMaker, err := e.Osvc.ListUserOrganizations(ctx, f.MakerID)
	if err != nil {
		t.Fatalf("ListUserOrganizations maker: %v", err)
	}
	if len(orgsMaker) != 1 {
		t.Fatalf("Maker org count: want 1, got %d", len(orgsMaker))
	}
	if orgsMaker[0].Role != "maker" {
		t.Fatalf("Maker role: want maker, got %s", orgsMaker[0].Role)
	}

	// 3. Checker checks their list
	orgsChecker, err := e.Osvc.ListUserOrganizations(ctx, f.Checkers[0])
	if err != nil {
		t.Fatalf("ListUserOrganizations checker: %v", err)
	}
	if len(orgsChecker) != 1 {
		t.Fatalf("Checker org count: want 1, got %d", len(orgsChecker))
	}
	if orgsChecker[0].Role != "checker" {
		t.Fatalf("Checker role: want checker, got %s", orgsChecker[0].Role)
	}
	if orgsChecker[0].CheckerLevel != 1 {
		t.Fatalf("Checker level: want 1, got %d", orgsChecker[0].CheckerLevel)
	}

	// 4. Outsider checks their list
	outsiderID, _, _ := e.UserWithMoney(t, 0)
	orgsOutsider, err := e.Osvc.ListUserOrganizations(ctx, outsiderID)
	if err != nil {
		t.Fatalf("ListUserOrganizations outsider: %v", err)
	}
	if len(orgsOutsider) != 0 {
		t.Fatalf("Outsider org count: want 0, got %d", len(orgsOutsider))
	}
}

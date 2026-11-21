package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
)

// orgFixture wires a funded org with one admin, one maker, N checkers and an
// external receiver — everything a maker-checker scenario needs.
type orgFixture struct {
	orgID          uuid.UUID
	orgWallet      uuid.UUID
	adminID        uuid.UUID
	makerID        uuid.UUID
	checkers       []uuid.UUID // index i has checker_level i+1... unless levels given
	receiverID     uuid.UUID
	receiverWallet uuid.UUID
	receiverPhone  string
}

// newOrgFixture creates the org, funds it, adds checkers at the given levels
// and installs one active policy with the given tiers.
func newOrgFixture(t *testing.T, e *env, fundAmount int64, checkerLevels []int, tiers []approval.Tier) *orgFixture {
	t.Helper()
	ctx := context.Background()

	adminID, _, _ := e.userWithMoney(t, 0)
	makerID, _, makerPhone := e.userWithMoney(t, 0)
	receiverID, receiverWallet, receiverPhone := e.userWithMoney(t, 0)

	o, err := e.osvc.CreateOrg(ctx, adminID, "Test Org "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	if err := e.osvc.AddMember(ctx, adminID, o.OrgID, makerPhone, "maker", 0); err != nil {
		t.Fatalf("add maker: %v", err)
	}
	f := &orgFixture{
		orgID: o.OrgID, orgWallet: o.WalletID, adminID: adminID, makerID: makerID,
		receiverID: receiverID, receiverWallet: receiverWallet, receiverPhone: receiverPhone,
	}
	for _, lvl := range checkerLevels {
		checkerID, _, checkerPhone := e.userWithMoney(t, 0)
		if err := e.osvc.AddMember(ctx, adminID, o.OrgID, checkerPhone, "checker", lvl); err != nil {
			t.Fatalf("add checker L%d: %v", lvl, err)
		}
		f.checkers = append(f.checkers, checkerID)
	}
	if tiers != nil {
		if _, err := e.osvc.CreatePolicy(ctx, adminID, o.OrgID, "default", 0, nil, tiers); err != nil {
			t.Fatalf("create policy: %v", err)
		}
	}
	if fundAmount > 0 {
		if _, err := e.osvc.Topup(ctx, adminID, o.OrgID, fundAmount, uuid.NewString()); err != nil {
			t.Fatalf("fund org: %v", err)
		}
	}
	return f
}

// approvalRequestFor finds the approval request created by the pause hook.
func approvalRequestFor(t *testing.T, e *env, refCode string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.pool.QueryRow(context.Background(), `
		SELECT r.id FROM approval_requests r
		JOIN transactions t ON t.id = r.subject_id
		WHERE t.ref_code = $1`, refCode).Scan(&id); err != nil {
		t.Fatalf("approval request for %s: %v", refCode, err)
	}
	return id
}

func (e *env) txnStatus(t *testing.T, refCode string) string {
	t.Helper()
	var s string
	if err := e.pool.QueryRow(context.Background(),
		`SELECT status FROM transactions WHERE ref_code=$1`, refCode).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *env) approvalStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := e.pool.QueryRow(context.Background(),
		`SELECT status FROM approval_requests WHERE id=$1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// decide runs the checker's challenge+decide pair (OTP code comes back in dev env).
func (e *env) decide(t *testing.T, checkerID, reqID uuid.UUID, approve bool) (approval.DecideResult, error) {
	t.Helper()
	code, err := e.asvc.Challenge(context.Background(), checkerID, reqID)
	if err != nil {
		return approval.DecideResult{}, err
	}
	return e.asvc.Decide(context.Background(), checkerID, reqID, approve, code)
}

// TestOrgDisbursementTwoTierEndToEnd: maker → WAIT_APPROVAL → L1 ký → L2 ký →
// engine resume → tiền đi từ ví org → completed; timeline có WAIT_APPROVAL.
func TestOrgDisbursementTwoTierEndToEnd(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 10_000_000, []int{1, 2}, []approval.Tier{
		{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 1},
	})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 5_000_000, "salary", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t) // VALIDATE → FRAUD_CHECK → pause at WAIT_APPROVAL

	if got := e.txnStatus(t, res.RefCode); got != "pending_approval" {
		t.Fatalf("txn status after pause: want pending_approval, got %s", got)
	}
	reqID := approvalRequestFor(t, e, res.RefCode)

	// Inbox: L1 sees the task, L2 does not (current tier is 1).
	inbox1, err := e.asvc.Inbox(ctx, f.checkers[0])
	if err != nil || len(inbox1) != 1 {
		t.Fatalf("L1 inbox: want 1 task, got %d (err=%v)", len(inbox1), err)
	}
	inbox2, _ := e.asvc.Inbox(ctx, f.checkers[1])
	if len(inbox2) != 0 {
		t.Fatalf("L2 inbox before L1 signs: want 0, got %d", len(inbox2))
	}

	d1, err := e.decide(t, f.checkers[0], reqID, true)
	if err != nil {
		t.Fatalf("L1 decide: %v", err)
	}
	if d1.Status != "pending" || d1.CurrentLevel != 2 {
		t.Fatalf("after L1: want pending level 2, got %+v", d1)
	}

	inbox2, _ = e.asvc.Inbox(ctx, f.checkers[1])
	if len(inbox2) != 1 {
		t.Fatalf("L2 inbox after L1 signs: want 1, got %d", len(inbox2))
	}

	d2, err := e.decide(t, f.checkers[1], reqID, true)
	if err != nil {
		t.Fatalf("L2 decide: %v", err)
	}
	if d2.Status != "approved" {
		t.Fatalf("after L2: want approved, got %+v", d2)
	}
	e.drain(t) // resume → MOVE_FUNDS → CONFIRM

	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn final status: want completed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.pool, f.orgWallet); got != 5_000_000 {
		t.Fatalf("org wallet: want 5000000 left, got %d", got)
	}
	if got := testutil.WalletBalance(t, e.pool, f.receiverWallet); got != 5_000_000 {
		t.Fatalf("receiver: want 5000000, got %d", got)
	}

	// Timeline shows the human gate as a completed step.
	var gateCount int
	if err := e.pool.QueryRow(ctx, `
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

// TestOrgDisbursementRejectFailsClean: checker reject → txn failed, KHÔNG bút toán.
func TestOrgDisbursementRejectFailsClean(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	d, err := e.decide(t, f.checkers[0], reqID, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "rejected" {
		t.Fatalf("want rejected, got %+v", d)
	}
	e.drain(t)

	if got := e.txnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("txn: want failed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.pool, f.orgWallet); got != 2_000_000 {
		t.Fatalf("org wallet untouched: want 2000000, got %d", got)
	}
	if net := e.txnLedgerNet(t, res.RefCode, f.orgWallet); net != 0 {
		t.Fatal("no ledger entries expected on reject")
	}
}

// TestEdge_E08_CheckerRaceSingleFinalize: 2 checker cùng level bấm approve đồng
// thời trên tier cần 1 chữ ký → FOR UPDATE tuần tự hóa: đúng 1 người finalize,
// người kia nhận ERR_ALREADY_FINALIZED; engine resume đúng 1 lần.
func TestEdge_E08_CheckerRaceSingleFinalize(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 3_000_000, []int{1, 1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "race", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	// Both checkers pass eligibility and get their own OTP while pending.
	codes := make([]string, 2)
	for i, c := range f.checkers {
		code, err := e.asvc.Challenge(ctx, c, reqID)
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
			_, errs[i] = e.asvc.Decide(ctx, f.checkers[i], reqID, true, codes[i])
		}(i)
	}
	wg.Wait()

	var ok, alreadyFinalized int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errIs(err, approval.ErrAlreadyFinalized):
			alreadyFinalized++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || alreadyFinalized != 1 {
		t.Fatalf("want exactly 1 success + 1 ERR_ALREADY_FINALIZED, got ok=%d already=%d", ok, alreadyFinalized)
	}

	var actionCount int
	if err := e.pool.QueryRow(ctx,
		`SELECT count(*) FROM approval_actions WHERE request_id=$1`, reqID).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 1 {
		t.Fatalf("approval_actions: want 1, got %d", actionCount)
	}

	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn: want completed, got %s", got)
	}
	// Money moved exactly once.
	if got := testutil.WalletBalance(t, e.pool, f.receiverWallet); got != 1_000_000 {
		t.Fatalf("receiver: want 1000000 exactly once, got %d", got)
	}
}

// TestEdge_E09_PolicySnapshotSurvivesPolicyChange: lệnh tạo dưới policy v1
// (1 chữ ký) vẫn duyệt theo v1 sau khi admin đổi sang v2 (2 tier);
// lệnh MỚI tạo sau đó theo v2.
func TestEdge_E09_PolicySnapshotSurvivesPolicyChange(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 10_000_000, []int{1, 2}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	resA, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "under v1", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqA := approvalRequestFor(t, e, resA.RefCode)

	// Admin upgrades the policy to 2 tiers while A is pending.
	if _, err := e.osvc.CreatePolicy(ctx, f.adminID, f.orgID, "default", 0, nil,
		[]approval.Tier{{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 1}}); err != nil {
		t.Fatal(err)
	}

	// A still finalizes with ONE signature — snapshot v1 (E09).
	dA, err := e.decide(t, f.checkers[0], reqA, true)
	if err != nil {
		t.Fatal(err)
	}
	if dA.Status != "approved" {
		t.Fatalf("request under old policy: want approved with 1 signature, got %+v", dA)
	}
	e.drain(t)
	if got := e.txnStatus(t, resA.RefCode); got != "completed" {
		t.Fatalf("txn A: want completed, got %s", got)
	}

	// A NEW disbursement snapshots v2 and needs the second tier.
	resB, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "under v2", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqB := approvalRequestFor(t, e, resB.RefCode)

	dB, err := e.decide(t, f.checkers[0], reqB, true)
	if err != nil {
		t.Fatal(err)
	}
	if dB.Status != "pending" || dB.CurrentLevel != 2 {
		t.Fatalf("request under new policy: want pending level 2 after L1, got %+v", dB)
	}
	if got := e.txnStatus(t, resB.RefCode); got != "pending_approval" {
		t.Fatalf("txn B still gated: want pending_approval, got %s", got)
	}
}

// TestEdge_E10_ExpiredApprovalFailsClean: request quá hạn → sweeper expire →
// instance failed, txn failed, KHÔNG bút toán; 2 sweeper song song không double-fire.
func TestEdge_E10_ExpiredApprovalFailsClean(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "expire me", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	if _, err := e.pool.Exec(ctx,
		`UPDATE approval_requests SET expires_at = now() - interval '1 hour' WHERE id=$1`, reqID); err != nil {
		t.Fatal(err)
	}

	// Expired requests must refuse decisions even before the sweeper runs.
	if _, err := e.decide(t, f.checkers[0], reqID, true); !errIs(err, approval.ErrExpired) {
		t.Fatalf("decide on overdue request: want ErrExpired, got %v", err)
	}

	// Two concurrent sweepers: the guarded UPDATE means only one fires.
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.asvc.ExpireDue(ctx); err != nil {
				t.Errorf("sweep: %v", err)
			}
		}()
	}
	wg.Wait()
	e.drain(t)

	if got := e.approvalStatus(t, reqID); got != "expired" {
		t.Fatalf("request: want expired, got %s", got)
	}
	if got := e.txnStatus(t, res.RefCode); got != "failed" {
		t.Fatalf("txn: want failed, got %s", got)
	}
	if got := testutil.WalletBalance(t, e.pool, f.orgWallet); got != 2_000_000 {
		t.Fatalf("org wallet untouched: want 2000000, got %d", got)
	}
	if net := e.txnLedgerNet(t, res.RefCode, f.orgWallet); net != 0 {
		t.Fatal("no ledger entries expected on expiry")
	}
	// Exactly one recorded WAIT_APPROVAL execution despite two sweepers.
	var gateCount int
	if err := e.pool.QueryRow(ctx, `
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

// TestApprovalEligibility: maker không tự duyệt được lệnh mình; người ngoài org
// không thấy request tồn tại; checker sai level bị chặn.
func TestApprovalEligibility(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	f := newOrgFixture(t, e, 2_000_000, []int{1, 2}, []approval.Tier{
		{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 1},
	})

	res, err := e.osvc.CreateDisbursement(ctx, f.makerID, f.orgID, f.receiverPhone, 1_000_000, "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	reqID := approvalRequestFor(t, e, res.RefCode)

	// Maker ≠ Checker: maker's role blocks signing their own order.
	if _, err := e.asvc.Challenge(ctx, f.makerID, reqID); !errIs(err, approval.ErrNotChecker) {
		t.Fatalf("maker challenge: want ErrNotChecker, got %v", err)
	}
	// A user outside the org must not learn the request exists.
	outsider, _, _ := e.userWithMoney(t, 0)
	if _, err := e.asvc.Challenge(ctx, outsider, reqID); !errIs(err, approval.ErrNotFound) {
		t.Fatalf("outsider challenge: want ErrNotFound, got %v", err)
	}
	// L2 cannot sign while the current tier is 1.
	if _, err := e.asvc.Challenge(ctx, f.checkers[1], reqID); !errIs(err, approval.ErrWrongLevel) {
		t.Fatalf("L2 at tier 1: want ErrWrongLevel, got %v", err)
	}
	// No matching policy → maker gets an immediate error on a NEW org.
	f2 := newOrgFixture(t, e, 1_000_000, []int{1}, nil) // no policy installed
	if _, err := e.osvc.CreateDisbursement(ctx, f2.makerID, f2.orgID, f2.receiverPhone, 500_000, "", uuid.NewString()); !errIs(err, approval.ErrNoPolicy) {
		t.Fatalf("no policy: want ErrNoPolicy, got %v", err)
	}
}

func TestIntegration_ListUserOrgs(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()

	// Create org with 10M funding
	f := newOrgFixture(t, e, 10_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	// 1. Admin checks their list
	orgsAdmin, err := e.osvc.ListUserOrganizations(ctx, f.adminID)
	if err != nil {
		t.Fatalf("ListUserOrganizations admin: %v", err)
	}
	if len(orgsAdmin) != 1 {
		t.Fatalf("Admin org count: want 1, got %d", len(orgsAdmin))
	}
	if orgsAdmin[0].OrgID != f.orgID {
		t.Fatalf("Admin org ID: want %s, got %s", f.orgID, orgsAdmin[0].OrgID)
	}
	if orgsAdmin[0].Role != "admin" {
		t.Fatalf("Admin role: want admin, got %s", orgsAdmin[0].Role)
	}
	if orgsAdmin[0].Balance != 10_000_000 {
		t.Fatalf("Admin org balance: want 10000000, got %d", orgsAdmin[0].Balance)
	}

	// 2. Maker checks their list
	orgsMaker, err := e.osvc.ListUserOrganizations(ctx, f.makerID)
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
	orgsChecker, err := e.osvc.ListUserOrganizations(ctx, f.checkers[0])
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
	outsiderID, _, _ := e.userWithMoney(t, 0)
	orgsOutsider, err := e.osvc.ListUserOrganizations(ctx, outsiderID)
	if err != nil {
		t.Fatalf("ListUserOrganizations outsider: %v", err)
	}
	if len(orgsOutsider) != 0 {
		t.Fatalf("Outsider org count: want 0, got %d", len(orgsOutsider))
	}
}

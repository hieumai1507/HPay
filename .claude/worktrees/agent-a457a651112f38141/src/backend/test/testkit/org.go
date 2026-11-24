package testkit

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/approval"
)

// orgFixture wires a funded org with one admin, one maker, N checkers and an
// external receiver — everything a maker-checker scenario needs.
type OrgFixture struct {
	OrgID          uuid.UUID
	OrgWallet      uuid.UUID
	AdminID        uuid.UUID
	MakerID        uuid.UUID
	Checkers       []uuid.UUID // index i has checker_level i+1... unless levels given
	ReceiverID     uuid.UUID
	ReceiverWallet uuid.UUID
	ReceiverPhone  string
}

// newOrgFixture creates the org, funds it, adds checkers at the given levels
// and installs one active policy with the given tiers.
func NewOrgFixture(t *testing.T, e *Env, fundAmount int64, checkerLevels []int, tiers []approval.Tier) *OrgFixture {
	t.Helper()
	ctx := context.Background()

	adminID, _, _ := e.UserWithMoney(t, 0)
	makerID, _, makerPhone := e.UserWithMoney(t, 0)
	receiverID, receiverWallet, receiverPhone := e.UserWithMoney(t, 0)

	o, err := e.Osvc.CreateOrg(ctx, adminID, "Test Org "+uuid.NewString()[:8])
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	if err := e.Osvc.AddMember(ctx, adminID, o.OrgID, makerPhone, "maker", 0); err != nil {
		t.Fatalf("add maker: %v", err)
	}
	f := &OrgFixture{
		OrgID: o.OrgID, OrgWallet: o.WalletID, AdminID: adminID, MakerID: makerID,
		ReceiverID: receiverID, ReceiverWallet: receiverWallet, ReceiverPhone: receiverPhone,
	}
	for _, lvl := range checkerLevels {
		checkerID, _, checkerPhone := e.UserWithMoney(t, 0)
		if err := e.Osvc.AddMember(ctx, adminID, o.OrgID, checkerPhone, "checker", lvl); err != nil {
			t.Fatalf("add checker L%d: %v", lvl, err)
		}
		f.Checkers = append(f.Checkers, checkerID)
	}
	if tiers != nil {
		if _, err := e.Osvc.CreatePolicy(ctx, adminID, o.OrgID, "default", 0, nil, tiers); err != nil {
			t.Fatalf("create policy: %v", err)
		}
	}
	if fundAmount > 0 {
		if _, err := e.Osvc.Topup(ctx, adminID, o.OrgID, fundAmount, uuid.NewString()); err != nil {
			t.Fatalf("fund org: %v", err)
		}
	}
	return f
}

// approvalRequestFor finds the approval request created by the pause hook.
func ApprovalRequestFor(t *testing.T, e *Env, refCode string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.Pool.QueryRow(context.Background(), `
		SELECT r.id FROM approval_requests r
		JOIN transactions t ON t.id = r.subject_id
		WHERE t.ref_code = $1`, refCode).Scan(&id); err != nil {
		t.Fatalf("approval request for %s: %v", refCode, err)
	}
	return id
}

func (e *Env) TxnStatus(t *testing.T, refCode string) string {
	t.Helper()
	var s string
	if err := e.Pool.QueryRow(context.Background(),
		`SELECT status FROM transactions WHERE ref_code=$1`, refCode).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func (e *Env) ApprovalStatus(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var s string
	if err := e.Pool.QueryRow(context.Background(),
		`SELECT status FROM approval_requests WHERE id=$1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// decide runs the checker's challenge+decide pair (OTP code comes back in dev env).
func (e *Env) Decide(t *testing.T, checkerID, reqID uuid.UUID, approve bool) (approval.DecideResult, error) {
	t.Helper()
	code, err := e.Asvc.Challenge(context.Background(), checkerID, reqID)
	if err != nil {
		return approval.DecideResult{}, err
	}
	return e.Asvc.Decide(context.Background(), checkerID, reqID, approve, code)
}

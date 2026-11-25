package admin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/listing"
	"github.com/hieumai1507/hpay-backend/internal/product"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// Workflow instances filters (docs plan workflow-instances-filters):
// ListWorkflowInstances now honors status/definition/stuck_only — the 3
// controls WorkflowPage.tsx already renders — plus the shared listing.Filter
// (ref_code search, created_at range) every other admin list supports.
//
// Rows are seeded directly into workflow_instances rather than driven through
// the engine, so each test controls status/created_at precisely without
// depending on org_disbursement's approval flow or timing.

func seedWorkflowInstance(t *testing.T, e *testkit.Env, defName, status string, createdAt time.Time, refCode string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	def, err := e.Q.GetActiveDefinitionByName(ctx, defName)
	if err != nil {
		t.Fatalf("definition %s: %v", defName, err)
	}
	var txnID *uuid.UUID
	if refCode != "" {
		userID, walletID, _ := e.UserWithMoney(t, 0)
		var id uuid.UUID
		if err := e.Pool.QueryRow(ctx, `
			INSERT INTO transactions (ref_code, type, sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by, status)
			VALUES ($1, 'p2p', $2, $2, 400, 0, $3, 'completed')
			RETURNING id`, refCode, walletID, userID).Scan(&id); err != nil {
			t.Fatalf("seed transaction: %v", err)
		}
		txnID = &id
	}
	var id uuid.UUID
	if err := e.Pool.QueryRow(ctx, `
		INSERT INTO workflow_instances (definition_id, transaction_id, status, current_step, context, created_at, updated_at)
		VALUES ($1, $2, $3, 'DONE', '{}', $4, $4)
		RETURNING id`, def.ID, txnID, status, createdAt).Scan(&id); err != nil {
		t.Fatalf("seed instance: %v", err)
	}
	return id
}

func containsID(rows []store.ListWorkflowInstancesRow, id uuid.UUID) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

func TestWorkflowInstancesFilter_Status(t *testing.T) {
	e := testkit.NewEnv(t, "")
	svc := product.NewService(e.Pool)
	now := time.Now()

	running := seedWorkflowInstance(t, e, "p2p_transfer", "running", now, "")
	completed := seedWorkflowInstance(t, e, "p2p_transfer", "completed", now, "")

	rows, total, err := svc.ListWorkflowInstances(context.Background(), listing.Filter{}, product.WorkflowInstanceFilter{Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(rows, completed) {
		t.Fatalf("want completed instance %s in results", completed)
	}
	if containsID(rows, running) {
		t.Fatalf("running instance %s must be excluded by status=completed", running)
	}
	if total < 1 {
		t.Fatalf("total must count at least the seeded completed row, got %d", total)
	}
}

func TestWorkflowInstancesFilter_StatusWaitingApproval(t *testing.T) {
	e := testkit.NewEnv(t, "")
	svc := product.NewService(e.Pool)
	now := time.Now()

	waiting := seedWorkflowInstance(t, e, "org_disbursement", "waiting_approval", now, "")
	running := seedWorkflowInstance(t, e, "org_disbursement", "running", now, "")

	// The handler is what maps the UI's short "waiting" label to
	// "waiting_approval" (docs plan workflow-instances-filters); the service
	// itself takes the real enum value directly.
	rows, total, err := svc.ListWorkflowInstances(context.Background(), listing.Filter{}, product.WorkflowInstanceFilter{Status: "waiting_approval"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(rows, waiting) {
		t.Fatalf("want waiting_approval instance %s in results", waiting)
	}
	if containsID(rows, running) {
		t.Fatalf("running instance %s must be excluded by status=waiting_approval", running)
	}
	_ = total
}

func TestWorkflowInstancesFilter_Definition(t *testing.T) {
	e := testkit.NewEnv(t, "")
	svc := product.NewService(e.Pool)
	now := time.Now()

	p2p := seedWorkflowInstance(t, e, "p2p_transfer", "running", now, "")
	org := seedWorkflowInstance(t, e, "org_disbursement", "running", now, "")

	rows, _, err := svc.ListWorkflowInstances(context.Background(), listing.Filter{}, product.WorkflowInstanceFilter{Definition: "org_disbursement"})
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(rows, org) {
		t.Fatalf("want org_disbursement instance %s in results", org)
	}
	if containsID(rows, p2p) {
		t.Fatalf("p2p_transfer instance %s must be excluded by definition=org_disbursement", p2p)
	}
}

func TestWorkflowInstancesFilter_StuckOnly(t *testing.T) {
	e := testkit.NewEnv(t, "")
	svc := product.NewService(e.Pool)

	fresh := seedWorkflowInstance(t, e, "p2p_transfer", "running", time.Now(), "")
	stale := seedWorkflowInstance(t, e, "p2p_transfer", "running", time.Now().Add(-10*time.Minute), "")
	staleButDone := seedWorkflowInstance(t, e, "p2p_transfer", "completed", time.Now().Add(-10*time.Minute), "")

	rows, _, err := svc.ListWorkflowInstances(context.Background(), listing.Filter{}, product.WorkflowInstanceFilter{StuckOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(rows, stale) {
		t.Fatalf("want the stale running instance %s in results", stale)
	}
	if containsID(rows, fresh) {
		t.Fatalf("fresh running instance %s must be excluded (not old enough)", fresh)
	}
	if containsID(rows, staleButDone) {
		t.Fatalf("stale but completed instance %s must be excluded (not running)", staleButDone)
	}
}

func TestWorkflowInstancesFilter_RefCodeSearch(t *testing.T) {
	e := testkit.NewEnv(t, "")
	svc := product.NewService(e.Pool)
	now := time.Now()
	tag := uuid.NewString()[:8]

	match := seedWorkflowInstance(t, e, "p2p_transfer", "completed", now, "REF-"+tag)
	other := seedWorkflowInstance(t, e, "p2p_transfer", "completed", now, "")

	rows, _, err := svc.ListWorkflowInstances(context.Background(), listing.Filter{Name: tag}, product.WorkflowInstanceFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if !containsID(rows, match) {
		t.Fatalf("want the matching ref_code instance %s in results", match)
	}
	if containsID(rows, other) {
		t.Fatalf("non-matching instance %s must be excluded", other)
	}
}

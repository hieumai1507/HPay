package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// G12: parked money cases must be visible. These tests pin the filters, the
// count endpoint and its permission (spec docs/specs/g12-needs-attention-queue).

type attentionSummary struct {
	NeedsManualApprovals        int64 `json:"needs_manual_approvals"`
	CompensationFailedWorkflows int64 `json:"compensation_failed_workflows"`
	Total                       int64 `json:"total"`
}

func getSummary(t *testing.T, rig *testkit.AdminRig, tok string) attentionSummary {
	t.Helper()
	w := rig.Call("GET", "/v1/admin/attention/summary", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("summary: %d %s", w.Code, w.Body.String())
	}
	var s attentionSummary
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// parkReversal creates a reversal request for a fresh completed transfer and parks it
// as needs_manual the way finalizeOrPark leaves it (status, finalized_at, audit row).
func parkReversal(t *testing.T, rig *testkit.AdminRig) (reqID uuid.UUID, ref string) {
	t.Helper()
	e := rig.E
	ctx := context.Background()
	ref, _, _, _, _ = e.CompletedTransfer(t, 1_234)
	cr, err := e.Revsvc.CreateRequest(ctx, e.SeedAdminID(t), ref, "receiver spent it")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE approval_requests SET status='needs_manual', finalized_at=now() WHERE id=$1`, cr.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pool.Exec(ctx, `INSERT INTO audit_logs (actor_id, actor_type, action, resource, reason)
		VALUES ($1, 'admin', 'reversal.needs_manual', $2, 'receiver balance too low')`,
		e.SeedAdminID(t), "transaction "+ref+" (approval_request "+cr.RequestID.String()+")"); err != nil {
		t.Fatal(err)
	}
	return cr.RequestID, ref
}

func listIDs(t *testing.T, w *httptest.ResponseRecorder, key string) map[string]map[string]any {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body[key], &rows); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]any{}
	for _, r := range rows {
		out[r["id"].(string)] = r
	}
	return out
}

// TC-G12-001 and TC-G12-003: status=needs_manual lists parked requests with money facts and the reason.
func TestAttention_ApprovalsFilterNeedsManual(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	parked, ref := parkReversal(t, rig)

	// a still-pending request must not show up under needs_manual
	pendingRef, _, _, _, _ := rig.E.CompletedTransfer(t, 220)
	pend, err := rig.E.Revsvc.CreateRequest(context.Background(), rig.E.SeedAdminID(t), pendingRef, "still pending")
	if err != nil {
		t.Fatal(err)
	}

	rows := listIDs(t, rig.Call("GET", "/v1/admin/approvals?status=needs_manual&limit=100", tok, nil), "approvals")
	row, ok := rows[parked.String()]
	if !ok {
		t.Fatalf("parked request %s missing from status=needs_manual", parked)
	}
	if _, bad := rows[pend.RequestID.String()]; bad {
		t.Errorf("pending request %s must not be listed under needs_manual", pend.RequestID)
	}
	if row["ref_code"] != ref || row["amount"].(float64) != 1_234 {
		t.Errorf("row money facts: ref=%v amount=%v, want %s / 1234", row["ref_code"], row["amount"], ref)
	}
	if row["reason"] != "receiver balance too low" {
		t.Errorf("row reason = %v, want the audit reason", row["reason"])
	}
	if row["parked_at"] == nil || row["parked_at"] == "" {
		t.Errorf("row must carry parked_at (age)")
	}

	// default listing (pending) must not contain the parked one
	pending := listIDs(t, rig.Call("GET", "/v1/admin/approvals?limit=100", tok, nil), "approvals")
	if _, bad := pending[parked.String()]; bad {
		t.Errorf("a needs_manual request must not appear in the pending inbox")
	}
}

// TC-G12-004: any other status is still refused.
func TestAttention_ApprovalsUnknownStatusRefused(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	if w := rig.Call("GET", "/v1/admin/approvals?status=approved", tok, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("status=approved: got %d, want 400", w.Code)
	}
}

// TC-G12-002: the workflow list filters on compensation_failed.
func TestAttention_WorkflowsFilterCompensationFailed(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	failed := seedWorkflowInstance(t, rig.E, "p2p_transfer", "compensation_failed", time.Now(), "")
	other := seedWorkflowInstance(t, rig.E, "p2p_transfer", "compensated", time.Now(), "")

	rows := listIDs(t, rig.Call("GET", "/v1/admin/workflows?status=compensation_failed&limit=100", tok, nil), "workflows")
	if _, ok := rows[failed.String()]; !ok {
		t.Fatalf("compensation_failed instance %s missing", failed)
	}
	if _, bad := rows[other.String()]; bad {
		t.Errorf("compensated instance %s must be excluded", other)
	}
}

// TC-G12-005 and TC-G12-006: the summary counts exactly the parked cases.
func TestAttention_SummaryCounts(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	before := getSummary(t, rig, tok)

	parkReversal(t, rig)
	seedWorkflowInstance(t, rig.E, "p2p_transfer", "compensation_failed", time.Now(), "")
	seedWorkflowInstance(t, rig.E, "p2p_transfer", "compensated", time.Now(), "") // not counted
	seedWorkflowInstance(t, rig.E, "p2p_transfer", "failed", time.Now(), "")      // not counted

	after := getSummary(t, rig, tok)
	if after.NeedsManualApprovals != before.NeedsManualApprovals+1 {
		t.Errorf("needs_manual_approvals %d -> %d, want +1", before.NeedsManualApprovals, after.NeedsManualApprovals)
	}
	if after.CompensationFailedWorkflows != before.CompensationFailedWorkflows+1 {
		t.Errorf("compensation_failed_workflows %d -> %d, want +1", before.CompensationFailedWorkflows, after.CompensationFailedWorkflows)
	}
	if after.Total != after.NeedsManualApprovals+after.CompensationFailedWorkflows {
		t.Errorf("total %d != sum of parts", after.Total)
	}
}

// TC-G12-007: the summary is read-only and permission-gated like the lists it summarises.
func TestAttention_SummaryPermission(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	for _, role := range []string{"viewer", "kyc_reviewer"} {
		_, tok := rig.NewAdmin(role, "active")
		if w := rig.Call("GET", "/v1/admin/attention/summary", tok, nil); w.Code != http.StatusForbidden {
			t.Errorf("%s: got %d, want 403", role, w.Code)
		}
		if w := rig.Call("GET", "/v1/admin/approvals?status=needs_manual", tok, nil); w.Code != http.StatusForbidden {
			t.Errorf("%s needs_manual list: got %d, want 403", role, w.Code)
		}
	}
	if w := rig.Call("GET", "/v1/admin/attention/summary", "", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d, want 401", w.Code)
	}
	for _, role := range []string{"risk_officer", "ops", "superadmin"} {
		_, tok := rig.NewAdmin(role, "active")
		if w := rig.Call("GET", "/v1/admin/attention/summary", tok, nil); w.Code != http.StatusOK {
			t.Errorf("%s: got %d, want 200", role, w.Code)
		}
	}
	// no write verb exists on the summary
	_, tok := rig.NewAdmin("superadmin", "active")
	if w := rig.Call("POST", "/v1/admin/attention/summary", tok, map[string]any{}); w.Code == http.StatusOK {
		t.Errorf("POST summary must not succeed")
	}
}

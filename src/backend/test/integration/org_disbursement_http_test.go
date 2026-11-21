package integration

// f6-org-disbursement-endpoint: HTTP-layer coverage for
// POST /v1/org/{id}/transactions. The service method (org.Service.
// CreateDisbursement) already has thorough coverage in approval_test.go —
// these tests only prove the handler + route + middleware wiring.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/api"
	"github.com/hieumai1507/hpay-backend/internal/approval"
)

func orgDisburseRouter(t *testing.T, e *env, asUser uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := api.NewOrgHandler(e.osvc)
	auth := func(c *gin.Context) { c.Set("user_id", asUser.String()); c.Next() }
	r.POST("/v1/org/:id/transactions", auth, api.RequireIdempotencyKey(), h.Disburse)
	return r
}

func postDisburse(r *gin.Engine, orgID uuid.UUID, body map[string]any, idemKey string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/org/"+orgID.String()+"/transactions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestOrgDisbursementHTTP_Success(t *testing.T) {
	e := newEnv(t, "")
	f := newOrgFixture(t, e, 10_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})
	r := orgDisburseRouter(t, e, f.makerID)

	w := postDisburse(r, f.orgID, map[string]any{
		"phone": f.receiverPhone, "amount": 1_000_000, "note": "http test",
	}, uuid.NewString())

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body)
	}
	var body struct {
		RefCode   string `json:"ref_code"`
		Status    string `json:"status"`
		Amount    int64  `json:"amount"`
		Fee       int64  `json:"fee"`
		Total     int64  `json:"total"`
		CreatedAt string `json:"created_at"`
		Duplicate bool   `json:"duplicate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.RefCode == "" {
		t.Fatal("ref_code must not be empty")
	}
	if body.Amount != 1_000_000 {
		t.Fatalf("amount: want 1000000, got %d", body.Amount)
	}
	if body.Total != body.Amount+body.Fee {
		t.Fatalf("total: want amount+fee (%d), got %d", body.Amount+body.Fee, body.Total)
	}
	if body.CreatedAt == "" {
		t.Fatal("created_at must not be empty")
	}
	if body.Duplicate {
		t.Fatal("first request must not be flagged duplicate")
	}
}

func TestOrgDisbursementHTTP_MissingIdempotencyKey(t *testing.T) {
	e := newEnv(t, "")
	f := newOrgFixture(t, e, 10_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})
	r := orgDisburseRouter(t, e, f.makerID)

	w := postDisburse(r, f.orgID, map[string]any{
		"phone": f.receiverPhone, "amount": 1_000_000,
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", w.Code, w.Body)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "ERR_IDEMPOTENCY_KEY_REQUIRED" {
		t.Fatalf("code: want ERR_IDEMPOTENCY_KEY_REQUIRED, got %q", body.Code)
	}
}

func TestOrgDisbursementHTTP_ForbiddenRole(t *testing.T) {
	e := newEnv(t, "")
	f := newOrgFixture(t, e, 10_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})
	// Checker, not maker, tries to create a disbursement.
	r := orgDisburseRouter(t, e, f.checkers[0])

	w := postDisburse(r, f.orgID, map[string]any{
		"phone": f.receiverPhone, "amount": 1_000_000,
	}, uuid.NewString())

	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d (%s)", w.Code, w.Body)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "ERR_FORBIDDEN_ROLE" {
		t.Fatalf("code: want ERR_FORBIDDEN_ROLE, got %q", body.Code)
	}
}

func TestOrgDisbursementHTTP_IdempotentReplay(t *testing.T) {
	e := newEnv(t, "")
	f := newOrgFixture(t, e, 10_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})
	r := orgDisburseRouter(t, e, f.makerID)
	key := uuid.NewString()
	body := map[string]any{"phone": f.receiverPhone, "amount": 1_000_000, "note": "replay"}

	first := postDisburse(r, f.orgID, body, key)
	if first.Code != http.StatusCreated {
		t.Fatalf("first: want 201, got %d (%s)", first.Code, first.Body)
	}
	var firstBody struct {
		RefCode string `json:"ref_code"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &firstBody); err != nil {
		t.Fatal(err)
	}

	second := postDisburse(r, f.orgID, body, key)
	if second.Code != http.StatusOK {
		t.Fatalf("replay: want 200, got %d (%s)", second.Code, second.Body)
	}
	var secondBody struct {
		RefCode   string `json:"ref_code"`
		Duplicate bool   `json:"duplicate"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &secondBody); err != nil {
		t.Fatal(err)
	}
	if !secondBody.Duplicate {
		t.Fatal("replay must be flagged duplicate")
	}
	if secondBody.RefCode != firstBody.RefCode {
		t.Fatalf("replay ref_code: want %q, got %q", firstBody.RefCode, secondBody.RefCode)
	}
}

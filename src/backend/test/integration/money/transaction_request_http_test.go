package money

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/api"
	"github.com/hieumai1507/hpay-backend/internal/platform/config"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func transactionRequestRouter(t *testing.T, e *testkit.Env, asUser uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := api.NewTransactionEngineHandler(e.Svc, config.Config{Env: config.EnvCI})
	auth := func(c *gin.Context) { c.Set("user_id", asUser.String()); c.Next() }
	r.POST("/v1/transactions/request", auth, api.RequireIdempotencyKey(), h.RequestTransaction)
	return r
}

func postTransactionRequest(r *gin.Engine, body map[string]any, idemKey string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/transactions/request", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if idemKey != "" {
		req.Header.Set("Idempotency-Key", idemKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// A client that follows api/openapi.yaml to the letter — Idempotency-Key
// header only, nothing extra in the body — must be deduped on retry. Before
// the fix, the handler ignored the header-derived context value and read a
// body field the spec never documents, so this exact client was unprotected
// against a double-submit (e.g. a mobile retry after a dropped response).
func TestTransactionRequestHTTP_IdempotencyKeyHeaderDedupes(t *testing.T) {
	e := testkit.NewEnv(t, "")
	// Topup (not a P2P-type fund helper) so this setup doesn't itself trip
	// the P2P_TRANSFER interval throttle for the sender right before the
	// real transfer under test.
	senderID, _, _ := e.UserWithMoney(t, 4_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)

	r := transactionRequestRouter(t, e, senderID)
	idemKey := uuid.NewString()
	body := map[string]any{
		"service_code":   "P2P_TRANSFER",
		"amount":         400,
		"receiver_phone": receiverPhone,
	}

	w1 := postTransactionRequest(r, body, idemKey)
	if w1.Code != http.StatusOK {
		t.Fatalf("first request: want 200, got %d (%s)", w1.Code, w1.Body)
	}
	var res1 struct {
		RefCode       string `json:"ref_code"`
		TransactionID string `json:"transaction_id"`
	}
	if err := json.Unmarshal(w1.Body.Bytes(), &res1); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if res1.RefCode == "" {
		t.Fatalf("first response missing ref_code: %s", w1.Body)
	}

	// Simulate a client retry (e.g. mobile resending after a dropped
	// response) — same header, no body field for idempotency at all.
	// Sleep past OFFER_CUST_L2's real 3s P2P_TRANSFER interval throttle
	// (offerguard runs before the idempotency dedup lookup) so this retry
	// exercises the dedup path itself, not the unrelated velocity guard.
	time.Sleep(3100 * time.Millisecond)
	w2 := postTransactionRequest(r, body, idemKey)
	if w2.Code != http.StatusOK {
		t.Fatalf("retry: want 200, got %d (%s)", w2.Code, w2.Body)
	}
	var res2 struct {
		RefCode       string `json:"ref_code"`
		TransactionID string `json:"transaction_id"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &res2); err != nil {
		t.Fatalf("decode retry response: %v", err)
	}
	if res2.RefCode != res1.RefCode {
		t.Fatalf("retry must dedupe to the same ref_code: first=%s retry=%s", res1.RefCode, res2.RefCode)
	}
}

// Missing header entirely must still be rejected by the middleware — the
// fix must not accidentally make idempotency optional.
func TestTransactionRequestHTTP_MissingIdempotencyKeyRejected(t *testing.T) {
	e := testkit.NewEnv(t, "")
	senderID, _, _ := e.UserWithMoney(t, 4_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)

	r := transactionRequestRouter(t, e, senderID)
	body := map[string]any{
		"service_code":   "P2P_TRANSFER",
		"amount":         200,
		"receiver_phone": receiverPhone,
	}

	w := postTransactionRequest(r, body, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("no Idempotency-Key: want 400, got %d (%s)", w.Code, w.Body)
	}
}

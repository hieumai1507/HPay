package money

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/api"
	"github.com/hieumai1507/hpay-backend/internal/user"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// "Test User" (testutil fixture name) masked per the locked format.
const maskedFixtureName = "Test U***"

func TestHistory_CounterpartyMaskedAndDirection(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, _, senderPhone := e.UserWithMoney(t, 2_000_000)
	receiverID, _, receiverPhone := e.UserWithMoney(t, 0)

	if _, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 500_000, "m3", uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	e.Drain(t)

	now, maxID := time.Now().Add(time.Minute), uuid.Max

	// Sender view: p2p is OUT with the receiver's masked identity; topup has no counterparty.
	rows, err := e.Wsvc.History(ctx, senderID, now, maxID, 20)
	if err != nil {
		t.Fatal(err)
	}
	var sawP2P, sawTopup bool
	for _, r := range rows {
		switch r.Type {
		case "p2p":
			sawP2P = true
			if r.Direction != "out" {
				t.Errorf("sender p2p direction: want out, got %s", r.Direction)
			}
			if r.CounterpartyName == nil || *r.CounterpartyName != maskedFixtureName {
				t.Errorf("counterparty_name: want %q, got %v", maskedFixtureName, r.CounterpartyName)
			}
			wantPhone := user.MaskPhone(receiverPhone)
			if r.CounterpartyPhone == nil || *r.CounterpartyPhone != wantPhone {
				t.Errorf("counterparty_phone: want %q, got %v", wantPhone, r.CounterpartyPhone)
			}
		case "topup":
			sawTopup = true
			if r.CounterpartyName != nil || r.CounterpartyPhone != nil {
				t.Errorf("topup must have nil counterparty (system wallet), got %v/%v", r.CounterpartyName, r.CounterpartyPhone)
			}
		}
	}
	if !sawP2P || !sawTopup {
		t.Fatalf("expected both p2p and topup rows, got p2p=%v topup=%v", sawP2P, sawTopup)
	}

	// Receiver view: same txn is IN with the sender's masked identity.
	rows, err = e.Wsvc.History(ctx, receiverID, now, maxID, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.Type != "p2p" {
			continue
		}
		found = true
		if r.Direction != "in" {
			t.Errorf("receiver p2p direction: want in, got %s", r.Direction)
		}
		wantPhone := user.MaskPhone(senderPhone)
		if r.CounterpartyPhone == nil || *r.CounterpartyPhone != wantPhone {
			t.Errorf("receiver counterparty_phone: want %q, got %v", wantPhone, r.CounterpartyPhone)
		}
	}
	if !found {
		t.Fatal("receiver history missing the p2p txn")
	}
}

func TestDetail_CounterpartyMasked(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, _, _ := e.UserWithMoney(t, 1_000_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 200_000, "detail", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)

	v, err := e.Wsvc.Detail(ctx, senderID, res.RefCode)
	if err != nil {
		t.Fatal(err)
	}
	if v.Direction != "out" || v.CounterpartyName == nil || *v.CounterpartyName != maskedFixtureName {
		t.Fatalf("detail: direction=%s counterparty=%v, want out/%q", v.Direction, v.CounterpartyName, maskedFixtureName)
	}
}

// TestTimeline_FailureCode: locked receiver fails MOVE_FUNDS ("... not active
// at posting time") → public timeline carries ERR_ACCOUNT_INACTIVE, never the
// raw internal error text.
func TestTimeline_FailureCode(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	senderID, _, _ := e.UserWithMoney(t, 2_000_000)
	receiverID, _, receiverPhone := e.UserWithMoney(t, 0)

	res, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 300_000, "failure code", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE users SET status='locked' WHERE id=$1`, receiverID); err != nil {
		t.Fatal(err)
	}
	e.Drain(t)

	var txnID uuid.UUID
	if err := e.Pool.QueryRow(ctx, `SELECT id FROM transactions WHERE ref_code=$1`, res.RefCode).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	tl, err := e.Svc.TimelineFor(ctx, txnID)
	if err != nil {
		t.Fatal(err)
	}
	if tl.InstanceStatus != "failed" && tl.InstanceStatus != "compensated" {
		t.Fatalf("instance: want failed/compensated, got %s", tl.InstanceStatus)
	}
	if tl.FailureCode != "ERR_ACCOUNT_INACTIVE" {
		t.Fatalf("failure_code: want ERR_ACCOUNT_INACTIVE, got %q", tl.FailureCode)
	}

	// Completed transfers must NOT carry a failure code.
	res2, err := e.Svc.Transfer(ctx, senderID, receiverPhoneActive(t, e), 100_000, "ok", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	e.Drain(t)
	if err := e.Pool.QueryRow(ctx, `SELECT id FROM transactions WHERE ref_code=$1`, res2.RefCode).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	tl2, err := e.Svc.TimelineFor(ctx, txnID)
	if err != nil {
		t.Fatal(err)
	}
	if tl2.FailureCode != "" {
		t.Fatalf("completed txn must have no failure_code, got %q", tl2.FailureCode)
	}
}

func receiverPhoneActive(t *testing.T, e *testkit.Env) string {
	t.Helper()
	_, _, phone := e.UserWithMoney(t, 0)
	return phone
}

// --- handler-level: /v1/users/lookup + /v1/users/me ------------------------

func userRouter(t *testing.T, e *testkit.Env, asUser uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := api.NewUserHandler(e.Pool)
	auth := func(c *gin.Context) { c.Set("user_id", asUser.String()); c.Next() }
	r.GET("/v1/users/lookup", auth, h.Lookup)
	r.GET("/v1/users/me", auth, h.Me)
	return r
}

func TestLookup_MaskedAndUniform404(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	callerID, _, callerPhone := e.UserWithMoney(t, 0)
	targetID, _, targetPhone := e.UserWithMoney(t, 0)
	r := userRouter(t, e, callerID)

	get := func(phone string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/users/lookup?phone="+phone, nil)
		r.ServeHTTP(w, req)
		return w
	}

	// found → masked name only
	w := get(targetPhone)
	if w.Code != http.StatusOK {
		t.Fatalf("lookup existing: want 200, got %d (%s)", w.Code, w.Body)
	}
	var body struct {
		Found      bool   `json:"found"`
		MaskedName string `json:"masked_name"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Found || body.MaskedName != maskedFixtureName {
		t.Fatalf("want found + %q, got %+v", maskedFixtureName, body)
	}

	// missing, locked, and self all answer the SAME 404 (anti-enumeration)
	if w := get("0900000000"); w.Code != http.StatusNotFound {
		t.Fatalf("missing user: want 404, got %d", w.Code)
	}
	if _, err := e.Pool.Exec(ctx, `UPDATE users SET status='locked' WHERE id=$1`, targetID); err != nil {
		t.Fatal(err)
	}
	if w := get(targetPhone); w.Code != http.StatusNotFound {
		t.Fatalf("locked user: want 404, got %d", w.Code)
	}
	if w := get(callerPhone); w.Code != http.StatusNotFound {
		t.Fatalf("self lookup: want 404, got %d", w.Code)
	}
}

// The per-tier txn_limit now comes from the user's offer: see TestUsersMe_ReportsOfferLimits in offer_limits_test.go.

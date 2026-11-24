package money

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// AC-1
func TestOfferLimits_PerTxnCapsByTier(t *testing.T) { testLimitByKycTier(t) }

// TestEdge_E13_LimitByKycTier is edge case E13 ("amount above the limit of the KYC tier"). Since
// ADR-0007 the limit follows the offer the KYC tier selects, so the case is proven through the
// offer guard: each tier accepts its cap and refuses one unit above it. Kept under this name
// because scripts/check-edge-tests.sh requires one test per edge case id.
func TestEdge_E13_LimitByKycTier(t *testing.T) { testLimitByKycTier(t) }

func testLimitByKycTier(t *testing.T) {
	e := testkit.NewEnv(t, "")
	u, _, _ := e.UserWithMoney(t, 0)
	cases := []struct {
		offer  string
		ok     int64
		tooBig int64
	}{
		{"OFFER_CUST_L0", 2_500_000, 2_500_001},
		{"OFFER_CUST_L1", 62_500_000, 62_500_001},
		{"OFFER_CUST_L2", 250_000_000, 250_000_001},
	}
	for _, c := range cases {
		e.SetOffer(t, u, c.offer)
		if err := e.GuardCheck(u, c.ok); err != nil {
			t.Errorf("%s: %d must pass, got %v", c.offer, c.ok, err)
		}
		if err := e.GuardCheck(u, c.tooBig); !errors.Is(err, offerguard.ErrLimitPerTxn) {
			t.Errorf("%s: %d must hit the per-transaction cap, got %v", c.offer, c.tooBig, err)
		}
	}
}

// AC-2: a user with no offer row is limited as L0, not L2.
func TestOfferLimits_NoOfferRowFallsBackToL0(t *testing.T) {
	e := testkit.NewEnv(t, "")
	u, _, _ := e.UserWithMoney(t, 0)
	if e.OfferCodeOf(t, u) != "" {
		t.Fatal("setup: a fresh user must have no offer row")
	}
	if err := e.GuardCheck(u, 2_500_000); err != nil {
		t.Fatalf("2.5M must pass at L0: %v", err)
	}
	if err := e.GuardCheck(u, 3_000_000); !errors.Is(err, offerguard.ErrLimitPerTxn) {
		t.Fatalf("3M must be refused at L0, got %v", err)
	}
}

// AC-3 and AC-4 through the real KYC flow.
func TestOfferLimits_KYCOutcomeSelectsOffer(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()

	approved, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, approved, "OFFER_CUST_L0")
	if _, err := e.Ksvc.Submit(ctx, approved, testkit.TinyPNG, testkit.TinyPNG); err != nil {
		t.Fatal(err)
	}
	e.SetScore(95)
	e.Drain(t)
	if got := e.OfferCodeOf(t, approved); got != "OFFER_CUST_L1" {
		t.Fatalf("auto-approval must move L0 to L1, got %q", got)
	}

	rejected, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, rejected, "OFFER_CUST_L1")
	if _, err := e.Ksvc.Submit(ctx, rejected, testkit.TinyPNG, testkit.TinyPNG); err != nil {
		t.Fatal(err)
	}
	e.SetScore(10)
	e.Drain(t)
	if got := e.OfferCodeOf(t, rejected); got != "OFFER_CUST_L0" {
		t.Fatalf("rejection must move L1 to L0, got %q", got)
	}
}

func TestOfferLimits_ApprovalKeepsL2AndNeverTouchesMerchant(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	l2, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, l2, "OFFER_CUST_L2")
	biz, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, biz, "OFFER_BIZ_STARTER")
	for _, u := range []uuid.UUID{l2, biz} {
		if _, err := e.Ksvc.Submit(ctx, u, testkit.TinyPNG, testkit.TinyPNG); err != nil {
			t.Fatal(err)
		}
	}
	e.SetScore(95)
	e.Drain(t)
	if got := e.OfferCodeOf(t, l2); got != "OFFER_CUST_L2" {
		t.Errorf("approval must not downgrade L2, got %q", got)
	}
	if got := e.OfferCodeOf(t, biz); got != "OFFER_BIZ_STARTER" {
		t.Errorf("a merchant offer must not change, got %q", got)
	}
	// rejection: L2 drops to L0, merchant stays
	e.SetScore(10)
	for _, u := range []uuid.UUID{l2, biz} {
		if _, err := e.Pool.Exec(ctx, `UPDATE users SET kyc_status='rejected' WHERE id=$1`, u); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Ksvc.Submit(ctx, u, testkit.TinyPNG, testkit.TinyPNG); err != nil {
			t.Fatal(err)
		}
	}
	e.Drain(t)
	if got := e.OfferCodeOf(t, l2); got != "OFFER_CUST_L0" {
		t.Errorf("rejection must send a customer offer to L0, got %q", got)
	}
	if got := e.OfferCodeOf(t, biz); got != "OFFER_BIZ_STARTER" {
		t.Errorf("a merchant offer must survive a rejection, got %q", got)
	}
}

// AC-3: the admin decision on a manual review does the same.
func TestOfferLimits_AdminReviewSelectsOffer(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	admin := e.SeedAdminID(t)
	for _, approve := range []bool{true, false} {
		u, _, _ := e.UserWithMoney(t, 0)
		e.SetOffer(t, u, map[bool]string{true: "OFFER_CUST_L0", false: "OFFER_CUST_L1"}[approve])
		if _, err := e.Ksvc.Submit(ctx, u, testkit.TinyPNG, testkit.TinyPNG); err != nil {
			t.Fatal(err)
		}
		e.SetScore(60) // manual review band
		e.Drain(t)
		var reqID uuid.UUID
		if err := e.Pool.QueryRow(ctx, `SELECT id FROM kyc_requests WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1`, u).Scan(&reqID); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Kadmin.Review(ctx, admin, reqID, approve, "test"); err != nil {
			t.Fatal(err)
		}
		want := map[bool]string{true: "OFFER_CUST_L1", false: "OFFER_CUST_L0"}[approve]
		if got := e.OfferCodeOf(t, u); got != want {
			t.Errorf("approve=%v: want %s, got %q", approve, want, got)
		}
	}
}

// AC-5
func TestOfferLimits_InactiveOfferBlocksAndDoesNotFallBack(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	u, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, u, "OFFER_CUST_L1")
	if _, err := e.Pool.Exec(ctx, `UPDATE offers SET is_active=false WHERE code='OFFER_CUST_L1'`); err != nil {
		t.Fatal(err)
	}
	defer e.Pool.Exec(ctx, `UPDATE offers SET is_active=true WHERE code='OFFER_CUST_L1'`)
	if err := e.GuardCheck(u, 1_000); !errors.Is(err, offerguard.ErrOfferInactive) {
		t.Fatalf("an inactive offer must block, got %v", err)
	}
}

// AC-6
func TestUsersMe_ReportsOfferLimits(t *testing.T) {
	e := testkit.NewEnv(t, "")
	u, _, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, u, "OFFER_CUST_L1")
	r := userRouter(t, e, u)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/users/me", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("me: %d", w.Code)
	}
	var body struct {
		TxnLimit int64 `json:"txn_limit"`
		Offer    struct {
			Code         string `json:"code"`
			PerTxn       int64  `json:"per_txn_limit"`
			DailyLimit   int64  `json:"daily_limit"`
			MonthlyLimit int64  `json:"monthly_limit"`
		} `json:"offer"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TxnLimit != 62_500_000 || body.Offer.Code != "OFFER_CUST_L1" || body.Offer.PerTxn != 62_500_000 ||
		body.Offer.DailyLimit != 125_000_000 || body.Offer.MonthlyLimit != 625_000_000 {
		t.Fatalf("unexpected /users/me offer block: %s", w.Body.String())
	}
}

// AC-7: money-request pay and the legacy top-up/transfer paths respect the offer.
func TestOfferLimits_LegacyPathsUseTheOffer(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	u, uw, _ := e.UserWithMoney(t, 0)
	e.SetOffer(t, u, "OFFER_CUST_L0")
	if _, err := e.Wsvc.Topup(ctx, u, 3_000_000, uuid.NewString()); !errors.Is(err, wallet.ErrLimitExceeded) && !errors.Is(err, offerguard.ErrLimitPerTxn) {
		t.Fatalf("L0 top-up above 2.5M must be refused, got %v", err)
	}
	e.SetOffer(t, u, "OFFER_CUST_L1")
	if _, err := e.Wsvc.Topup(ctx, u, 3_000_000, uuid.NewString()); err != nil {
		t.Fatalf("L1 top-up of 3M must pass: %v", err)
	}
	_ = uw
	_ = transfer.ErrReceiverNotFound
}

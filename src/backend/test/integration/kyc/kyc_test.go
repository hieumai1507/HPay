package kyc

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/kyc"
	"github.com/hieumai1507/hpay-backend/internal/listing"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// stubScorer returns a fixed score — tests drive each classification branch.
type stubScorer struct{ score int }

func (stubScorer) Name() string { return "stub" }

func (s stubScorer) Score(context.Context, []byte, []byte) (kyc.Result, error) {
	return kyc.Result{Score: s.score, Reasons: []string{"stubbed"}}, nil
}

// submitAndDrain runs one full KYC flow with the given stub score and returns
// (userID, kyc request status, users.kyc_status).
func submitAndDrain(t *testing.T, e *testkit.Env, score int) (uuid.UUID, store.KycStatus, store.KycStatus) {
	t.Helper()
	ctx := context.Background()
	userID, _, _ := e.UserWithMoney(t, 0)

	res, err := e.Ksvc.Submit(ctx, userID, testkit.TinyPNG, testkit.TinyPNG)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	// swap the scorer for this flow (registry handler holds a pointer we control)
	e.SetScore(score)
	e.Drain(t)

	req, err := e.Q.GetKYCRequest(ctx, res.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	u, err := e.Q.GetUserStatusByID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	return userID, req.Status, u.KycStatus
}

func TestKYCAutoApproveBranch(t *testing.T) {
	e := testkit.NewEnv(t, "")
	_, reqStatus, userStatus := submitAndDrain(t, e, 90)
	if reqStatus != store.KycStatusAutoApproved || userStatus != store.KycStatusAutoApproved {
		t.Fatalf("want auto_approved/auto_approved, got %s/%s", reqStatus, userStatus)
	}
}

func TestKYCManualThenAdminApprove(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	userID, reqStatus, userStatus := submitAndDrain(t, e, 60)
	if reqStatus != store.KycStatusManualReview || userStatus != store.KycStatusManualReview {
		t.Fatalf("want manual_review/manual_review, got %s/%s", reqStatus, userStatus)
	}

	// admin queue must contain the request
	items, _, err := e.Kadmin.Queue(ctx, "manual", listing.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var reqID uuid.UUID
	for _, it := range items {
		if it.UserID == userID {
			reqID = it.ID
		}
	}
	if reqID == uuid.Nil {
		t.Fatal("request not in manual queue")
	}

	adminID := e.SeedAdminID(t)
	if _, err := e.Kadmin.Review(ctx, adminID, reqID, true, "docs look fine"); err != nil {
		t.Fatalf("review: %v", err)
	}
	u, _ := e.Q.GetUserStatusByID(ctx, userID)
	if u.KycStatus != store.KycStatusVerified {
		t.Fatalf("user after approve: want verified, got %s", u.KycStatus)
	}

	// double review → 409-shaped error
	if _, err := e.Kadmin.Review(ctx, adminID, reqID, false, "changed my mind"); !testkit.ErrIs(err, kyc.ErrAlreadyReviewed) {
		t.Fatalf("double review: want ErrAlreadyReviewed, got %v", err)
	}
}

func TestKYCRejectBranchAllowsResubmit(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	userID, reqStatus, userStatus := submitAndDrain(t, e, 30)
	if reqStatus != store.KycStatusRejected || userStatus != store.KycStatusRejected {
		t.Fatalf("want rejected/rejected, got %s/%s", reqStatus, userStatus)
	}
	// rejected users may try again (re-KYC)
	if _, err := e.Ksvc.Submit(ctx, userID, testkit.TinyPNG, testkit.TinyPNG); err != nil {
		t.Fatalf("re-submit after reject: %v", err)
	}
}

func TestKYCSubmitGuards(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	userID, _, _ := e.UserWithMoney(t, 0)

	if _, err := e.Ksvc.Submit(ctx, userID, []byte("not an image"), testkit.TinyPNG); !testkit.ErrIs(err, kyc.ErrBadImage) {
		t.Fatalf("bad image: want ErrBadImage, got %v", err)
	}
	if _, err := e.Ksvc.Submit(ctx, userID, testkit.TinyPNG, testkit.TinyPNG); err != nil {
		t.Fatal(err)
	}
	// second submit while pending → blocked
	if _, err := e.Ksvc.Submit(ctx, userID, testkit.TinyPNG, testkit.TinyPNG); !testkit.ErrIs(err, kyc.ErrAlreadyPending) {
		t.Fatalf("double submit: want ErrAlreadyPending, got %v", err)
	}
}

// TestKYCReviewRace: 2 admins review the same request concurrently → guarded UPDATE
// lets exactly one win (basis for admin edge A01).
func TestKYCReviewRace(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	_, reqStatus, _ := submitAndDrain(t, e, 60)
	if reqStatus != store.KycStatusManualReview {
		t.Fatalf("setup: want manual_review, got %s", reqStatus)
	}
	items, _, err := e.Kadmin.Queue(ctx, "manual", listing.Filter{})
	if err != nil || len(items) == 0 {
		t.Fatalf("queue: %v (%d items)", err, len(items))
	}
	reqID := items[len(items)-1].ID
	adminID := e.SeedAdminID(t)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	actions := []bool{true, false} // one approves, one rejects
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.Kadmin.Review(ctx, adminID, reqID, actions[i], "race")
		}(i)
	}
	wg.Wait()

	var ok, conflict int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case testkit.ErrIs(err, kyc.ErrAlreadyReviewed):
			conflict++
		default:
			t.Fatalf("unexpected: %v", err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("want 1 winner + 1 conflict, got ok=%d conflict=%d", ok, conflict)
	}
}

// TestKYCTierUnlocksTransferLimit: connects F8 to F3 — auto_approved raises
// wallet.TxnLimit from 2M to 100M.
func TestKYCTierUnlocksTransferLimit(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	// Fund via direct ledger post — Topup itself is tier-limited (2M at none),
	// and this test needs a 20M balance before KYC.
	senderID, senderWallet, _ := e.UserWithMoney(t, 0)
	testkit.Fund(t, e.Pool, senderID, senderWallet, 80_000)
	_, _, receiverPhone := e.UserWithMoney(t, 0)

	// tier none: 10M > 2M limit → blocked
	if _, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 40_000, "", uuid.NewString()); err == nil {
		t.Fatal("want limit error at tier none")
	}

	if _, err := e.Ksvc.Submit(ctx, senderID, testkit.TinyPNG, testkit.TinyPNG); err != nil {
		t.Fatal(err)
	}
	e.SetScore(95)
	e.Drain(t)

	// tier auto_approved: same transfer passes
	if _, err := e.Svc.Transfer(ctx, senderID, receiverPhone, 40_000, "", uuid.NewString()); err != nil {
		t.Fatalf("transfer after kyc: %v", err)
	}
	e.Drain(t)
}

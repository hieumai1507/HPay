package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/kyc"
	"github.com/hieumai1507/hpay-backend/internal/listing"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// tinyPNG is a 1x1 transparent PNG — a real, valid image payload.
var tinyPNG = []byte{
	0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89,
	0x00, 0x00, 0x00, 0x0A, 'I', 'D', 'A', 'T',
	0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00, 0x01,
	0x0D, 0x0A, 0x2D, 0xB4,
	0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82,
}

// stubScorer returns a fixed score — tests drive each classification branch.
type stubScorer struct{ score int }

func (stubScorer) Name() string { return "stub" }
func (s stubScorer) Score(context.Context, []byte, []byte) (kyc.Result, error) {
	return kyc.Result{Score: s.score, Reasons: []string{"stubbed"}}, nil
}

// submitAndDrain runs one full KYC flow with the given stub score and returns
// (userID, kyc request status, users.kyc_status).
func submitAndDrain(t *testing.T, e *env, score int) (uuid.UUID, store.KycStatus, store.KycStatus) {
	t.Helper()
	ctx := context.Background()
	userID, _, _ := e.userWithMoney(t, 0)

	res, err := e.ksvc.Submit(ctx, userID, tinyPNG, tinyPNG)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	// swap the scorer for this flow (registry handler holds a pointer we control)
	e.setScore(score)
	e.drain(t)

	req, err := e.q.GetKYCRequest(ctx, res.RequestID)
	if err != nil {
		t.Fatal(err)
	}
	u, err := e.q.GetUserStatusByID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	return userID, req.Status, u.KycStatus
}

func TestKYCAutoApproveBranch(t *testing.T) {
	e := newEnv(t, "")
	_, reqStatus, userStatus := submitAndDrain(t, e, 90)
	if reqStatus != store.KycStatusAutoApproved || userStatus != store.KycStatusAutoApproved {
		t.Fatalf("want auto_approved/auto_approved, got %s/%s", reqStatus, userStatus)
	}
}

func TestKYCManualThenAdminApprove(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	userID, reqStatus, userStatus := submitAndDrain(t, e, 60)
	if reqStatus != store.KycStatusManualReview || userStatus != store.KycStatusManualReview {
		t.Fatalf("want manual_review/manual_review, got %s/%s", reqStatus, userStatus)
	}

	// admin queue must contain the request
	items, _, err := e.kadmin.Queue(ctx, "manual", listing.Filter{})
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

	adminID := e.seedAdminID(t)
	if _, err := e.kadmin.Review(ctx, adminID, reqID, true, "docs look fine"); err != nil {
		t.Fatalf("review: %v", err)
	}
	u, _ := e.q.GetUserStatusByID(ctx, userID)
	if u.KycStatus != store.KycStatusVerified {
		t.Fatalf("user after approve: want verified, got %s", u.KycStatus)
	}

	// double review → 409-shaped error
	if _, err := e.kadmin.Review(ctx, adminID, reqID, false, "changed my mind"); !errIs(err, kyc.ErrAlreadyReviewed) {
		t.Fatalf("double review: want ErrAlreadyReviewed, got %v", err)
	}
}

func TestKYCRejectBranchAllowsResubmit(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	userID, reqStatus, userStatus := submitAndDrain(t, e, 30)
	if reqStatus != store.KycStatusRejected || userStatus != store.KycStatusRejected {
		t.Fatalf("want rejected/rejected, got %s/%s", reqStatus, userStatus)
	}
	// rejected users may try again (re-KYC)
	if _, err := e.ksvc.Submit(ctx, userID, tinyPNG, tinyPNG); err != nil {
		t.Fatalf("re-submit after reject: %v", err)
	}
}

func TestKYCSubmitGuards(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	userID, _, _ := e.userWithMoney(t, 0)

	if _, err := e.ksvc.Submit(ctx, userID, []byte("not an image"), tinyPNG); !errIs(err, kyc.ErrBadImage) {
		t.Fatalf("bad image: want ErrBadImage, got %v", err)
	}
	if _, err := e.ksvc.Submit(ctx, userID, tinyPNG, tinyPNG); err != nil {
		t.Fatal(err)
	}
	// second submit while pending → blocked
	if _, err := e.ksvc.Submit(ctx, userID, tinyPNG, tinyPNG); !errIs(err, kyc.ErrAlreadyPending) {
		t.Fatalf("double submit: want ErrAlreadyPending, got %v", err)
	}
}

// TestKYCReviewRace: 2 admins review the same request concurrently → guarded UPDATE
// lets exactly one win (basis for admin edge A01).
func TestKYCReviewRace(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	_, reqStatus, _ := submitAndDrain(t, e, 60)
	if reqStatus != store.KycStatusManualReview {
		t.Fatalf("setup: want manual_review, got %s", reqStatus)
	}
	items, _, err := e.kadmin.Queue(ctx, "manual", listing.Filter{})
	if err != nil || len(items) == 0 {
		t.Fatalf("queue: %v (%d items)", err, len(items))
	}
	reqID := items[len(items)-1].ID
	adminID := e.seedAdminID(t)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	actions := []bool{true, false} // one approves, one rejects
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.kadmin.Review(ctx, adminID, reqID, actions[i], "race")
		}(i)
	}
	wg.Wait()

	var ok, conflict int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errIs(err, kyc.ErrAlreadyReviewed):
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
	e := newEnv(t, "")
	ctx := context.Background()
	// Fund via direct ledger post — Topup itself is tier-limited (2M at none),
	// and this test needs a 20M balance before KYC.
	senderID, senderWallet, _ := e.userWithMoney(t, 0)
	fund(t, e.pool, senderID, senderWallet, 20_000_000)
	_, _, receiverPhone := e.userWithMoney(t, 0)

	// tier none: 10M > 2M limit → blocked
	if _, err := e.svc.Transfer(ctx, senderID, receiverPhone, 10_000_000, "", uuid.NewString()); err == nil {
		t.Fatal("want limit error at tier none")
	}

	if _, err := e.ksvc.Submit(ctx, senderID, tinyPNG, tinyPNG); err != nil {
		t.Fatal(err)
	}
	e.setScore(95)
	e.drain(t)

	// tier auto_approved: same transfer passes
	if _, err := e.svc.Transfer(ctx, senderID, receiverPhone, 10_000_000, "", uuid.NewString()); err != nil {
		t.Fatalf("transfer after kyc: %v", err)
	}
	e.drain(t)
}

package fraud

// G15 / ADR-0009 / C-13: the LLM advisory can never change a fraud decision. Whatever the
// advisor does (fails, returns nothing, hangs, is slow), the rule verdict stands, the flag
// keeps its rule evidence, and the step does not wait for the advisor.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/ai"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

type stubAdvisor struct {
	text    string
	err     error
	block   chan struct{} // when set, Complete waits for it or for the context
	started chan struct{}
}

func (stubAdvisor) Name() string { return "stub" }
func (s stubAdvisor) Complete(ctx context.Context, _, _ string) (string, error) {
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return s.text, s.err
}

var _ ai.Advisor = stubAdvisor{}

// fraudTxn builds a transaction the rules flag (velocity + new receiver = grey zone) or block.
func fraudTxn(t *testing.T, amount int64, history int, histAmount int64) (*handler, uuid.UUID, uuid.UUID) {
	t.Helper()
	pool := testutil.PoolFromEnv(t)
	q := store.New(pool)
	ctx := context.Background()
	sender, senderWallet := testutil.NewUserWithWallet(t, pool)
	_, receiverWallet := testutil.NewUserWithWallet(t, pool)
	_, histWallet := testutil.NewUserWithWallet(t, pool)
	for i := 0; i < history; i++ {
		if _, err := pool.Exec(ctx, `INSERT INTO transactions (ref_code, idempotency_key, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by, completed_at)
			VALUES ($1,$2,'p2p','completed',$3,$4,$5,0,$6,now())`, "MPH"+uuid.NewString()[:10], uuid.NewString(), senderWallet, histWallet, histAmount, sender); err != nil {
			t.Fatal(err)
		}
	}
	var txnID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO transactions (ref_code, idempotency_key, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by)
		VALUES ($1,$2,'p2p','processing',$3,$4,$5,0,$6) RETURNING id`, "MPF"+uuid.NewString()[:10], uuid.NewString(), senderWallet, receiverWallet, amount, sender).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	h := &handler{pool: pool, q: q, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return h, txnID, sender
}

func flagReasons(t *testing.T, h *handler, txnID uuid.UUID) (string, bool) {
	t.Helper()
	var reasons string
	err := h.pool.QueryRow(context.Background(), `SELECT reasons::text FROM fraud_flags WHERE transaction_id=$1`, txnID).Scan(&reasons)
	if err != nil {
		return "", false
	}
	return reasons, true
}

func runStep(h *handler, txnID uuid.UUID) (workflow.StepOutput, error) {
	return h.Execute(context.Background(), workflow.StepInput{TransactionID: txnID})
}

// The decision is the same with every kind of advisor failure.
func TestAdvisoryOutage_NeverChangesTheDecision(t *testing.T) {
	cases := map[string]stubAdvisor{
		"provider error": {err: errors.New("upstream 503")},
		"empty text":     {text: ""},
		"timeout":        {err: context.DeadlineExceeded},
		"works":          {text: "looks risky"},
	}
	for name, adv := range cases {
		t.Run(name, func(t *testing.T) {
			// grey zone: velocity 30 + new receiver 20 = 50 -> flagged, money proceeds
			h, txnID, _ := fraudTxn(t, 200_000, 4, 200_000)
			done := make(chan struct{})
			h.advisorOverride, h.advisoryDone = adv, func() { close(done) }
			out, err := runStep(h, txnID)
			if err != nil {
				t.Fatalf("a flagged transaction must pass the step, got %v", err)
			}
			if out.Context["fraud_verdict"] != "flagged" {
				t.Fatalf("verdict = %v, want flagged", out.Context["fraud_verdict"])
			}
			<-done
			reasons, ok := flagReasons(t, h, txnID)
			if !ok {
				t.Fatal("the flag must exist whatever the advisor does")
			}
			var parsed struct {
				Signals []map[string]any `json:"signals"`
			}
			if err := json.Unmarshal([]byte(reasons), &parsed); err != nil || len(parsed.Signals) == 0 {
				t.Fatalf("the rule evidence must survive an advisor failure: %s", reasons)
			}
		})
	}
}

// A blocked transaction stays blocked when the advisor is down.
func TestAdvisoryOutage_BlockedStaysBlocked(t *testing.T) {
	// velocity + spike + new receiver >= 85
	h, txnID, _ := fraudTxn(t, 20_000_000, 4, 100_000)
	done := make(chan struct{})
	h.advisorOverride, h.advisoryDone = stubAdvisor{err: errors.New("down")}, func() { close(done) }
	if _, err := runStep(h, txnID); !errors.Is(err, ErrBlocked) {
		t.Fatalf("a high-risk transaction must still be blocked, got %v", err)
	}
	<-done
	if _, ok := flagReasons(t, h, txnID); !ok {
		t.Fatal("the flag must exist")
	}
}

// The step returns while the advisor is still hanging: the payment does not wait for the LLM.
func TestAdvisoryOutage_StepDoesNotWaitForAdvisor(t *testing.T) {
	h, txnID, _ := fraudTxn(t, 200_000, 4, 200_000)
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	done := make(chan struct{})
	h.advisorOverride = stubAdvisor{text: "late", block: release, started: started}
	h.advisoryDone = func() { close(done) }

	stepDone := make(chan error, 1)
	go func() { _, err := runStep(h, txnID); stepDone <- err }()
	select {
	case err := <-stepDone:
		if err != nil {
			t.Fatalf("step: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the fraud step is waiting for the advisor")
	}
	<-started // the advisor really is running (and blocked) while the step has already returned
	close(release)
	<-done
}

// A rule-side failure is retryable (the step fails closed, nothing passes silently).
func TestRuleSideFailureIsRetryableNotAPass(t *testing.T) {
	h, _, _ := fraudTxn(t, 200_000, 0, 0)
	_, err := runStep(h, uuid.New()) // transaction does not exist
	var re workflow.RetryableError
	if !errors.As(err, &re) {
		t.Fatalf("a failed rule input read must be retryable, got %v", err)
	}
}

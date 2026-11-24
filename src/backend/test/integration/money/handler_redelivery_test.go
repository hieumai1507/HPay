package money

// G19: every registered workflow step handler is run twice with the SAME StepInput (same
// instance, step and attempt). That is exactly what a redelivered start message does when
// the first run crashed after its side effect but before recordAndEmit committed the step
// record. The second run must not add side effects and must not fail.
//
// The table below is the guard: a handler registered without a scenario fails
// TestHandlerRedelivery_EveryRegisteredHandlerHasAScenario, so a new handler cannot ship
// without declaring how it behaves on redelivery (docs/domain/outbox-and-events.md).

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/workflow"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

type redeliveryFixture struct {
	in       workflow.StepInput
	snapshot func(t *testing.T) string // every side effect the handler can leave behind
}

type redeliveryScenario struct {
	handler string
	setup   func(t *testing.T, e *testkit.Env) redeliveryFixture
}

// processingTxn inserts a funded sender, a receiver and a 'processing' p2p transaction.
func processingTxn(t *testing.T, e *testkit.Env, amount int64) (txnID, sender, senderWallet, receiverWallet uuid.UUID, refCode string) {
	t.Helper()
	sender, senderWallet, _ = e.UserWithMoney(t, 8_000)
	_, receiverWallet, _ = e.UserWithMoney(t, 0)
	refCode = "G19" + uuid.NewString()[:10]
	if err := e.Pool.QueryRow(context.Background(), `
		INSERT INTO transactions (ref_code, idempotency_key, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by)
		VALUES ($1,$2,'p2p','processing',$3,$4,$5,0,$6) RETURNING id`,
		refCode, uuid.NewString(), senderWallet, receiverWallet, amount, sender).Scan(&txnID); err != nil {
		t.Fatal(err)
	}
	return
}

func scalar(t *testing.T, e *testkit.Env, sql string, args ...any) string {
	t.Helper()
	var s string
	if err := e.Pool.QueryRow(context.Background(), sql, args...).Scan(&s); err != nil {
		t.Fatalf("snapshot query: %v", err)
	}
	return s
}

func txnSnapshot(e *testkit.Env, txnID, senderWallet, receiverWallet uuid.UUID) func(t *testing.T) string {
	return func(t *testing.T) string {
		return strings.Join([]string{
			"ledger=" + scalar(t, e, `SELECT count(*)::text FROM ledger_entries WHERE transaction_id=$1`, txnID),
			"flags=" + scalar(t, e, `SELECT count(*)::text FROM fraud_flags WHERE transaction_id=$1`, txnID),
			"status=" + scalar(t, e, `SELECT status::text FROM transactions WHERE id=$1`, txnID),
			"sender=" + scalar(t, e, `SELECT balance::text FROM wallets WHERE id=$1`, senderWallet),
			"receiver=" + scalar(t, e, `SELECT balance::text FROM wallets WHERE id=$1`, receiverWallet),
		}, " ")
	}
}

func baseInput(txnID uuid.UUID, step string, ctxMap map[string]any) workflow.StepInput {
	return workflow.StepInput{
		InstanceID: uuid.New(), TransactionID: txnID,
		Step: workflow.StepDef{Name: step}, Attempt: 1, Context: ctxMap,
	}
}

func redeliveryScenarios() []redeliveryScenario {
	simpleTxn := func(step string) func(t *testing.T, e *testkit.Env) redeliveryFixture {
		return func(t *testing.T, e *testkit.Env) redeliveryFixture {
			txnID, _, sw, rw, _ := processingTxn(t, e, 492)
			return redeliveryFixture{in: baseInput(txnID, step, nil), snapshot: txnSnapshot(e, txnID, sw, rw)}
		}
	}
	return []redeliveryScenario{
		{"validator", simpleTxn("VALIDATE")},
		{"wallet", simpleTxn("MOVE_FUNDS")},
		{"confirmer", simpleTxn("CONFIRM")},
		{"fraud", func(t *testing.T, e *testkit.Env) redeliveryFixture {
			// velocity 30 + new receiver 20 = 50: the grey zone, a flag is written and money proceeds.
			txnID, sender, sw, rw, _ := processingTxn(t, e, 800)
			_, histWallet, _ := e.UserWithMoney(t, 0)
			insertP2PHistory(t, e, sender, sw, histWallet, 4, 800)
			return redeliveryFixture{in: baseInput(txnID, "FRAUD_CHECK", nil), snapshot: txnSnapshot(e, txnID, sw, rw)}
		}},
		{"round_up", func(t *testing.T, e *testkit.Env) redeliveryFixture {
			txnID, sender, sw, rw, _ := processingTxn(t, e, 492)
			vaultID := newVault(t, e, sender, "G19 round-up")
			setRoundUpTarget(t, e, sender, vaultID, true)
			snap := txnSnapshot(e, txnID, sw, rw)
			return redeliveryFixture{in: baseInput(txnID, "ROUND_UP", nil), snapshot: func(t *testing.T) string {
				return snap(t) + " vault=" + scalar(t, e, `SELECT balance::text FROM wallets WHERE id=$1`, vaultID)
			}}
		}},
		{"action_pipeline", func(t *testing.T, e *testkit.Env) redeliveryFixture {
			txnID, _, sw, rw, ref := processingTxn(t, e, 492)
			code := "SVC_G19_" + strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
			if _, err := e.Pool.Exec(context.Background(), `
				INSERT INTO services (code, name, category, status, action_priority, actions)
				VALUES ($1,'G19','transfer','active','before_gl',$2::jsonb)`, code,
				`[{"code":"bank_notify","name":"Bank","type":"Directional Integration","entry_point":"https://partner.test/hook","method":"POST","required":true}]`); err != nil {
				t.Fatal(err)
			}
			e.Gw.Reset()
			snap := txnSnapshot(e, txnID, sw, rw)
			return redeliveryFixture{
				in: baseInput(txnID, "ACTIONS_BEFORE_GL", map[string]any{"service_code": code, "ref_code": ref}),
				snapshot: func(t *testing.T) string {
					// The partner call is re-sent on redelivery by design; what must hold is that
					// every send carries the same idempotency key so the partner dedupes it.
					keys := map[string]bool{}
					for _, c := range e.Gw.Calls() {
						keys[c.IdempotencyRef] = true
					}
					var ks []string
					for k := range keys {
						ks = append(ks, k)
					}
					sort.Strings(ks)
					return snap(t) + " keys=" + strings.Join(ks, ",")
				}}
		}},
		{"kyc_scorer", func(t *testing.T, e *testkit.Env) redeliveryFixture {
			reqID, userID := kycRequest(t, e)
			return redeliveryFixture{in: baseInput(uuid.Nil, "KYC_SCORE", map[string]any{"kyc_request_id": reqID.String()}),
				snapshot: kycSnapshot(e, reqID, userID)}
		}},
		{"kyc_classifier", func(t *testing.T, e *testkit.Env) redeliveryFixture {
			reqID, userID := kycRequest(t, e)
			if _, err := e.Pool.Exec(context.Background(),
				`UPDATE kyc_requests SET ai_score=90, ai_verdict='pass' WHERE id=$1`, reqID); err != nil {
				t.Fatal(err)
			}
			return redeliveryFixture{in: baseInput(uuid.Nil, "KYC_CLASSIFY", map[string]any{"kyc_request_id": reqID.String()}),
				snapshot: kycSnapshot(e, reqID, userID)}
		}},
	}
}

func kycRequest(t *testing.T, e *testkit.Env) (reqID, userID uuid.UUID) {
	t.Helper()
	userID, _, _ = e.UserWithMoney(t, 0)
	if _, err := e.Pool.Exec(context.Background(), `UPDATE users SET kyc_status='pending' WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	if err := e.Pool.QueryRow(context.Background(),
		`INSERT INTO kyc_requests (user_id, id_card_url, selfie_url) VALUES ($1,'g19/id.png','g19/selfie.png') RETURNING id`,
		userID).Scan(&reqID); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"g19/id.png", "g19/selfie.png"} {
		if err := e.Files.Put(context.Background(), k, testkit.TinyPNG); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func kycSnapshot(e *testkit.Env, reqID, userID uuid.UUID) func(t *testing.T) string {
	return func(t *testing.T) string {
		return strings.Join([]string{
			"req=" + scalar(t, e, `SELECT COALESCE(ai_score::text,'-')||'/'||status::text FROM kyc_requests WHERE id=$1`, reqID),
			"user=" + scalar(t, e, `SELECT kyc_status::text FROM users WHERE id=$1`, userID),
			"offers=" + scalar(t, e, `SELECT count(*)::text FROM user_offers WHERE user_id=$1`, userID),
		}, " ")
	}
}

func TestHandlerRedelivery_RunningTwiceAddsNoSideEffects(t *testing.T) {
	for _, sc := range redeliveryScenarios() {
		sc := sc
		t.Run(sc.handler, func(t *testing.T) {
			e := testkit.NewEnv(t, "")
			h, ok := e.Reg.Get(sc.handler)
			if !ok {
				t.Fatalf("handler %q is not registered", sc.handler)
			}
			fx := sc.setup(t, e)
			ctx := context.Background()

			out1, err := h.Execute(ctx, fx.in)
			if err != nil {
				t.Fatalf("first run: %v", err)
			}
			after1 := fx.snapshot(t)

			out2, err := h.Execute(ctx, fx.in) // the redelivered message
			if err != nil {
				t.Fatalf("redelivered run must succeed like the first (out1=%v): %v", out1, err)
			}
			if after2 := fx.snapshot(t); after2 != after1 {
				t.Fatalf("redelivery changed side effects\n after run 1: %s\n after run 2: %s\n out2=%v", after1, after2, out2)
			}
		})
	}
}

func TestHandlerRedelivery_EveryRegisteredHandlerHasAScenario(t *testing.T) {
	e := testkit.NewEnv(t, "")
	have := map[string]bool{"boom": true} // boom is a test-only failing handler
	for _, sc := range redeliveryScenarios() {
		have[sc.handler] = true
	}
	for _, name := range e.Reg.Names() {
		if !have[name] {
			t.Errorf("handler %q is registered but has no redelivery scenario in handler_redelivery_test.go", name)
		}
	}
}

// A redelivery that races the original run (XAUTOCLAIM of a slow message) must wait for it
// instead of writing a second flag. Deterministic: this test plays the original run by
// holding the per-transaction lock itself, and the handler must not get past it.
func TestHandlerRedelivery_FraudRunWaitsForTheRunHoldingTheTransactionLock(t *testing.T) {
	e := testkit.NewEnv(t, "")
	h, _ := e.Reg.Get("fraud")
	fx := redeliveryScenarios()[3].setup(t, e)
	ctx := context.Background()

	holder, err := e.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if _, err := holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('fraud_flag:' || $1::text, 0))`, fx.in.TransactionID); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { _, err := h.Execute(ctx, fx.in); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("the redelivered run must wait for the lock holder, but it finished (err=%v)", err)
	case <-time.After(500 * time.Millisecond):
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("run after the lock was released: %v", err)
	}
	if got := scalar(t, e, `SELECT count(*)::text FROM fraud_flags WHERE transaction_id=$1`, fx.in.TransactionID); got != "1" {
		t.Fatalf("fraud flags = %s, want 1", got)
	}
}

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/hieumai1507/hpay-backend/internal/transfer"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

// Action Pipeline wiring (docs plan action-pipeline-wiring): ACTIONS_BEFORE_GL
// / ACTIONS_AFTER_GL now exist in p2p_transfer v3 / org_disbursement v2. Only
// P2P_TRANSFER has a non-empty services.actions in the seed data (one
// Directional Integration action, action_priority=before_gl); every other
// seeded service has actions=[] and must stay a no-op.

func TestActionPipelineWiring_P2PCallsGateway(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	senderID, _, _ := e.userWithMoney(t, 2_000_000)
	_, _, receiverPhone := e.userWithMoney(t, 0)

	res, err := e.svc.ExecuteEngineRequest(ctx, transfer.EngineRequestInput{
		ServiceCode:   "P2P_TRANSFER",
		SenderUserID:  senderID,
		ReceiverPhone: receiverPhone,
		Amount:        100_000,
		Idempotency:   wallet.NewRefCode(),
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn status: want completed, got %s", got)
	}

	calls := e.gw.Calls()
	if len(calls) != 1 {
		t.Fatalf("gateway calls: want 1, got %d (%+v)", len(calls), calls)
	}
	if !strings.Contains(calls[0].URL, "bankpartner.vn") {
		t.Fatalf("gateway call URL: want bank_gateway_notify endpoint, got %s", calls[0].URL)
	}
	if !strings.HasSuffix(calls[0].IdempotencyRef, ":bank_gateway_notify") {
		t.Fatalf("gateway call idempotency ref: want *:bank_gateway_notify, got %s", calls[0].IdempotencyRef)
	}
}

func TestActionPipelineWiring_EmptyActionsStillCompletes(t *testing.T) {
	e := newEnv(t, "")
	ctx := context.Background()
	senderID, _, _ := e.userWithMoney(t, 1_000_000)
	_, _, receiverPhone := e.userWithMoney(t, 0)

	// MONEY_REQUEST: services.actions='[]' in the seed — ACTIONS_BEFORE_GL must
	// report "no_actions" and never call the gateway, but the transfer itself
	// must still complete.
	res, err := e.svc.ExecuteEngineRequest(ctx, transfer.EngineRequestInput{
		ServiceCode:   "MONEY_REQUEST",
		SenderUserID:  senderID,
		ReceiverPhone: receiverPhone,
		Amount:        100_000,
		Idempotency:   wallet.NewRefCode(),
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	e.drain(t)
	if got := e.txnStatus(t, res.RefCode); got != "completed" {
		t.Fatalf("txn status: want completed, got %s", got)
	}
	if calls := e.gw.Calls(); len(calls) != 0 {
		t.Fatalf("gateway calls: want 0 for a service with no configured actions, got %d", len(calls))
	}
}

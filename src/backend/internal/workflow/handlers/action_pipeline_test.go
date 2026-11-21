package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/gateway"
	"github.com/hieumai1507/hpay-backend/internal/testutil"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
	"github.com/hieumai1507/hpay-backend/internal/workflow/handlers"
)

const hookURL = "https://partner.test/hook"

// newService inserts a throwaway service row carrying an Action Pipeline config.
func newService(t *testing.T, pool *pgxpool.Pool, priority, actionsJSON string) string {
	t.Helper()
	code := fmt.Sprintf("SVC_TEST_%d", rand.Intn(1_000_000_000))
	_, err := pool.Exec(context.Background(), `
		INSERT INTO services (code, name, category, status, action_priority, actions)
		VALUES ($1, 'Pipeline test', 'transfer', 'active', $2, $3::jsonb)`,
		code, priority, actionsJSON)
	if err != nil {
		t.Fatalf("insert service: %v", err)
	}
	return code
}

func stepInput(serviceCode, stepName string) workflow.StepInput {
	return workflow.StepInput{
		InstanceID:    uuid.New(),
		TransactionID: uuid.New(),
		Step:          workflow.StepDef{Name: stepName},
		Attempt:       1,
		Context: map[string]any{
			"service_code": serviceCode,
			"ref_code":     "REFTEST123",
		},
	}
}

// G4: a Directional Integration action fires an outbound gateway call, carrying
// an idempotency ref so partner retries dedupe.
func TestActionPipeline_CallsGatewayBeforeGL(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	code := newService(t, pool, "before_gl", fmt.Sprintf(`[
		{"code":"gl_posting","name":"GL","type":"GL Posting","entry_point":"model:transdefinition","method":"performAction","required":true},
		{"code":"bank_notify","name":"Bank","type":"Directional Integration","entry_point":%q,"method":"POST","required":false}
	]`, hookURL))

	mock := gateway.NewMock()
	reg := workflow.NewRegistry()
	handlers.RegisterActionPipeline(reg, pool, mock)
	h, ok := reg.Get("action_pipeline")
	if !ok {
		t.Fatal("action_pipeline handler not registered")
	}

	out, err := h.Execute(context.Background(), stepInput(code, "ACTION_BEFORE_GL"))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("gateway calls = %d, want 1", len(calls))
	}
	if calls[0].URL != hookURL {
		t.Errorf("url = %q, want %q", calls[0].URL, hookURL)
	}
	if calls[0].IdempotencyRef != "REFTEST123:bank_notify" {
		t.Errorf("idempotency ref = %q", calls[0].IdempotencyRef)
	}
	if out.Context["pipeline"] != "before_gl" {
		t.Errorf("pipeline phase = %v, want before_gl", out.Context["pipeline"])
	}
}

// The service's action_priority decides which slot fires: an after_gl service
// must not run in the before_gl slot.
func TestActionPipeline_SkipsMismatchedPhase(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	code := newService(t, pool, "after_gl", fmt.Sprintf(`[
		{"code":"bank_notify","name":"Bank","type":"Directional Integration","entry_point":%q,"method":"POST","required":true}
	]`, hookURL))

	mock := gateway.NewMock()
	reg := workflow.NewRegistry()
	handlers.RegisterActionPipeline(reg, pool, mock)
	h, _ := reg.Get("action_pipeline")

	if _, err := h.Execute(context.Background(), stepInput(code, "ACTION_BEFORE_GL")); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := len(mock.Calls()); got != 0 {
		t.Fatalf("gateway calls = %d, want 0 (phase mismatch)", got)
	}
}

// A failing REQUIRED action is retryable so the engine backs off rather than
// failing the transaction on the first partner blip (E04 semantics).
func TestActionPipeline_RequiredFailureIsRetryable(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	code := newService(t, pool, "before_gl", fmt.Sprintf(`[
		{"code":"bank_notify","name":"Bank","type":"Directional Integration","entry_point":%q,"method":"POST","required":true}
	]`, hookURL))

	mock := gateway.NewMock()
	mock.FailURLs[hookURL] = errors.New("partner down")
	reg := workflow.NewRegistry()
	handlers.RegisterActionPipeline(reg, pool, mock)
	h, _ := reg.Get("action_pipeline")

	_, err := h.Execute(context.Background(), stepInput(code, "ACTION_BEFORE_GL"))
	if err == nil {
		t.Fatal("expected an error for a failing required action")
	}
	if !workflow.IsRetryable(err) {
		t.Fatalf("err = %v, want RetryableError", err)
	}
}

// An OPTIONAL action that fails must not break the transaction.
func TestActionPipeline_OptionalFailureContinues(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	code := newService(t, pool, "before_gl", fmt.Sprintf(`[
		{"code":"bank_notify","name":"Bank","type":"Directional Integration","entry_point":%q,"method":"POST","required":false}
	]`, hookURL))

	mock := gateway.NewMock()
	mock.FailURLs[hookURL] = errors.New("partner down")
	reg := workflow.NewRegistry()
	handlers.RegisterActionPipeline(reg, pool, mock)
	h, _ := reg.Get("action_pipeline")

	if _, err := h.Execute(context.Background(), stepInput(code, "ACTION_BEFORE_GL")); err != nil {
		t.Fatalf("optional failure should not error, got %v", err)
	}
}

// Compensation must not re-send outbound notifications.
func TestActionPipeline_CompensationIsNoop(t *testing.T) {
	pool := testutil.PoolFromEnv(t)
	code := newService(t, pool, "before_gl", fmt.Sprintf(`[
		{"code":"bank_notify","name":"Bank","type":"Directional Integration","entry_point":%q,"method":"POST","required":true}
	]`, hookURL))

	mock := gateway.NewMock()
	reg := workflow.NewRegistry()
	handlers.RegisterActionPipeline(reg, pool, mock)
	h, _ := reg.Get("action_pipeline")

	in := stepInput(code, "ACTION_BEFORE_GL")
	in.Compensating = true
	if _, err := h.Execute(context.Background(), in); err != nil {
		t.Fatalf("compensating execute: %v", err)
	}
	if got := len(mock.Calls()); got != 0 {
		t.Fatalf("gateway calls during compensation = %d, want 0", got)
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/gateway"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

// Action mirrors one entry of services.actions (the Admin 5-tab "Action
// Pipeline Builder", docs 06 Tab 3).
type Action struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Type       string `json:"type"`        // "GL Posting" | "Directional Integration"
	EntryPoint string `json:"entry_point"` // model:... for GL, URL for integrations
	Method     string `json:"method"`
	Required   bool   `json:"required"`
}

const (
	actionTypeGL          = "gl posting"
	actionTypeIntegration = "directional integration"
)

// ActionPipeline executes the 3rd-party actions a service declares, honouring
// the service's action_priority (before_gl / after_gl). GL Posting actions are
// deliberately NOT executed here — DEBIT/CREDIT are owned by the Wallet step,
// and posting again would double-move funds. They are recorded as observed.
type ActionPipeline struct {
	pool *pgxpool.Pool
	gw   gateway.Gateway
}

// RegisterActionPipeline wires the pipeline handler. Registered separately from
// RegisterAll because it needs an outbound gateway (same pattern as fraud).
func RegisterActionPipeline(reg *workflow.Registry, pool *pgxpool.Pool, gw gateway.Gateway) {
	reg.Register(&ActionPipeline{pool: pool, gw: gw})
}

func (a *ActionPipeline) Name() string { return "action_pipeline" }

// phaseOf derives which slot this step occupies from its name, so a single
// workflow definition can carry both a before-GL and an after-GL slot and the
// service config decides which one actually fires.
func phaseOf(stepName string) string {
	if strings.Contains(strings.ToUpper(stepName), "AFTER_GL") {
		return "after_gl"
	}
	return "before_gl"
}

func (a *ActionPipeline) Execute(ctx context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	if in.Compensating {
		// Outbound notifications are not reversible; compensation is a no-op.
		return workflow.StepOutput{}, nil
	}

	serviceCode, _ := in.Context["service_code"].(string)
	if serviceCode == "" {
		return workflow.StepOutput{Context: map[string]any{"pipeline": "skipped_no_service"}}, nil
	}

	q := store.New(a.pool)
	srv, err := q.GetServiceByCode(ctx, serviceCode)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}

	phase := phaseOf(in.Step.Name)
	if !strings.EqualFold(strings.TrimSpace(srv.ActionPriority), phase) {
		return workflow.StepOutput{Context: map[string]any{"pipeline": "skipped_phase_" + phase}}, nil
	}

	var actions []Action
	if len(srv.Actions) > 0 {
		if err := json.Unmarshal(srv.Actions, &actions); err != nil {
			// Malformed config is terminal — retrying cannot fix bad JSON.
			return workflow.StepOutput{}, fmt.Errorf("service %s: bad actions config: %w", serviceCode, err)
		}
	}
	if len(actions) == 0 {
		return workflow.StepOutput{Context: map[string]any{"pipeline": "no_actions"}}, nil
	}

	refCode, _ := in.Context["ref_code"].(string)
	executed := make([]string, 0, len(actions))

	for _, act := range actions {
		switch strings.ToLower(strings.TrimSpace(act.Type)) {
		case actionTypeGL:
			// Owned by the Wallet step; recorded only.
			executed = append(executed, act.Code+":gl_delegated")

		case actionTypeIntegration:
			if !strings.HasPrefix(strings.ToLower(act.EntryPoint), "http") {
				executed = append(executed, act.Code+":skipped_no_url")
				continue
			}
			resp, cerr := a.gw.Call(ctx, gateway.Request{
				URL:            act.EntryPoint,
				Method:         act.Method,
				IdempotencyRef: refCode + ":" + act.Code,
				Payload: map[string]any{
					"ref_code":     refCode,
					"service_code": serviceCode,
					"action":       act.Code,
				},
			})
			if cerr != nil {
				if errors.Is(cerr, gateway.ErrBlockedURL) {
					// The outbound guard refused the destination (C-10). That cannot become
					// acceptable by retrying, so it is terminal for a required action and
					// a visible skip for an optional one. Only the host is logged.
					if act.Required {
						return workflow.StepOutput{}, fmt.Errorf("action %s: %w", act.Code, cerr)
					}
					executed = append(executed, act.Code+":blocked")
					continue
				}
				if act.Required {
					// Transient by assumption — let the engine retry/backoff, and
					// on_failure decide once attempts are exhausted.
					return workflow.StepOutput{}, workflow.RetryableError{Err: fmt.Errorf("action %s: %w", act.Code, cerr)}
				}
				executed = append(executed, fmt.Sprintf("%s:optional_failed(%d)", act.Code, resp.StatusCode))
				continue
			}
			executed = append(executed, fmt.Sprintf("%s:ok(%d)", act.Code, resp.StatusCode))

		default:
			executed = append(executed, act.Code+":unknown_type")
		}
	}

	return workflow.StepOutput{Context: map[string]any{
		"pipeline":         phase,
		"pipeline_actions": executed,
	}}, nil
}

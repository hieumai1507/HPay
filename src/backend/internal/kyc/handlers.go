package kyc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/storage"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

// RegisterHandlers wires the two KYC steps into the worker registry.
func RegisterHandlers(reg *workflow.Registry, pool *pgxpool.Pool, files storage.Store, override ...Scorer) {
	var ov Scorer
	if len(override) > 0 {
		ov = override[0]
	}
	reg.Register(&scorerHandler{q: store.New(pool), files: files, override: ov})
	reg.Register(&classifierHandler{pool: pool})
}

// requestIDFromContext reads the kyc_request_id the submit tx seeded into the
// instance context — the KYC workflow has no transaction (docs plans f8).
func requestIDFromContext(in workflow.StepInput) (uuid.UUID, error) {
	raw, _ := in.Context["kyc_request_id"].(string)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("kyc_request_id missing from workflow context: %q", raw)
	}
	return id, nil
}

// --- SCORE --------------------------------------------------------------------

type scorerHandler struct {
	q        *store.Queries
	files    storage.Store
	override Scorer
}

func (*scorerHandler) Name() string { return "kyc_scorer" }

func (h *scorerHandler) Execute(ctx context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	if in.Compensating {
		return workflow.StepOutput{}, nil // scoring has nothing to reverse
	}
	reqID, err := requestIDFromContext(in)
	if err != nil {
		return workflow.StepOutput{}, err // terminal: bad context never heals
	}
	req, err := h.q.GetKYCRequest(ctx, reqID)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	// Replay guard: already scored (at-least-once redelivery) — don't pay for
	// a second LLM call, just re-emit the recorded score.
	if req.AiScore >= 0 {
		return workflow.StepOutput{Context: map[string]any{"kyc_score": int(req.AiScore)}}, nil
	}

	idCard, err := h.files.Get(ctx, req.IDCardUrl)
	if err != nil {
		return workflow.StepOutput{}, fmt.Errorf("load id card image: %w", err) // terminal: file won't appear on retry
	}
	selfie, err := h.files.Get(ctx, req.SelfieUrl)
	if err != nil {
		return workflow.StepOutput{}, fmt.Errorf("load selfie image: %w", err)
	}

	var activeScorer Scorer
	if h.override != nil {
		activeScorer = h.override
	} else {
		// Fetch dynamic active setting for kyc_scoring from DB
		setting, err := h.q.GetActiveAISetting(ctx, "kyc_scoring")
		if err != nil {
			return workflow.StepOutput{}, workflow.RetryableError{Err: fmt.Errorf("fetch AI configs: %w", err)}
		}

		switch setting.Provider {
		case "mock":
			activeScorer = MockScorer{}
		case "anthropic":
			if !setting.KeyActive.Bool || !setting.KeyActive.Valid {
				return workflow.StepOutput{}, fmt.Errorf("AI key is inactive: %s (code: ERR_AI_PROVIDER_UNAVAILABLE)", setting.KeyName.String)
			}
			activeScorer = NewClaudeScorer(setting.ApiKey.String)
		case "openai":
			if !setting.KeyActive.Bool || !setting.KeyActive.Valid {
				return workflow.StepOutput{}, fmt.Errorf("AI key is inactive: %s (code: ERR_AI_PROVIDER_UNAVAILABLE)", setting.KeyName.String)
			}
			activeScorer = NewOpenAIScorer(setting.ApiKey.String, setting.ModelName)
		default:
			activeScorer = MockScorer{}
		}
	}

	res, err := activeScorer.Score(ctx, idCard, selfie)
	if err != nil {
		return workflow.StepOutput{}, err // scorer already classified retryable vs terminal
	}
	verdict, _ := Classify(res.Score)
	reasons, err := json.Marshal(res.Reasons)
	if err != nil {
		return workflow.StepOutput{}, err
	}
	if err := h.q.SetKYCScore(ctx, store.SetKYCScoreParams{
		ID: reqID, AiScore: int32(res.Score),
		AiVerdict: pgtype.Text{String: string(verdict), Valid: true},
		AiReasons: reasons,
	}); err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	return workflow.StepOutput{Context: map[string]any{"kyc_score": res.Score}}, nil
}

// --- CLASSIFY ------------------------------------------------------------------

type classifierHandler struct {
	pool *pgxpool.Pool
}

func (*classifierHandler) Name() string { return "kyc_classifier" }

func (h *classifierHandler) Execute(ctx context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	if in.Compensating {
		return workflow.StepOutput{}, nil
	}
	reqID, err := requestIDFromContext(in)
	if err != nil {
		return workflow.StepOutput{}, err
	}
	q := store.New(h.pool)
	req, err := q.GetKYCRequest(ctx, reqID)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	if req.Status != store.KycStatusPending {
		// Replay: a previous attempt already classified — no-op (guarded).
		return workflow.StepOutput{Context: map[string]any{"kyc_status": string(req.Status)}}, nil
	}
	if req.AiScore < 0 {
		return workflow.StepOutput{}, errors.New("classify before score: ai_score missing")
	}

	_, newStatus := Classify(int(req.AiScore))

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	defer tx.Rollback(ctx)
	qtx := q.WithTx(tx)

	if _, err := qtx.ClassifyKYCGuarded(ctx, store.ClassifyKYCGuardedParams{ID: reqID, Status: newStatus}); err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	if _, err := qtx.SetUserKycFromPending(ctx, store.SetUserKycFromPendingParams{ID: req.UserID, KycStatus: newStatus}); err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	if err := tx.Commit(ctx); err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	return workflow.StepOutput{Context: map[string]any{"kyc_status": string(newStatus)}}, nil
}

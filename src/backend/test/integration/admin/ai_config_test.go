package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hieumai1507/hpay-backend/internal/ai"
	"github.com/hieumai1507/hpay-backend/internal/kyc"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

func TestAIConfig_DefaultMock(t *testing.T) {
	e := testkit.NewEnv(t, "ai_config")
	defer e.Pool.Close()

	userID, _, _ := e.UserWithMoney(t, 0)
	res, err := e.Ksvc.Submit(context.Background(), userID, testkit.TinyPNG, testkit.TinyPNG)
	if err != nil {
		t.Fatalf("submit KYC: %v", err)
	}

	e.Drain(t)

	// Fetch scoring outcome
	req, err := e.Q.GetKYCRequest(context.Background(), res.RequestID)
	if err != nil {
		t.Fatalf("get kyc request: %v", err)
	}

	if req.AiScore < 0 {
		t.Errorf("expected AI score to be filled, got %d", req.AiScore)
	}
	if req.AiVerdict == "" {
		t.Errorf("expected AI verdict to be filled")
	}
}

func TestAIConfig_OpenAIRoutingAndInactivation(t *testing.T) {
	e := testkit.NewEnv(t, "ai_config")
	defer e.Pool.Close()

	// Restore default mock settings at the end of the test to prevent polluting other tests
	defer func() {
		_, _ = e.Q.UpdateAISetting(context.Background(), store.UpdateAISettingParams{
			FunctionName: "kyc_scoring",
			Provider:     "mock",
			ModelName:    "mock",
			KeyID:        uuid.Nil,
		})
	}()

	// 1. Setup mock OpenAI API server
	called := false
	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return OpenAI structure
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": `{"score": 85, "reasons": ["Face matches perfectly", "ID card is valid"]}`,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockOpenAI.Close()

	// Intercept OpenAI endpoint URL
	originalURL := "https://api.openai.com/v1/chat/completions"
	kyc.SetOpenAIURL(mockOpenAI.URL)
	ai.SetOpenAIURL(mockOpenAI.URL)
	defer func() {
		kyc.SetOpenAIURL(originalURL)
		ai.SetOpenAIURL(originalURL)
	}()

	// 2. Add an OpenAI Key to DB
	key, err := e.Q.InsertAIKey(context.Background(), store.InsertAIKeyParams{
		Name:     "Test OpenAI Key",
		Provider: "openai",
		ApiKey:   pgtype.Text{String: "sk-test-1234567890", Valid: true},
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("insert key: %v", err)
	}

	// 3. Map kyc_scoring function to OpenAI
	_, err = e.Q.UpdateAISetting(context.Background(), store.UpdateAISettingParams{
		FunctionName: "kyc_scoring",
		Provider:     "openai",
		ModelName:    "gpt-4o",
		KeyID:        key.ID,
	})
	if err != nil {
		t.Fatalf("update setting: %v", err)
	}

	// 4. Run KYC and verify it hit the OpenAI mock endpoint
	userID, _, _ := e.UserWithMoney(t, 0)
	res, err := e.Ksvc.Submit(context.Background(), userID, testkit.TinyPNG, testkit.TinyPNG)
	if err != nil {
		t.Fatalf("submit KYC: %v", err)
	}

	e.Drain(t)

	if !called {
		t.Errorf("expected OpenAI mock server to be called, but it was not")
	}

	req, err := e.Q.GetKYCRequest(context.Background(), res.RequestID)
	if err != nil {
		t.Fatalf("get kyc request: %v", err)
	}
	if req.AiScore != 85 {
		t.Errorf("expected AI score to be 85, got %d", req.AiScore)
	}

	// 5. Disable (deactivate) the key and verify KYC fails with provider unavailable error
	_, err = e.Q.UpdateAIKey(context.Background(), store.UpdateAIKeyParams{
		ID:       key.ID,
		Name:     "Test OpenAI Key",
		ApiKey:   pgtype.Text{String: "sk-test-1234567890", Valid: true},
		IsActive: false,
	})
	if err != nil {
		t.Fatalf("update key is_active: %v", err)
	}

	userID2, _, _ := e.UserWithMoney(t, 0)
	res2, err := e.Ksvc.Submit(context.Background(), userID2, testkit.TinyPNG, testkit.TinyPNG)
	if err != nil {
		t.Fatalf("submit KYC: %v", err)
	}

	// When drained, the scoring step should fail and throw an error because key is inactive
	e.Drain(t)

	req2, err := e.Q.GetKYCRequest(context.Background(), res2.RequestID)
	if err != nil {
		t.Fatalf("get kyc request 2: %v", err)
	}

	// Because key was inactive, ai_score should remain -1 (not scored)
	if req2.AiScore >= 0 {
		t.Errorf("expected KYC score to not be populated when key is inactive, got %d", req2.AiScore)
	}
}

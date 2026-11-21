package kyc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

var openaiURL = "https://api.openai.com/v1/chat/completions"

func SetOpenAIURL(u string) { openaiURL = u }

// OpenAIScorer calls the OpenAI chat completions API (vision) over net/http.
type OpenAIScorer struct {
	apiKey    string
	modelName string
	client    *http.Client
}

func NewOpenAIScorer(apiKey, modelName string) *OpenAIScorer {
	if modelName == "" || modelName == "mock" {
		modelName = "gpt-4o"
	}
	return &OpenAIScorer{
		apiKey:    apiKey,
		modelName: modelName,
		client:    &http.Client{Timeout: 25 * time.Second},
	}
}

func (*OpenAIScorer) Name() string { return "openai" }

func (o *OpenAIScorer) Score(ctx context.Context, idCard, selfie []byte) (Result, error) {
	type openAIMessage struct {
		Role    string `json:"role"`
		Content []any  `json:"content"`
	}

	messages := []openAIMessage{
		{
			Role: "user",
			Content: []any{
				map[string]any{
					"type": "text",
					"text": scorePrompt,
				},
				openaiImageBlock(idCard),
				openaiImageBlock(selfie),
			},
		},
	}

	body, err := json.Marshal(map[string]any{
		"model":           o.modelName,
		"messages":        messages,
		"max_tokens":      500,
		"response_format": map[string]string{"type": "json_object"},
	})
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiURL, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return Result{}, workflow.RetryableError{Err: fmt.Errorf("openai request: %w", err)}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Result{}, workflow.RetryableError{Err: err}
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return Result{}, workflow.RetryableError{Err: fmt.Errorf("openai status %d", resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("openai status %d: %s", resp.StatusCode, string(raw))
	}

	type openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var or openaiResp
	if err := json.Unmarshal(raw, &or); err != nil {
		return Result{}, workflow.RetryableError{Err: fmt.Errorf("openai response parse: %w", err)}
	}
	if len(or.Choices) == 0 {
		return Result{}, workflow.RetryableError{Err: fmt.Errorf("no choices returned by openai")}
	}

	res, err := parseScoreJSON(or.Choices[0].Message.Content)
	if err != nil {
		return Result{}, workflow.RetryableError{Err: err}
	}
	return res, nil
}

func openaiImageBlock(img []byte) map[string]any {
	contentType := http.DetectContentType(img)
	b64 := base64.StdEncoding.EncodeToString(img)
	return map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url": fmt.Sprintf("data:%s;base64,%s", contentType, b64),
		},
	}
}

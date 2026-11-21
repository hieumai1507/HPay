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

var anthropicURL = "https://api.anthropic.com/v1/messages"

func SetAnthropicURL(u string) { anthropicURL = u }

const (
	anthropicVersion = "2023-06-01"
	claudeModel      = "claude-sonnet-5"
	maxResponseBytes = 1 << 20
)

// ClaudeScorer calls the Anthropic messages API (vision) over plain net/http —
// no SDK dependency. Network/5xx problems surface as workflow.RetryableError
// so the SCORE step retries with backoff; a malformed model reply is also
// retryable (the model may do better next attempt).
type ClaudeScorer struct {
	apiKey string
	client *http.Client
}

func NewClaudeScorer(apiKey string) *ClaudeScorer {
	return &ClaudeScorer{apiKey: apiKey, client: &http.Client{Timeout: 25 * time.Second}}
}

func (*ClaudeScorer) Name() string { return "claude" }

const scorePrompt = `You are a KYC verification assistant. You are given two images:
first a government ID card photo, second a selfie of the person claiming that identity.
Assess: (1) is the ID card readable and plausibly genuine, (2) does the selfie
plausibly match the ID photo, (3) signs of tampering, screenshots, or printouts.
Respond with ONLY a JSON object, no prose:
{"score": <integer 0-100, overall confidence this is a legitimate match>,
 "reasons": [<short strings explaining the score, in Vietnamese>]}`

type anthropicResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (c *ClaudeScorer) Score(ctx context.Context, idCard, selfie []byte) (Result, error) {
	body, err := json.Marshal(map[string]any{
		"model":      claudeModel,
		"max_tokens": 500,
		"messages": []map[string]any{{
			"role": "user",
			"content": []map[string]any{
				imageBlock(idCard),
				imageBlock(selfie),
				{"type": "text", "text": scorePrompt},
			},
		}},
	})
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicURL, bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.client.Do(req)
	if err != nil {
		return Result{}, workflow.RetryableError{Err: fmt.Errorf("anthropic request: %w", err)}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Result{}, workflow.RetryableError{Err: err}
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return Result{}, workflow.RetryableError{Err: fmt.Errorf("anthropic status %d", resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		// 4xx (bad key, bad request): retrying with the same input won't help.
		return Result{}, fmt.Errorf("anthropic status %d", resp.StatusCode)
	}

	var ar anthropicResp
	if err := json.Unmarshal(raw, &ar); err != nil {
		return Result{}, workflow.RetryableError{Err: fmt.Errorf("anthropic response parse: %w", err)}
	}
	var text string
	for _, block := range ar.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	res, err := parseScoreJSON(text)
	if err != nil {
		return Result{}, workflow.RetryableError{Err: err}
	}
	return res, nil
}

func imageBlock(img []byte) map[string]any {
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type":       "base64",
			"media_type": http.DetectContentType(img),
			"data":       base64.StdEncoding.EncodeToString(img),
		},
	}
}

// parseScoreJSON extracts the {"score":..,"reasons":[..]} object from model
// text (tolerates surrounding prose/markdown fences) and clamps the score.
func parseScoreJSON(text string) (Result, error) {
	start := bytes.IndexByte([]byte(text), '{')
	end := bytes.LastIndexByte([]byte(text), '}')
	if start < 0 || end <= start {
		return Result{}, fmt.Errorf("no JSON object in model reply")
	}
	var res Result
	if err := json.Unmarshal([]byte(text[start:end+1]), &res); err != nil {
		return Result{}, fmt.Errorf("model reply not valid score JSON: %w", err)
	}
	if res.Score < 0 {
		res.Score = 0
	}
	if res.Score > 100 {
		res.Score = 100
	}
	return res, nil
}

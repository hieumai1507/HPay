package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	anthropicVersion = "2023-06-01"
	claudeModel      = "claude-sonnet-5"
	maxResponseBytes = 1 << 20
)

var anthropicURL = "https://api.anthropic.com/v1/messages"

func SetAnthropicURL(u string) { anthropicURL = u }

// ClaudeAdvisor calls the Anthropic messages API for a text completion over
// plain net/http (no SDK). Errors are returned so callers can fall back to no
// advice — this is never on a money path.
type ClaudeAdvisor struct {
	apiKey string
	client *http.Client
}

func NewClaudeAdvisor(apiKey string) *ClaudeAdvisor {
	return &ClaudeAdvisor{apiKey: apiKey, client: &http.Client{Timeout: 15 * time.Second}}
}

func (*ClaudeAdvisor) Name() string { return "claude" }

type anthropicResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (c *ClaudeAdvisor) Complete(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":      claudeModel,
		"max_tokens": 400,
		"system":     system,
		"messages":   []map[string]any{{"role": "user", "content": user}},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic status %d", resp.StatusCode)
	}
	var ar anthropicResp
	if err := json.Unmarshal(raw, &ar); err != nil {
		return "", err
	}
	var text string
	for _, b := range ar.Content {
		if b.Type == "text" {
			text += b.Text
		}
	}
	return text, nil
}

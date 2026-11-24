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

var openaiURL = "https://api.openai.com/v1/chat/completions"

func SetOpenAIURL(u string) { openaiURL = u }

// OpenAIAdvisor calls the OpenAI chat completions API for natural language assistance.
type OpenAIAdvisor struct {
	apiKey    string
	modelName string
	client    *http.Client
}

func NewOpenAIAdvisor(apiKey, modelName string) *OpenAIAdvisor {
	if modelName == "" || modelName == "mock" {
		modelName = "gpt-4o"
	}
	return &OpenAIAdvisor{
		apiKey:    apiKey,
		modelName: modelName,
		client:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (*OpenAIAdvisor) Name() string { return "openai" }

func (o *OpenAIAdvisor) Complete(ctx context.Context, system, user string) (string, error) {
	type openaiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	messages := []openaiMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}

	body, err := json.Marshal(map[string]any{
		"model":    o.modelName,
		"messages": messages,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openaiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai error %d: %s", resp.StatusCode, string(raw))
	}

	type openaiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var or openaiResp
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return "", err
	}
	if len(or.Choices) == 0 {
		return "", fmt.Errorf("openai empty choices")
	}

	return or.Choices[0].Message.Content, nil
}

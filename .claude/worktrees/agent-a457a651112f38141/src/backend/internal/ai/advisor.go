// Package ai provides the LLM advisory used by fraud explanations and recon
// summaries (docs 00 §8.2). It is ALWAYS advisory — the LLM never decides
// anything about money; a mock fallback keeps CI/offline deterministic.
package ai

import (
	"context"
	"strings"
)

// Advisor turns a system + user prompt into a short natural-language note.
type Advisor interface {
	Name() string
	// Complete returns advisory text. Callers treat any error as "no advice"
	// and proceed — the LLM must never block a decision.
	Complete(ctx context.Context, system, user string) (string, error)
}

// MockAdvisor is the offline/CI fallback: a deterministic template so tests
// can assert non-empty advisory text without a network call or a key.
type MockAdvisor struct{}

func (MockAdvisor) Name() string { return "mock" }

func (MockAdvisor) Complete(_ context.Context, _system, user string) (string, error) {
	// Echo a trimmed slice of the context so the output is obviously derived
	// from the input (useful in demos) but clearly labelled as mock.
	snippet := strings.TrimSpace(user)
	if len(snippet) > 160 {
		snippet = snippet[:160] + "…"
	}
	return "[mock advisory] " + snippet, nil
}

// New picks the real Claude advisor when a key is present, mock otherwise.
func New(apiKey string) Advisor {
	if apiKey == "" {
		return MockAdvisor{}
	}
	return NewClaudeAdvisor(apiKey)
}

// Package kyc implements identity verification: images go to storage, a
// vision scorer (Claude or deterministic mock) grades them 0-100, and
// deterministic thresholds classify the outcome (docs 00 §8.2). The LLM never
// decides anything — it only produces a score; thresholds + humans decide.
package kyc

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
)

// Result is what a scorer returns. Score is an int 0-100 end-to-end — no
// float anywhere near this codebase.
type Result struct {
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}

// Scorer grades an id-card + selfie pair.
type Scorer interface {
	Name() string
	Score(ctx context.Context, idCard, selfie []byte) (Result, error)
}

// --- mock ---------------------------------------------------------------------

// MockScorer is the offline/CI fallback (no ANTHROPIC_API_KEY): deterministic
// by image content, so the same upload always yields the same verdict — a
// repeatable demo instead of pure randomness.
type MockScorer struct{}

func (MockScorer) Name() string { return "mock" }

func (MockScorer) Score(_ context.Context, idCard, selfie []byte) (Result, error) {
	h := sha256.New()
	h.Write(idCard)
	h.Write(selfie)
	sum := h.Sum(nil)
	// Map the hash into 40..99 so mock demos mostly land in manual/auto bands
	// but can still hit every branch with crafted inputs.
	score := 40 + int(binary.BigEndian.Uint16(sum[:2])%60)
	return Result{
		Score:   score,
		Reasons: []string{"mock scorer (no ANTHROPIC_API_KEY): score derived from image hash"},
	}, nil
}

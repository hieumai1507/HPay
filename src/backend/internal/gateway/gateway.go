// Package gateway is the outbound seam for 3rd-party "Directional Integration"
// actions declared in a service's Action Pipeline (docs 05/06; audit gap G4).
//
// It is an interface on purpose: integration tests and dev runs use Mock so the
// engine stays deterministic, while production swaps in HTTP without touching
// the step handler.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Request is one outbound partner call derived from an action definition.
type Request struct {
	URL    string
	Method string
	// IdempotencyRef lets the partner dedupe retries (engine is at-least-once).
	IdempotencyRef string
	Payload        map[string]any
}

type Response struct {
	StatusCode int
	Body       string
}

type Gateway interface {
	Call(ctx context.Context, req Request) (Response, error)
}

// --- HTTP -------------------------------------------------------------------

type HTTP struct{ client *http.Client }

func NewHTTP(timeout time.Duration) *HTTP {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTP{client: &http.Client{Timeout: timeout}}
}

func (h *HTTP) Call(ctx context.Context, req Request) (Response, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodPost
	}
	body, err := json.Marshal(req.Payload)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.IdempotencyRef != "" {
		httpReq.Header.Set("Idempotency-Key", req.IdempotencyRef)
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	out := Response{StatusCode: resp.StatusCode, Body: string(raw)}
	if resp.StatusCode >= 400 {
		return out, fmt.Errorf("gateway %s returned %d", req.URL, resp.StatusCode)
	}
	return out, nil
}

// --- Mock -------------------------------------------------------------------

// Mock records calls and replays canned outcomes. Safe for concurrent workers.
type Mock struct {
	mu    sync.Mutex
	calls []Request
	// FailURLs makes any call to a matching URL fail (error-path tests).
	FailURLs map[string]error
}

func NewMock() *Mock { return &Mock{FailURLs: map[string]error{}} }

func (m *Mock) Call(_ context.Context, req Request) (Response, error) {
	m.mu.Lock()
	m.calls = append(m.calls, req)
	m.mu.Unlock()

	if err, bad := m.FailURLs[req.URL]; bad {
		return Response{StatusCode: 502}, err
	}
	return Response{StatusCode: 200, Body: `{"status":"ok"}`}, nil
}

// Calls returns a copy of everything recorded so far.
func (m *Mock) Calls() []Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Request, len(m.calls))
	copy(out, m.calls)
	return out
}

func (m *Mock) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
}

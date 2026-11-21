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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

// HTTP is the production gateway. Every call goes through the outbound URL guard
// (guard.go): https only, public addresses only (checked on the address actually
// dialed), no redirects, POST or PUT only, optional host allowlist from
// OUTBOUND_ALLOWED_HOSTS.
type HTTP struct {
	client *http.Client
	guard  *Guard
}

func NewHTTP(timeout time.Duration) *HTTP {
	return newHTTP(timeout, NewGuard(GuardOptions{AllowedHosts: os.Getenv("OUTBOUND_ALLOWED_HOSTS")}), nil)
}

// newHTTPForTest is a test-only seam: it permits the loopback address (so a local TLS
// server can be reached) and trusts the supplied TLS config. Unexported, so production
// code cannot construct a gateway that is weaker than NewHTTP.
func newHTTPForTest(timeout time.Duration, tlsCfg *tls.Config) *HTTP {
	return newHTTP(timeout, NewGuard(GuardOptions{allowLoopback: true}), tlsCfg)
}

func newHTTP(timeout time.Duration, g *Guard, tlsCfg *tls.Config) *HTTP {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tr := &http.Transport{
		DialContext:         g.DialContext,
		TLSClientConfig:     tlsCfg,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableKeepAlives:   true,
		Proxy:               nil, // never route through an environment proxy: it would bypass the address check
	}
	return &HTTP{
		guard: g,
		client: &http.Client{
			Timeout:   timeout,
			Transport: tr,
			// A redirect could point anywhere (including an internal address); never follow it.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// FormClient is a guarded client for provider APIs that take form-encoded POSTs with Basic auth
// (for example Twilio). It shares the outbound URL guard of HTTP: https only, public addresses
// only, no redirects. The credentials are passed per call and never logged here.
type FormClient struct{ h *HTTP }

// NewFormClient builds the production client (allowlist from OUTBOUND_ALLOWED_HOSTS).
func NewFormClient(timeout time.Duration) *FormClient { return &FormClient{h: NewHTTP(timeout)} }

// TestClient is a test-only constructor that permits loopback and trusts the given TLS config.
// Production code uses NewFormClient, which cannot be weakened.
func TestClient(tlsCfg *tls.Config) *FormClient {
	return &FormClient{h: newHTTP(5*time.Second, NewGuard(GuardOptions{allowLoopback: true}), tlsCfg)}
}

// PostForm sends form fields with Basic auth and returns the status code. The body of a failed
// response is not returned (it can echo request data); only the host and status are reported.
func (c *FormClient) PostForm(ctx context.Context, rawURL, user, pass string, form map[string]string) (int, error) {
	if err := c.h.guard.CheckURL(rawURL); err != nil {
		return 0, err
	}
	vals := make(url.Values, len(form))
	for k, v := range form {
		vals.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(vals.Encode()))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(user, pass)
	resp, err := c.h.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("provider call to %s failed", hostOf(rawURL))
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("provider call to %s returned %d", hostOf(rawURL), resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func (h *HTTP) Call(ctx context.Context, req Request) (Response, error) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodPost
	}
	if method != http.MethodPost && method != http.MethodPut {
		return Response{}, blocked("method %q is not allowed (POST or PUT only)", method)
	}
	if err := h.guard.CheckURL(req.URL); err != nil {
		return Response{}, err
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
	if resp.StatusCode >= 300 {
		// 3xx is a refused redirect (never followed); >= 400 is a partner error.
		// Report the host only: the URL may carry tokens in its query string.
		return out, fmt.Errorf("gateway call to %s returned %d", hostOf(req.URL), resp.StatusCode)
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

// hostOf returns only the host of a URL for logs and errors (never the query or path).
func hostOf(raw string) string {
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if i := strings.IndexAny(raw, "/?#"); i >= 0 {
		raw = raw[:i]
	}
	return raw
}

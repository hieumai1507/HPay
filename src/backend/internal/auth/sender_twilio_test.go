package auth

// ADR-0017 / spec w4-2fa: the Twilio adapter behind the Sender port and the choice of sender
// by environment. Cases TC-2FA-019..021. No test reaches the real Twilio: a local TLS server
// stands in, reached only through the test seam.

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/gateway"
)

type seenRequest struct {
	path, method, contentType, user, pass, body string
}

func twilioStub(t *testing.T, status int) (*httptest.Server, *seenRequest, *sync.Mutex) {
	t.Helper()
	var mu sync.Mutex
	seen := &seenRequest{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		u, p, _ := r.BasicAuth()
		mu.Lock()
		*seen = seenRequest{r.URL.Path, r.Method, r.Header.Get("Content-Type"), u, p, string(b)}
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"sid":"SM123","status":"queued"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, seen, &mu
}

func testTwilioConfig() TwilioConfig {
	return TwilioConfig{AccountSID: "ACtest", APIKey: "SKtest", APISecret: "secret-value", From: "+15550001111"}
}

// TC-2FA-019
func TestTwilioSender_SendsThroughMessagesAPI(t *testing.T) {
	srv, seen, mu := twilioStub(t, 201)
	var logBuf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := testTwilioConfig()
	cfg.BaseURL = srv.URL
	tr := srv.Client().Transport.(*http.Transport)
	s := newTwilioSenderForTest(cfg, log, gateway.TestClient(tr.TLSClientConfig), func(context.Context, uuid.UUID) (string, error) { return "+84901234567", nil })

	if err := s.SendCode(context.Background(), uuid.New(), PurposePINReset, "482915"); err != nil {
		t.Fatalf("send: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen.method != "POST" || seen.path != "/2010-04-01/Accounts/ACtest/Messages.json" {
		t.Fatalf("wrong request line: %s %s", seen.method, seen.path)
	}
	if !strings.HasPrefix(seen.contentType, "application/x-www-form-urlencoded") {
		t.Fatalf("content type %q", seen.contentType)
	}
	if seen.user != "SKtest" || seen.pass != "secret-value" {
		t.Fatalf("basic auth must carry the API key and secret, got %q / %q", seen.user, seen.pass)
	}
	for _, want := range []string{"To=%2B84901234567", "From=%2B15550001111", "Body="} {
		if !strings.Contains(seen.body, want) {
			t.Errorf("form body is missing %s: %s", want, seen.body)
		}
	}
	if !strings.Contains(seen.body, "482915") {
		t.Errorf("the message must carry the code")
	}
	if strings.Contains(logBuf.String(), "482915") || strings.Contains(logBuf.String(), "secret-value") {
		t.Fatalf("the log leaks the code or the API secret: %s", logBuf.String())
	}
}

func TestTwilioSender_ErrorStatusIsAnError(t *testing.T) {
	srv, _, _ := twilioStub(t, 400)
	cfg := testTwilioConfig()
	cfg.BaseURL = srv.URL
	tr := srv.Client().Transport.(*http.Transport)
	s := newTwilioSenderForTest(cfg, slog.Default(), gateway.TestClient(tr.TLSClientConfig), func(context.Context, uuid.UUID) (string, error) { return "+84901234567", nil })
	if err := s.SendCode(context.Background(), uuid.New(), PurposePINReset, "482915"); err == nil {
		t.Fatal("a 400 from the provider must be reported, never swallowed")
	}
}

func TestTwilioSender_NoPhoneMeansNoSend(t *testing.T) {
	srv, seen, mu := twilioStub(t, 201)
	cfg := testTwilioConfig()
	cfg.BaseURL = srv.URL
	tr := srv.Client().Transport.(*http.Transport)
	s := newTwilioSenderForTest(cfg, slog.Default(), gateway.TestClient(tr.TLSClientConfig), func(context.Context, uuid.UUID) (string, error) { return "", ErrSenderUnavailable })
	if err := s.SendCode(context.Background(), uuid.New(), PurposePINReset, "482915"); err == nil {
		t.Fatal("no phone number: must fail")
	}
	mu.Lock()
	defer mu.Unlock()
	if seen.path != "" {
		t.Fatal("nothing may be sent without a destination")
	}
}

func TestToE164(t *testing.T) {
	cases := map[string]string{"0901234567": "+84901234567", "+84901234567": "+84901234567", "84901234567": "+84901234567"}
	for in, want := range cases {
		if got, err := toE164(in); err != nil || got != want {
			t.Errorf("toE164(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "abc", "123", "0901 234 567 89012345678"} {
		if _, err := toE164(bad); err == nil {
			t.Errorf("toE164(%q) must fail", bad)
		}
	}
}

// TC-2FA-020
func TestSenderSelection_ByEnvironmentAndProvider(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	lookup := func(context.Context, uuid.UUID) (string, error) { return "+84901234567", nil }

	// production without a provider: refuses
	s := NewSenderFromConfig(SenderConfig{Env: "production", Provider: ""}, log, lookup)
	if err := s.SendCode(context.Background(), uuid.New(), PurposePINReset, "123456"); err == nil {
		t.Fatal("production without a provider must refuse")
	}
	// production with twilio but incomplete settings: refuses (never silently falls back to the log sender)
	s = NewSenderFromConfig(SenderConfig{Env: "production", Provider: "twilio", Twilio: TwilioConfig{AccountSID: "AC"}}, log, lookup)
	if err := s.SendCode(context.Background(), uuid.New(), PurposePINReset, "123456"); err == nil {
		t.Fatal("incomplete Twilio settings must refuse")
	}
	// production with complete twilio settings: a Twilio sender
	if _, ok := NewSenderFromConfig(SenderConfig{Env: "production", Provider: "twilio", Twilio: testTwilioConfig()}, log, lookup).(*twilioSender); !ok {
		t.Fatal("production with complete Twilio settings must use the Twilio sender")
	}
	// dev and ci: log-only
	for _, env := range []string{"dev", "ci"} {
		if _, ok := NewSenderFromConfig(SenderConfig{Env: env}, log, lookup).(logSender); !ok {
			t.Fatalf("%s must use the log-only sender", env)
		}
	}
	// dev with twilio settings still honours the provider
	if _, ok := NewSenderFromConfig(SenderConfig{Env: "dev", Provider: "twilio", Twilio: testTwilioConfig()}, log, lookup).(*twilioSender); !ok {
		t.Fatal("an explicit provider is honoured in dev too")
	}
}

// TC-2FA-021
func TestTwilioSender_UsesOutboundGuard(t *testing.T) {
	cfg := testTwilioConfig()
	cfg.BaseURL = "https://127.0.0.1:1" // loopback: the production guard must refuse it
	s := NewTwilioSender(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context, uuid.UUID) (string, error) { return "+84901234567", nil })
	start := time.Now()
	err := s.SendCode(context.Background(), uuid.New(), PurposePINReset, "123456")
	if err == nil {
		t.Fatal("a loopback destination must be refused")
	}
	if !strings.Contains(err.Error(), "blocked") && !strings.Contains(strings.ToLower(err.Error()), "not allowed") {
		t.Fatalf("want the outbound guard to refuse it, got %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("the guard must refuse before any network round trip")
	}
	_ = tls.VersionTLS12
}

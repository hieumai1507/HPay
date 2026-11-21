package gateway

// Outbound URL guard (constitution C-10, docs/specs/w3-ssrf-guard).
// Cases: AC-1..AC-9. No test here touches the real network except a local TLS
// server on 127.0.0.1, reached only through the test-only seam.

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestGuard(resolve func(ctx context.Context, host string) ([]net.IP, error), allowed string) *Guard {
	return NewGuard(GuardOptions{AllowedHosts: allowed, Resolve: resolve})
}

func fixed(ips ...string) func(context.Context, string) ([]net.IP, error) {
	return func(_ context.Context, _ string) ([]net.IP, error) {
		var out []net.IP
		for _, s := range ips {
			out = append(out, net.ParseIP(s))
		}
		return out, nil
	}
}

// AC-1, AC-7: only https, no credentials, a real host, no control characters.
func TestGuard_RejectsBadURLs(t *testing.T) {
	g := newTestGuard(fixed("93.184.216.34"), "")
	bad := []string{
		"http://example.com/hook",
		"ftp://example.com/x",
		"file:///etc/passwd",
		"gopher://example.com",
		"//example.com/x",
		"https://",
		"https:///path",
		"https://user:pass@example.com/x",
		"https://example.com/\r\nInjected: 1",
		"https://exa mple.com/x",
		"",
		"not a url",
	}
	for _, u := range bad {
		if err := g.CheckURL(u); !errors.Is(err, ErrBlockedURL) {
			t.Errorf("CheckURL(%q) = %v, want ErrBlockedURL", u, err)
		}
	}
	if err := g.CheckURL("https://example.com/hook"); err != nil {
		t.Errorf("a plain public https URL must pass the static check, got %v", err)
	}
}

// AC-2: literal internal addresses.
func TestGuard_RejectsInternalLiteralIPs(t *testing.T) {
	g := newTestGuard(fixed("93.184.216.34"), "")
	for _, host := range []string{
		"127.0.0.1", "127.1.2.3", "10.0.0.5", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "169.254.0.1", "100.64.0.1", "0.0.0.0", "224.0.0.1",
		"[::1]", "[fe80::1]", "[fd00::1]", "[::ffff:127.0.0.1]", "[::]", "[ff02::1]",
	} {
		if err := g.CheckURL("https://" + host + "/x"); !errors.Is(err, ErrBlockedURL) {
			t.Errorf("literal %s must be blocked, got %v", host, err)
		}
	}
	for _, host := range []string{"93.184.216.34", "1.1.1.1", "8.8.8.8"} {
		if err := g.CheckURL("https://" + host + "/x"); err != nil {
			t.Errorf("public literal %s must pass, got %v", host, err)
		}
	}
}

func TestIsBlockedIPTable(t *testing.T) {
	blocked := []string{"127.0.0.1", "10.1.1.1", "172.20.0.1", "192.168.0.10", "169.254.169.254", "100.100.100.100",
		"0.0.0.0", "255.255.255.255", "224.0.0.251", "::1", "fe80::1", "fc00::1", "::", "::ffff:10.0.0.1"}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	for _, s := range []string{"93.184.216.34", "8.8.8.8", "2606:4700:4700::1111", "172.32.0.1", "100.63.255.255", "100.128.0.1"} {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s is public and must not be blocked", s)
		}
	}
}

// AC-3, AC-4: the address that is actually dialed is checked, every resolved address must pass.
func TestGuard_BlocksHostnameResolvingInternally(t *testing.T) {
	cases := map[string][]string{
		"resolves to loopback":         {"127.0.0.1"},
		"resolves to metadata":         {"169.254.169.254"},
		"resolves to private":          {"10.0.0.7"},
		"one public one private":       {"93.184.216.34", "10.0.0.7"},
		"one public one ipv6 loopback": {"93.184.216.34", "::1"},
	}
	for name, ips := range cases {
		t.Run(name, func(t *testing.T) {
			g := newTestGuard(fixed(ips...), "")
			_, err := g.safeDialAddrs(context.Background(), "partner.example.com")
			if !errors.Is(err, ErrBlockedURL) {
				t.Fatalf("want ErrBlockedURL, got %v", err)
			}
		})
	}
	g := newTestGuard(fixed("93.184.216.34", "2606:4700:4700::1111"), "")
	if _, err := g.safeDialAddrs(context.Background(), "partner.example.com"); err != nil {
		t.Fatalf("all-public resolution must pass: %v", err)
	}
	g = newTestGuard(func(context.Context, string) ([]net.IP, error) { return nil, errors.New("nxdomain") }, "")
	if _, err := g.safeDialAddrs(context.Background(), "nope.example.com"); err == nil {
		t.Fatal("a resolution failure must fail closed")
	}
}

// AC-6: optional host allowlist, exact and wildcard.
func TestGuard_AllowedHosts(t *testing.T) {
	g := newTestGuard(fixed("93.184.216.34"), "api.partner.com, *.bank.example")
	for _, u := range []string{"https://api.partner.com/x", "https://sandbox.bank.example/x", "https://a.b.bank.example/x"} {
		if err := g.CheckURL(u); err != nil {
			t.Errorf("%s must be allowed: %v", u, err)
		}
	}
	for _, u := range []string{"https://evil.com/x", "https://partner.com/x", "https://bank.example/x", "https://api.partner.com.evil.com/x", "https://xapi.partner.com/x"} {
		if err := g.CheckURL(u); !errors.Is(err, ErrBlockedURL) {
			t.Errorf("%s must be refused by the allowlist, got %v", u, err)
		}
	}
	open := newTestGuard(fixed("93.184.216.34"), "")
	if err := open.CheckURL("https://anything.example/x"); err != nil {
		t.Errorf("with no allowlist any public host passes: %v", err)
	}
}

// AC-5, AC-8: a redirect is not followed and the target is never contacted; a normal call works.
func TestHTTP_DoesNotFollowRedirects_AndSendsIdempotencyKey(t *testing.T) {
	var targetHits int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&targetHits, 1)
		w.WriteHeader(200)
	}))
	defer target.Close()

	var gotKey string
	front := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("Idempotency-Key")
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, target.URL+"/secret", http.StatusFound)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer front.Close()

	h := newHTTPForTest(5*time.Second, front.Client().Transport.(*http.Transport).TLSClientConfig)
	resp, err := h.Call(context.Background(), Request{URL: front.URL + "/ok", Method: "POST", IdempotencyRef: "ref-1:act", Payload: map[string]any{"a": 1}})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("a normal call must work: %v %+v", err, resp)
	}
	if gotKey != "ref-1:act" {
		t.Errorf("Idempotency-Key = %q, want ref-1:act", gotKey)
	}

	resp, err = h.Call(context.Background(), Request{URL: front.URL + "/redirect", Method: "POST", Payload: map[string]any{}})
	if err == nil {
		t.Fatalf("a redirect must be reported as a failed call, got %+v", resp)
	}
	if hits := atomic.LoadInt32(&targetHits); hits != 0 {
		t.Fatalf("the redirect target was contacted %d time(s)", hits)
	}
}

// Production code must refuse loopback even though the test seam allows it.
func TestHTTP_ProductionClientRefusesLoopback(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	h := NewHTTP(2 * time.Second)
	if _, err := h.Call(context.Background(), Request{URL: srv.URL + "/x", Method: "POST", Payload: map[string]any{}}); !errors.Is(err, ErrBlockedURL) {
		t.Fatalf("the production client must refuse a loopback target, got %v", err)
	}
}

// Methods other than POST and PUT are refused; empty defaults to POST.
func TestHTTP_MethodRestriction(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Method))
	}))
	defer srv.Close()
	h := newHTTPForTest(2*time.Second, srv.Client().Transport.(*http.Transport).TLSClientConfig)
	for _, m := range []string{"DELETE", "GET", "PATCH", "CONNECT", "TRACE"} {
		if _, err := h.Call(context.Background(), Request{URL: srv.URL, Method: m, Payload: map[string]any{}}); !errors.Is(err, ErrBlockedURL) {
			t.Errorf("method %s must be refused, got %v", m, err)
		}
	}
	for _, m := range []string{"", "post", "PUT"} {
		resp, err := h.Call(context.Background(), Request{URL: srv.URL, Method: m, Payload: map[string]any{}})
		if err != nil {
			t.Errorf("method %q must work: %v", m, err)
			continue
		}
		want := strings.ToUpper(m)
		if want == "" {
			want = "POST"
		}
		if resp.Body != want {
			t.Errorf("method %q sent as %q, want %q", m, resp.Body, want)
		}
	}
}

// The guard must run inside Call, not only in the dialer: the dialer alone would let an
// http:// URL, credentials in the URL, or a host outside the allowlist through, because
// none of those involve an internal address.
func TestHTTP_CallEnforcesURLRulesBeforeAnyNetworkIO(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	h := newHTTPForTest(2*time.Second, nil)
	for _, u := range []string{srv.URL + "/x", "https://user:pw@example.com/x", "ftp://example.com/x"} {
		if _, err := h.Call(context.Background(), Request{URL: u, Method: "POST", Payload: map[string]any{}}); !errors.Is(err, ErrBlockedURL) {
			t.Errorf("Call(%q) = %v, want ErrBlockedURL", u, err)
		}
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatalf("a refused URL still reached the server %d time(s)", hits)
	}

	allow := newHTTP(2*time.Second, NewGuard(GuardOptions{AllowedHosts: "api.partner.com", allowLoopback: true}), nil)
	if _, err := allow.Call(context.Background(), Request{URL: "https://evil.example/x", Method: "POST", Payload: map[string]any{}}); !errors.Is(err, ErrBlockedURL) {
		t.Errorf("a host outside the allowlist must be refused by Call, got %v", err)
	}
}

var _ = tls.VersionTLS12

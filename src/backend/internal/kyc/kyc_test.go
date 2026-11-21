package kyc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

func TestClassifyThresholds(t *testing.T) {
	cases := []struct {
		score   int
		verdict Verdict
		status  store.KycStatus
	}{
		{100, VerdictAutoApprove, store.KycStatusAutoApproved},
		{85, VerdictAutoApprove, store.KycStatusAutoApproved},
		{84, VerdictManualReview, store.KycStatusManualReview},
		{50, VerdictManualReview, store.KycStatusManualReview},
		{49, VerdictAutoReject, store.KycStatusRejected},
		{0, VerdictAutoReject, store.KycStatusRejected},
	}
	for _, tc := range cases {
		v, s := Classify(tc.score)
		if v != tc.verdict || s != tc.status {
			t.Fatalf("Classify(%d) = %s/%s, want %s/%s", tc.score, v, s, tc.verdict, tc.status)
		}
	}
}

func TestMockScorerDeterministic(t *testing.T) {
	m := MockScorer{}
	a1, err := m.Score(context.Background(), []byte("card-A"), []byte("selfie-A"))
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := m.Score(context.Background(), []byte("card-A"), []byte("selfie-A"))
	if a1.Score != a2.Score {
		t.Fatalf("same images must yield same score: %d vs %d", a1.Score, a2.Score)
	}
	if a1.Score < 40 || a1.Score > 99 {
		t.Fatalf("mock score out of band: %d", a1.Score)
	}
	b, _ := m.Score(context.Background(), []byte("card-B"), []byte("selfie-B"))
	_ = b // different inputs may collide in score — only determinism is contractual
}

func TestParseScoreJSON(t *testing.T) {
	res, err := parseScoreJSON("Here you go:\n```json\n{\"score\": 72, \"reasons\": [\"ảnh rõ\"]}\n```")
	if err != nil || res.Score != 72 || len(res.Reasons) != 1 {
		t.Fatalf("parse: %v %+v", err, res)
	}
	if res, err := parseScoreJSON(`{"score": 150, "reasons": []}`); err != nil || res.Score != 100 {
		t.Fatalf("clamp high: %v %+v", err, res)
	}
	if _, err := parseScoreJSON("no json here"); err == nil {
		t.Fatal("want error for missing JSON")
	}
}

// TestClaudeScorerErrorClassification: mạng/5xx/parse lỗi → RetryableError
// (engine retry); 4xx → terminal.
func TestClaudeScorerErrorClassification(t *testing.T) {
	newServer := func(status int, body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
	}

	run := func(t *testing.T, status int, body string) error {
		srv := newServer(status, body)
		defer srv.Close()
		c := NewClaudeScorer("test-key")
		c.client = srv.Client()
		// point the request at the test server by rewriting via transport
		c.client.Transport = rewriteHost(srv.URL)
		_, err := c.Score(context.Background(), []byte("img1"), []byte("img2"))
		return err
	}

	if err := run(t, 500, "oops"); !workflow.IsRetryable(err) {
		t.Fatalf("5xx: want retryable, got %v", err)
	}
	if err := run(t, 429, "slow down"); !workflow.IsRetryable(err) {
		t.Fatalf("429: want retryable, got %v", err)
	}
	if err := run(t, 401, "bad key"); err == nil || workflow.IsRetryable(err) {
		t.Fatalf("401: want terminal error, got %v", err)
	}
	if err := run(t, 200, `{"content":[{"type":"text","text":"gibberish"}]}`); !workflow.IsRetryable(err) {
		t.Fatalf("bad model reply: want retryable, got %v", err)
	}
	if err := run(t, 200, `{"content":[{"type":"text","text":"{\"score\":91,\"reasons\":[\"ok\"]}"}]}`); err != nil {
		t.Fatalf("happy path: %v", err)
	}
}

type rewriteHost string

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	target := string(r)
	req.URL.Scheme = "http"
	req.URL.Host = target[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}

package api

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/fraud"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// Regression for the Audit Log page silently showing 3 hardcoded demo rows
// on a real, correct zero-match search. sqlc leaves ListAuditLogs' result as
// a nil slice when no rows match; encoding/json marshals nil as `null`, and
// hpay-admin's useAuditQueries.ts `if (res?.logs)` treated that null as "the
// call didn't work" and fell back to demo data. `[]` (empty, non-nil) is
// truthy in JS, so coercing nil to an empty slice before responding is the
// whole fix — nonNilAuditLogs mirrors exactly what ListAuditLogs does.
func TestNonNilAuditLogs_NilSliceMarshalsAsEmptyArrayNotNull(t *testing.T) {
	var nilLogs []store.AuditLog // exactly what sqlc leaves behind for zero rows

	out, err := json.Marshal(nonNilAuditLogs(nilLogs))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != "[]" {
		t.Fatalf("want \"[]\", got %s (this is exactly the null the frontend misread as failure)", out)
	}

	// A real non-empty slice must pass through unchanged.
	real := []store.AuditLog{{ID: 1, Action: "POLICY_UPDATE"}}
	out, err = json.Marshal(nonNilAuditLogs(real))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded []store.AuditLog
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Action != "POLICY_UPDATE" {
		t.Fatalf("want the real row preserved, got %v", decoded)
	}
}

// Regression for the Fraud Monitor page crash: fraud.FlagItem.Reasons is a
// raw []byte (the reasons jsonb column scanned unparsed by sqlc), and
// encoding/json marshals a bare []byte field as a base64 string — not as
// embedded JSON. Serializing fraud.FlagItem straight to the client sent
// hpay-admin an unreadable base64 blob where it expected a JSON array,
// crashing the whole page. toFraudFlagResponse must re-type Reasons as
// json.RawMessage so it's embedded verbatim.
func TestToFraudFlagResponse_ReasonsEmbedsAsJSONArrayNotBase64(t *testing.T) {
	item := fraud.FlagItem{
		ID:            uuid.New(),
		TransactionID: uuid.New(),
		RefCode:       "TXN123",
		RuleCode:      "VELOCITY",
		Score:         72,
		Reasons:       []byte(`["velocity_spike","new_receiver"]`),
		Status:        store.FlagStatusOpen,
	}

	out, err := json.Marshal(toFraudFlagResponse([]fraud.FlagItem{item})[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		Reasons []string `json:"reasons"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("client-side decode of the response must succeed with reasons as a real string array, got %s: %v", out, err)
	}
	if len(decoded.Reasons) != 2 || decoded.Reasons[0] != "velocity_spike" || decoded.Reasons[1] != "new_receiver" {
		t.Fatalf("want [\"velocity_spike\",\"new_receiver\"], got %v", decoded.Reasons)
	}

	// Belt-and-suspenders: prove the naive (buggy) path really does produce
	// an undecodable base64 string for the same input, so this test would
	// have caught the original bug.
	naive, _ := json.Marshal(item)
	var naiveDecoded struct {
		Reasons []string `json:"reasons"`
	}
	if json.Unmarshal(naive, &naiveDecoded) == nil {
		t.Fatalf("expected the naive []byte serialization to fail decoding as []string (base64 string, not an array) — got %v", naiveDecoded.Reasons)
	}
}

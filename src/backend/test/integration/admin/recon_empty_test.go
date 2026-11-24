package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// "No reconciliation has run for this date" is a normal answer, not an error. The admin page opens on
// today, where there is usually no run yet; a 404 put a red error in the browser console on every visit.
func TestReconGet_NoRunForTheDateIsAnEmptyAnswer(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")

	w := rig.Call("GET", "/v1/admin/recon/runs/1999-01-02", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("a date with no run must answer 200 with an empty result, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Run   json.RawMessage `json:"run"`
		Items []any           `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if string(res.Run) != "null" {
		t.Fatalf("run must be null when there is none, got %s", res.Run)
	}
	if res.Items == nil || len(res.Items) != 0 || res.Total != 0 {
		t.Fatalf("items must be an empty list, never null: %+v", res)
	}
}

func TestReconGet_BadDateStillRefused(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	if w := rig.Call("GET", "/v1/admin/recon/runs/not-a-date", tok, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

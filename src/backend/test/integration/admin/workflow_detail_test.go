package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// AUD-006: the workflow detail page showed "—" for the transaction and the amount although the list
// showed them, because the detail query did not join the transaction.
func TestWorkflowDetail_CarriesTheTransactionAndTheDefinition(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	ref, _, _, _, _ := rig.E.CompletedTransfer(t, 308)

	var instID string
	if err := rig.E.Pool.QueryRow(t.Context(), `
		SELECT wi.id FROM workflow_instances wi JOIN transactions tx ON tx.id = wi.transaction_id
		WHERE tx.ref_code = $1`, ref).Scan(&instID); err != nil {
		t.Fatalf("a completed transfer has a workflow instance: %v", err)
	}

	w := rig.Call("GET", "/v1/admin/workflows/"+instID, tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Instance struct {
			ID                string `json:"id"`
			DefinitionName    string `json:"definition_name"`
			DefinitionVersion int    `json:"definition_version"`
			TxnRef            string `json:"txn_ref"`
			TxnAmount         int64  `json:"txn_amount"`
		} `json:"instance"`
		Steps []struct {
			StepName string `json:"step_name"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Instance.TxnRef != ref || res.Instance.TxnAmount != 308 {
		t.Fatalf("want the transaction %s / 308, got %q / %d", ref, res.Instance.TxnRef, res.Instance.TxnAmount)
	}
	if res.Instance.DefinitionName == "" || res.Instance.DefinitionVersion < 1 {
		t.Fatalf("definition missing: %+v", res.Instance)
	}
	if len(res.Steps) == 0 {
		t.Fatal("a completed transfer ran steps")
	}
}

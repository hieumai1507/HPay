package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// AUD-001: the offer detail page showed `undefined` everywhere and an empty authentication matrix
// because the API sent the matrix nowhere. The detail now carries each service's tiers.
func TestOfferDetail_CarriesTheAuthMatrixOfEachService(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")

	const l1 = "b0000000-0000-0000-0000-000000000001" // OFFER_CUST_L1, seeded by the migrations
	w := rig.Call("GET", "/v1/admin/offers/"+l1, tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Offer    struct{ Code string } `json:"offer"`
		Services []struct {
			ServiceCode string `json:"service_code"`
			ServiceName string `json:"service_name"`
			Daily       int64  `json:"daily_limit"`
			AuthMatrix  []struct {
				MinAmount  int64  `json:"min_amount"`
				MaxAmount  *int64 `json:"max_amount"`
				AuthMethod string `json:"auth_method"`
			} `json:"auth_matrix"`
		} `json:"services"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Offer.Code != "OFFER_CUST_L1" || len(res.Services) == 0 {
		t.Fatalf("unexpected offer detail: %s", w.Body.String())
	}
	var p2p bool
	for _, s := range res.Services {
		if s.ServiceCode == "" || s.ServiceName == "" {
			t.Fatalf("service without code or name: %+v", s)
		}
		if s.AuthMatrix == nil {
			t.Fatalf("%s: auth_matrix must be a list, never null", s.ServiceCode)
		}
		if s.ServiceCode != "P2P_TRANSFER" {
			continue
		}
		p2p = true
		if len(s.AuthMatrix) != 3 {
			t.Fatalf("P2P_TRANSFER has pin, otp and 2fa tiers: got %d", len(s.AuthMatrix))
		}
		methods := []string{s.AuthMatrix[0].AuthMethod, s.AuthMatrix[1].AuthMethod, s.AuthMatrix[2].AuthMethod}
		if methods[0] != "pin" || methods[1] != "otp" || methods[2] != "2fa" {
			t.Fatalf("tiers must be ordered by amount: %v", methods)
		}
		if s.AuthMatrix[0].MinAmount != 0 || s.AuthMatrix[2].MaxAmount != nil {
			t.Fatalf("first tier starts at 0 and the last has no upper bound: %+v", s.AuthMatrix)
		}
	}
	if !p2p {
		t.Fatal("OFFER_CUST_L1 offers P2P_TRANSFER")
	}
}

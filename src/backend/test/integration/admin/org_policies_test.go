package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// The Organizations & Policy page used to show policy text baked into the UI. It now reads the
// organization's real policy versions from this route.
func TestOrgPolicies_ListsTheRealVersions(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	f := testkit.NewOrgFixture(t, rig.E, 1_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	w := rig.Call("GET", "/v1/admin/orgs/"+f.OrgID.String()+"/policies", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Policies []struct {
			ID       string          `json:"id"`
			OrgID    string          `json:"org_id"`
			Name     string          `json:"name"`
			Version  int             `json:"version"`
			IsActive bool            `json:"is_active"`
			Tiers    json.RawMessage `json:"tiers"`
		} `json:"policies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Policies) == 0 {
		t.Fatal("the fixture created a policy: the list must contain it")
	}
	p := res.Policies[0]
	if p.OrgID != f.OrgID.String() || !p.IsActive || p.Version < 1 || len(p.Tiers) == 0 {
		t.Fatalf("unexpected policy row: %+v", p)
	}
}

func TestOrgPolicies_OnlyShowsThisOrganization(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	a := testkit.NewOrgFixture(t, rig.E, 1_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})
	b := testkit.NewOrgFixture(t, rig.E, 1_000_000, []int{1}, []approval.Tier{{Level: 1, RequiredApprovals: 1}})

	w := rig.Call("GET", "/v1/admin/orgs/"+a.OrgID.String()+"/policies", tok, nil)
	var res struct {
		Policies []struct {
			OrgID string `json:"org_id"`
		} `json:"policies"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Policies {
		if p.OrgID == b.OrgID.String() {
			t.Fatalf("organization %s leaked a policy of organization %s", a.OrgID, b.OrgID)
		}
	}
}

func TestOrgPolicies_BadIDAndUnknownOrg(t *testing.T) {
	rig := testkit.NewAdminRig(t)
	_, tok := rig.NewAdmin("ops", "active")
	if w := rig.Call("GET", "/v1/admin/orgs/not-a-uuid/policies", tok, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id: want 400, got %d", w.Code)
	}
	w := rig.Call("GET", "/v1/admin/orgs/00000000-0000-0000-0000-000000000000/policies", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("unknown org: want an empty list (200), got %d", w.Code)
	}
}

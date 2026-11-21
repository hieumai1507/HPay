package approval

import "testing"

func snap(tiers ...Tier) PolicySnapshot { return PolicySnapshot{Tiers: tiers} }

func TestEvaluate(t *testing.T) {
	oneL1 := snap(Tier{Level: 1, RequiredApprovals: 1})
	twoTier := snap(Tier{Level: 1, RequiredApprovals: 1}, Tier{Level: 2, RequiredApprovals: 1})
	dualL1 := snap(Tier{Level: 1, RequiredApprovals: 2})

	cases := []struct {
		name      string
		snap      PolicySnapshot
		actions   []ActionView
		want      DecisionStatus
		wantLevel int
	}{
		{"no actions -> pending at level 1", oneL1, nil, DecisionPending, 1},
		{"one approve fills single tier", oneL1, []ActionView{{1, true}}, DecisionApproved, 0},
		{"reject is final regardless of approvals", twoTier,
			[]ActionView{{1, true}, {2, false}}, DecisionRejected, 0},
		{"tier 1 done -> pending at tier 2", twoTier,
			[]ActionView{{1, true}}, DecisionPending, 2},
		{"both tiers done -> approved", twoTier,
			[]ActionView{{1, true}, {2, true}}, DecisionApproved, 0},
		{"needs 2 signatures at level 1", dualL1,
			[]ActionView{{1, true}}, DecisionPending, 1},
		{"2 of 2 at level 1 -> approved", dualL1,
			[]ActionView{{1, true}, {1, true}}, DecisionApproved, 0},
		{"unsorted tiers still evaluate ascending",
			snap(Tier{Level: 2, RequiredApprovals: 1}, Tier{Level: 1, RequiredApprovals: 1}),
			[]ActionView{{1, true}}, DecisionPending, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Evaluate(tc.snap, tc.actions)
			if d.Status != tc.want {
				t.Fatalf("status = %s, want %s", d.Status, tc.want)
			}
			if tc.want == DecisionPending && d.CurrentLevel != tc.wantLevel {
				t.Fatalf("current_level = %d, want %d", d.CurrentLevel, tc.wantLevel)
			}
		})
	}
}

func TestValidateTiers(t *testing.T) {
	bad := [][]Tier{
		{},
		{{Level: 0, RequiredApprovals: 1}},
		{{Level: 1, RequiredApprovals: 0}},
		{{Level: 1, RequiredApprovals: 1}, {Level: 1, RequiredApprovals: 2}},
	}
	for i, tiers := range bad {
		if err := ValidateTiers(tiers); err == nil {
			t.Fatalf("case %d: expected error for %+v", i, tiers)
		}
	}
	if err := ValidateTiers([]Tier{{Level: 1, RequiredApprovals: 1}, {Level: 2, RequiredApprovals: 2}}); err != nil {
		t.Fatalf("valid tiers rejected: %v", err)
	}
}

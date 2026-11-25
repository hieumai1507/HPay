package fraud

import "testing"

func TestEvaluateAndDecide(t *testing.T) {
	cases := []struct {
		name      string
		in        Input
		wantScore int
		want      Decision
	}{
		{"clean", Input{Amount: 400, Avg30dAmount: 400, TxnsInLast5Min: 1, HourOfDay: 14}, 0, DecisionPass},
		{"new receiver only", Input{Amount: 400, TxnsInLast5Min: 1, IsNewReceiver: true, HourOfDay: 14}, 20, DecisionPass},
		{"velocity + new receiver = flag", Input{Amount: 400, TxnsInLast5Min: 4, IsNewReceiver: true, HourOfDay: 14}, 50, DecisionFlag},
		{"spike + odd hours = flag boundary", Input{Amount: 20_000, Avg30dAmount: 4_000, TxnsInLast5Min: 1, HourOfDay: 2}, 55, DecisionFlag},
		{"all four = block, capped", Input{Amount: 40_000, Avg30dAmount: 2_000, TxnsInLast5Min: 5, IsNewReceiver: true, HourOfDay: 3}, 100, DecisionBlock},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			signals, score := Evaluate(tc.in)
			if score != tc.wantScore {
				t.Fatalf("score = %d, want %d (signals=%v)", score, tc.wantScore, signals)
			}
			if d := Decide(score); d != tc.want {
				t.Fatalf("decision = %s, want %s", d, tc.want)
			}
		})
	}
}

func TestDecideBoundaries(t *testing.T) {
	if Decide(39) != DecisionPass {
		t.Fatal("39 must pass")
	}
	if Decide(40) != DecisionFlag {
		t.Fatal("40 must flag")
	}
	if Decide(70) != DecisionFlag {
		t.Fatal("70 must flag (block is strictly above)")
	}
	if Decide(71) != DecisionBlock {
		t.Fatal("71 must block")
	}
}

func TestAmountSpikeNeedsHistory(t *testing.T) {
	// No 30-day history -> no spike rule even for a huge amount.
	_, score := Evaluate(Input{Amount: 4_000_000, Avg30dAmount: 0, TxnsInLast5Min: 1, HourOfDay: 14})
	if score != 0 {
		t.Fatalf("spike must not fire without history, got score %d", score)
	}
}

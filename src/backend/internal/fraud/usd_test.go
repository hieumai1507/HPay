package fraud

import "testing"

func TestUSD_WritesCentsAsDollars(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{0, "$0.00"}, {1, "$0.01"}, {5, "$0.05"}, {99, "$0.99"}, {100, "$1.00"},
		{4000, "$40.00"}, {20000, "$200.00"}, {123456, "$1234.56"}, {-250, "-$2.50"},
	}
	for _, c := range cases {
		if got := usd(c.cents); got != c.want {
			t.Errorf("usd(%d) = %q, want %q", c.cents, got, c.want)
		}
	}
}

func TestAmountSpikeEvidenceNamesDollars(t *testing.T) {
	signals, _ := Evaluate(Input{Amount: 20000, Avg30dAmount: 4000, HourOfDay: 12})
	for _, s := range signals {
		if s.Code == "AMOUNT_SPIKE" {
			want := "amount $200.00 is 5 times the 30-day average ($40.00)"
			if s.Evidence != want {
				t.Fatalf("evidence = %q, want %q", s.Evidence, want)
			}
			return
		}
	}
	t.Fatal("AMOUNT_SPIKE signal not produced")
}

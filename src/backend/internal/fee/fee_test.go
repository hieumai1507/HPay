package fee

import "testing"

func TestCalc(t *testing.T) {
	cases := []struct {
		name   string
		p      Policy
		amount int64
		want   int64
	}{
		{"percent only", Policy{PercentBP: 10}, 400_000, 400},
		{"clamped to max", Policy{PercentBP: 10, MaxFee: 100, HasMax: true}, 400_000, 100},
		{"raised to min", Policy{PercentBP: 10, MinFee: 5}, 1_000, 5},
		{"fixed plus percent", Policy{FixedFee: 25, PercentBP: 25}, 400_000, 1_025},
		{"free", Policy{}, 20_000, 0},
		{"never negative", Policy{FixedFee: -5}, 100, 0},
	}
	for _, c := range cases {
		if got := Calc(c.p, c.amount); got != c.want {
			t.Errorf("%s: want %d got %d", c.name, c.want, got)
		}
	}
}

// AC-6: a percentage fee is amount * bp / 10,000 rounded down to a whole cent.
func TestCalc_PercentRoundsDownToTheCent(t *testing.T) {
	p := Policy{PercentBP: 25}
	cases := []struct{ amount, want int64 }{
		{1, 0}, {99, 0}, {399, 0}, {400, 1}, {10_000, 25}, {10_001, 25}, {10_399, 25}, {10_400, 26},
	}
	for _, c := range cases {
		if got := Calc(p, c.amount); got != c.want {
			t.Errorf("Calc(25bp, %d) = %d, want %d", c.amount, got, c.want)
		}
	}
}

// AC-6: the fixed P2P fee is 25 cents whatever the amount.
func TestCalc_FixedFee25IsFlat(t *testing.T) {
	p := Policy{FixedFee: 25}
	for _, amount := range []int64{1, 99, 10_000, 1_000_000} {
		if got := Calc(p, amount); got != 25 {
			t.Errorf("Calc(fixed 25, %d) = %d, want 25", amount, got)
		}
	}
}
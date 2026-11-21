package fee

import "testing"

func TestCalc(t *testing.T) {
	cases := []struct {
		name   string
		p      Policy
		amount int64
		want   int64
	}{
		{"percent only", Policy{PercentBP: 10}, 1_000_000, 1_000},
		{"clamped to max", Policy{PercentBP: 10, MaxFee: 10_000, HasMax: true}, 100_000_000, 10_000},
		{"raised to min", Policy{PercentBP: 10, MinFee: 500}, 10_000, 500},
		{"fixed plus percent", Policy{FixedFee: 1_000, PercentBP: 25}, 1_000_000, 3_500},
		{"free", Policy{}, 5_000_000, 0},
		{"never negative", Policy{FixedFee: -5}, 100, 0},
	}
	for _, c := range cases {
		if got := Calc(c.p, c.amount); got != c.want {
			t.Errorf("%s: want %d got %d", c.name, c.want, got)
		}
	}
}

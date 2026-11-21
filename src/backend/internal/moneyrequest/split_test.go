package moneyrequest

import "testing"

func sum(xs []int64) int64 {
	var s int64
	for _, x := range xs {
		s += x
	}
	return s
}

func TestSplitEven(t *testing.T) {
	cases := []struct {
		total int64
		n     int
		want  []int64
	}{
		{100_000, 3, []int64{33_334, 33_333, 33_333}},
		{100_000, 4, []int64{25_000, 25_000, 25_000, 25_000}},
		{1, 1, []int64{1}},
		{10, 3, []int64{4, 3, 3}},
	}
	for _, tc := range cases {
		got, err := SplitEven(tc.total, tc.n)
		if err != nil {
			t.Fatalf("SplitEven(%d,%d): %v", tc.total, tc.n, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("SplitEven(%d,%d) = %v, want %v", tc.total, tc.n, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("SplitEven(%d,%d)[%d] = %d, want %d", tc.total, tc.n, i, got[i], tc.want[i])
			}
		}
		if s := sum(got); s != tc.total {
			t.Fatalf("SplitEven(%d,%d) sums to %d, want %d", tc.total, tc.n, s, tc.total)
		}
	}
}

func TestSplitEvenRejectsTooManyShares(t *testing.T) {
	if _, err := SplitEven(2, 3); err == nil {
		t.Fatal("want error when total < n (some share would be non-positive)")
	}
	if _, err := SplitEven(100, 0); err == nil {
		t.Fatal("want error for n=0")
	}
}

func TestValidateCustomSplit(t *testing.T) {
	if err := ValidateCustomSplit([]int64{50_000, 50_000}, 100_000); err != nil {
		t.Fatalf("valid split rejected: %v", err)
	}
	if err := ValidateCustomSplit([]int64{50_000, 40_000}, 100_000); err == nil {
		t.Fatal("want error when amounts don't sum to total")
	}
	if err := ValidateCustomSplit([]int64{100_000, 0}, 100_000); err == nil {
		t.Fatal("want error for a zero share")
	}
	if err := ValidateCustomSplit(nil, 0); err == nil {
		t.Fatal("want error for empty amounts")
	}
}

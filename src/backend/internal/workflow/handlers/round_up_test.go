package handlers

import "testing"

// AC-8: the round-up is the distance to the next multiple of the unit, in cents.
func TestRoundUpDiff(t *testing.T) {
	cases := []struct{ amount, unit, want int64 }{
		{1_234, 100, 66},
		{1_200, 100, 0},
		{1, 100, 99},
		{99, 100, 1},
		{100, 100, 0},
		{1_234, 500, 266},
	}
	for _, c := range cases {
		if got := roundUpDiff(c.amount, c.unit); got != c.want {
			t.Errorf("roundUpDiff(%d, %d) = %d, want %d", c.amount, c.unit, got, c.want)
		}
	}
}

// AC-8: ROUND_UP_UNIT sets the unit, default 100 cents; the old VND name is gone.
func TestRoundUpUnitFromEnv(t *testing.T) {
	t.Setenv("ROUND_UP_UNIT", "")
	t.Setenv("ROUND_UP_UNIT_VND", "")
	if got := roundUpUnitFromEnv(); got != 100 {
		t.Errorf("default unit = %d, want 100", got)
	}
	t.Setenv("ROUND_UP_UNIT", "500")
	if got := roundUpUnitFromEnv(); got != 500 {
		t.Errorf("ROUND_UP_UNIT=500 gave %d", got)
	}
	t.Setenv("ROUND_UP_UNIT", "")
	t.Setenv("ROUND_UP_UNIT_VND", "7")
	if got := roundUpUnitFromEnv(); got != 100 {
		t.Errorf("the old env name must be ignored, got %d", got)
	}
	t.Setenv("ROUND_UP_UNIT_VND", "")
	for _, bad := range []string{"0", "-5", "abc"} {
		t.Setenv("ROUND_UP_UNIT", bad)
		if got := roundUpUnitFromEnv(); got != 100 {
			t.Errorf("ROUND_UP_UNIT=%q gave %d, want the default 100", bad, got)
		}
	}
}

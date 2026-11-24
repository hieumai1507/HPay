package user

import "testing"

// Formats here are LOCKED by the m3-api-support plan — mobile and admin both
// render these strings verbatim. Changing them is an API change (openapi.yaml).
func TestMaskPhone(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0912345678", "091****678"},
		{"0912345", "091*345"},
		{"091234", "******"},
		{"", ""},
	}
	for _, c := range cases {
		if got := MaskPhone(c.in); got != c.want {
			t.Errorf("MaskPhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMaskName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Nguyễn Văn An", "Nguyễn V** A*"},
		{"Trần Bình", "Trần B***"},
		{"Madonna", "M******"},
		{"  Lê   Thị  Hồng  ", "Lê T** H***"},
		{"", ""},
		{"A B", "A B"}, // single-rune words stay as-is (nothing left to mask)
	}
	for _, c := range cases {
		if got := MaskName(c.in); got != c.want {
			t.Errorf("MaskName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

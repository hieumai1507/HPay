package wallet

import (
	"strings"
	"testing"
)

func TestNewRefCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		rc := NewRefCode()
		if !strings.HasPrefix(rc, "MP") || len(rc) != 14 {
			t.Fatalf("bad ref code %q", rc)
		}
		if seen[rc] {
			t.Fatalf("duplicate ref code %q", rc)
		}
		seen[rc] = true
	}
}

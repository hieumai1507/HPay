package auth

import (
	"testing"
	"time"
)

func TestDecideRotation(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name      string
		used, rev *time.Time
		expires   time.Time
		want      RotationDecision
	}{
		{"ok", nil, nil, future, RotationOK},
		{"reused -> theft", &past, nil, future, RotationReused},
		{"revoked wins over reused", &past, &past, future, RotationRevoked},
		{"expired", nil, nil, past, RotationExpired},
	}
	for _, c := range cases {
		if got := DecideRotation(c.used, c.rev, c.expires, now); got != c.want {
			t.Errorf("%s: want %v got %v", c.name, c.want, got)
		}
	}
}

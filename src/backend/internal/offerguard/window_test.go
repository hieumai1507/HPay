package offerguard

import (
	"testing"
	"time"
)

// C-12: day and month windows follow Vietnam time (UTC+7), not the server zone.
func TestWindowStarts_UseVietnamTime(t *testing.T) {
	// 18:30 UTC on 30 Nov is 01:30 on 1 Dec in Vietnam: a new day AND a new month there.
	now := time.Date(2026, 11, 30, 18, 30, 0, 0, time.UTC)
	day, month := windowStarts(now)

	wantDay := time.Date(2026, 11, 30, 17, 0, 0, 0, time.UTC)   // 1 Dec 00:00 ICT
	wantMonth := time.Date(2026, 11, 30, 17, 0, 0, 0, time.UTC) // 1 Dec 00:00 ICT
	if !day.Equal(wantDay) {
		t.Fatalf("day start: want %s, got %s", wantDay, day.UTC())
	}
	if !month.Equal(wantMonth) {
		t.Fatalf("month start: want %s, got %s", wantMonth, month.UTC())
	}

	// 16:00 UTC is 23:00 the same Vietnam day: still the old window.
	now = time.Date(2026, 11, 30, 16, 0, 0, 0, time.UTC)
	day, month = windowStarts(now)
	if want := time.Date(2026, 11, 29, 17, 0, 0, 0, time.UTC); !day.Equal(want) {
		t.Fatalf("day start: want %s, got %s", want, day.UTC())
	}
	if want := time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC); !month.Equal(want) {
		t.Fatalf("month start: want %s, got %s", want, month.UTC())
	}
}

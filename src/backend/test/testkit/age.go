package testkit

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// AgeUserTransactions moves a user's existing transactions into the past so the interval throttle
// (interval_seconds) no longer sees them. A transaction is measured from started_at, falling back
// to created_at (BUG-0017), so both columns must move; moving only created_at leaves started_at
// at "just now" and the throttle would still fire.
func AgeUserTransactions(t *testing.T, e *Env, userID uuid.UUID, interval string) {
	t.Helper()
	if _, err := e.Pool.Exec(context.Background(),
		`UPDATE transactions SET created_at = created_at - $2::interval, started_at = started_at - $2::interval WHERE initiated_by = $1`,
		userID, interval); err != nil {
		t.Fatal(err)
	}
}

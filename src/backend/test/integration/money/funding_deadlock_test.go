package money

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// Load-test finding P6: concurrent funding from the shared SYSTEM_CASH_IN
// wallet must never deadlock (SQLSTATE 40P01). ledger.Post locks every wallet
// of a posting in one ORDER BY id call, so many top-ups plus transfers on the
// same system wallet queue instead of forming a cycle.
func TestFunding_ConcurrentTopupsNeverDeadlock(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()

	const n = 24
	users := make([]uuid.UUID, n)
	for i := range users {
		u, _, _ := e.UserWithMoney(t, 0)
		users[i] = u
	}
	_, _, phone := e.UserWithMoney(t, 0)
	rich, _, _ := e.UserWithMoney(t, 2_000_000)

	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	for _, u := range users {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := e.Wsvc.Topup(ctx, u, 50_000, uuid.NewString())
			errs <- err
		}()
		go func() {
			defer wg.Done()
			_, err := e.Svc.Transfer(ctx, rich, phone, 1_000, "dl", uuid.NewString())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "40P01" {
			t.Fatalf("deadlock while funding concurrently: %v", err)
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	e.Drain(t)
}

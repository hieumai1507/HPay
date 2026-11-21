package auth

// Login lockout and constant work (BE-H8, BUG-0016, constitution C-07/C-13).
//
// Five consecutive wrong passwords lock the account for 15 minutes; a correct password is
// refused while the lock lasts. The failure counter is one atomic SQL update, so parallel
// guesses cannot slip past it. An unknown phone or e-mail runs the same argon2id work as a
// known one, so response time does not reveal which accounts exist.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// ErrLoginLocked is returned while an account is locked after too many failed logins.
var ErrLoginLocked = errors.New("too many failed logins; try again later")

const loginMaxFailures = 5

var (
	dummyOnce sync.Once
	dummyHash string
)

// burnPasswordWork spends the cost of one password verification when there is no real
// hash to check (unknown account). The result is always ignored.
func burnPasswordWork(password string) {
	dummyOnce.Do(func() { dummyHash, _ = HashPassword("not-a-real-password") })
	_ = VerifyPassword(dummyHash, password)
}

func lockActive(until time.Time, valid bool) bool { return valid && until.After(time.Now()) }

// auditLogin writes a trail entry that never contains the password.
func auditLogin(ctx context.Context, q *store.Queries, actorID uuid.UUID, actorType, action string, attempts int32) {
	_ = q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: actorID, ActorType: actorType, Action: action,
		Resource: actorType + " " + actorID.String(),
		Reason:   pgtype.Text{String: fmt.Sprintf("failed attempt %d of %d", attempts, loginMaxFailures), Valid: true},
	})
}

func (s *Service) auditLogin(ctx context.Context, id uuid.UUID, actorType, action string, attempts int32) {
	auditLogin(ctx, s.q, id, actorType, action, attempts)
}

func (s *AdminService) auditLogin(ctx context.Context, id uuid.UUID, actorType, action string, attempts int32) {
	auditLogin(ctx, s.q, id, actorType, action, attempts)
}

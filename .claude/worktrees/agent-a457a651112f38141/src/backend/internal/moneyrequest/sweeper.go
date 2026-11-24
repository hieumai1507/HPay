package moneyrequest

import (
	"context"
	"log/slog"
	"time"
)

const sweepInterval = 30 * time.Second

// Sweeper expires overdue money requests. Same claim shape as the F6 approval
// sweeper: each row is finalized inside its own guarded transaction, so two
// sweepers (or a sweeper racing Pay()) never double-fire.
type Sweeper struct {
	svc *Service
	log *slog.Logger
}

func NewSweeper(svc *Service, log *slog.Logger) *Sweeper {
	return &Sweeper{svc: svc, log: log}
}

func (sw *Sweeper) Run(ctx context.Context) {
	t := time.NewTicker(sweepInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n, err := sw.svc.ExpireDue(ctx)
			if err != nil && ctx.Err() == nil {
				sw.log.Warn("money request expiry sweep failed; will retry", "err", err)
			}
			if n > 0 {
				sw.log.Info("expired overdue money requests", "count", n)
			}
		}
	}
}

// ExpireDue finalizes overdue pending requests (E19's other half: a request
// nobody pays gets cleaned up by the sweeper, not left pending forever).
func (s *Service) ExpireDue(ctx context.Context) (int, error) {
	ids, err := s.q.ListDueMoneyRequestIDs(ctx, 50)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		affected, err := s.q.ExpireMoneyRequest(ctx, id)
		if err != nil {
			return n, err
		}
		if affected > 0 {
			n++
		}
	}
	return n, nil
}

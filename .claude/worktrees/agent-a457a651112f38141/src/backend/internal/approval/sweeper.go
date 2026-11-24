package approval

import (
	"context"
	"log/slog"
	"time"
)

const sweepInterval = 30 * time.Second

// Sweeper expires overdue approval requests (E10). It runs under the worker
// role next to the workflow recovery loop.
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
				sw.log.Warn("approval expiry sweep failed; will retry", "err", err)
			}
			if n > 0 {
				sw.log.Info("expired overdue approval requests", "count", n)
			}
		}
	}
}

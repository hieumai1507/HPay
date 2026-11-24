package recon

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// Cron runs the daily reconciliation for the previous day at ~00:15, then every
// 24h. A missed/duplicate tick is harmless: run_date UNIQUE makes RunForDate
// idempotent.
type Cron struct {
	svc *Service
	log *slog.Logger
}

func NewCron(svc *Service, log *slog.Logger) *Cron {
	return &Cron{svc: svc, log: log}
}

func (c *Cron) Run(ctx context.Context) {
	for {
		next := nextRunAt(time.Now())
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			day := time.Now().AddDate(0, 0, -1)
			if _, err := c.svc.RunForDate(ctx, day); err != nil {
				if errors.Is(err, ErrAlreadyRan) {
					c.log.Debug("recon already ran", "date", day.Format("2006-01-02"))
				} else if ctx.Err() == nil {
					c.log.Warn("daily recon failed", "date", day.Format("2006-01-02"), "err", err)
				}
			} else {
				c.log.Info("daily recon complete", "date", day.Format("2006-01-02"))
			}
		}
	}
}

// nextRunAt returns the next 00:15 strictly after now.
func nextRunAt(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 15, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

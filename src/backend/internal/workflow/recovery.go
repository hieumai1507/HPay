package workflow

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

const (
	stuckThreshold  = 2 * time.Minute
	pendingMinIdle  = 60 * time.Second
	recoverInterval = 30 * time.Second
	sweepBatch      = 20
)

// Recovery makes sure nothing hangs forever (E20 + processing_async story):
//  1. XAUTOCLAIM: stream messages whose consumer died get reprocessed.
//  2. DB sweep: running instances untouched past the threshold are leased
//     (status -> recovering, the UPDATE itself is the claim) and re-emitted.
type Recovery struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	rdb    *redis.Client
	reg    *Registry
	engine *Engine
	worker *Worker
	log    *slog.Logger
}

func NewRecovery(pool *pgxpool.Pool, rdb *redis.Client, reg *Registry, engine *Engine, worker *Worker, log *slog.Logger) *Recovery {
	return &Recovery{pool: pool, q: store.New(pool), rdb: rdb, reg: reg, engine: engine, worker: worker, log: log}
}

func (r *Recovery) Run(ctx context.Context) {
	t := time.NewTicker(recoverInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.autoClaim(ctx)
			if err := r.SweepStuck(ctx); err != nil && ctx.Err() == nil {
				r.log.Warn("stuck sweep failed", "err", err)
			}
		}
	}
}

func (r *Recovery) autoClaim(ctx context.Context) {
	streams := []struct{ stream, group string }{{EventsStream, groupEngine}}
	for _, h := range r.reg.Names() {
		streams = append(streams, struct{ stream, group string }{StepStream(h), groupWorkers})
	}
	for _, sg := range streams {
		msgs, _, err := r.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream: sg.stream, Group: sg.group, Consumer: consumerName,
			MinIdle: pendingMinIdle, Start: "0", Count: 20,
		}).Result()
		if err != nil || len(msgs) == 0 {
			continue
		}
		r.log.Info("autoclaim reprocessing dead-consumer messages", "stream", sg.stream, "count", len(msgs))
		for _, m := range msgs {
			process := r.worker.processStepMessage
			if sg.stream == EventsStream {
				process = r.worker.processResultMessage
			}
			r.worker.handleMessage(ctx, sg.stream, sg.group, m, process)
		}
	}
}

// SweepStuck is exported for the E20 test: two concurrent sweeps must not
// double-claim thanks to the UPDATE lease + SKIP LOCKED.
func (r *Recovery) SweepStuck(ctx context.Context) error {
	rows, err := r.q.ClaimStuckInstances(ctx, store.ClaimStuckInstancesParams{
		UpdatedAt: pgtype.Timestamptz{Time: time.Now().Add(-stuckThreshold), Valid: true},
		Limit:     sweepBatch,
	})
	if err != nil {
		return err
	}
	for _, inst := range rows {
		r.log.Info("recovering stuck instance", "instance", inst.ID, "step", inst.CurrentStep.String)
		if err := r.engine.ReEmitCurrentStep(ctx, inst.ID); err != nil {
			r.log.Error("re-emit failed", "instance", inst.ID, "err", err)
		}
	}
	return nil
}

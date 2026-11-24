// Package outbox publishes committed events to Redis Streams. The relay is the
// ONLY writer to streams — business code writes outbox rows in its own DB tx
// (docs f4 §1: events never publish before commit, never get lost after).
package outbox

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

type Relay struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
	log  *slog.Logger
}

func NewRelay(pool *pgxpool.Pool, rdb *redis.Client, log *slog.Logger) *Relay {
	return &Relay{pool: pool, rdb: rdb, log: log}
}

const relayBatch = 100

// Run polls until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// A full batch means more is waiting: relay again at once instead
			// of idling until the next tick (caps throughput at batch/tick).
			for {
				n, err := r.RelayOnce(ctx, relayBatch)
				if err != nil {
					if ctx.Err() == nil {
						r.log.Warn("outbox relay pass failed; will retry", "err", err) // E17: keep going
					}
					break
				}
				if n < relayBatch || ctx.Err() != nil {
					break
				}
			}
		}
	}
}

// RelayOnce claims one batch, publishes, and marks published. Exported so
// tests can drive the relay deterministically (E05).
func (r *Relay) RelayOnce(ctx context.Context, batch int32) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := store.New(tx)

	rows, err := q.ClaimOutboxBatch(ctx, batch)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, tx.Commit(ctx)
	}

	published := make([]int64, 0, len(rows))
	for _, row := range rows {
		stream := destinationStream(row.EventType, row.Payload)
		if err := r.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: stream,
			Values: map[string]any{"type": row.EventType, "payload": string(row.Payload)},
		}).Err(); err != nil {
			// Redis down: stop here; unpublished rows stay claimed only within
			// this tx, which we roll back — nothing is lost (E05/E17).
			r.log.Warn("xadd failed; keeping outbox rows", "stream", stream, "err", err)
			break
		}
		published = append(published, row.ID)
	}
	if len(published) == 0 {
		return 0, tx.Rollback(ctx)
	}
	if err := q.MarkOutboxPublished(ctx, published); err != nil {
		return 0, err
	}
	return len(published), tx.Commit(ctx)
}

// destinationStream routes events: step.start goes to the handler's stream,
// everything else (results, approval.finalized in F6) to wf:events.
func destinationStream(eventType string, payload []byte) string {
	if eventType == workflow.EventStepStart {
		var p struct {
			Handler string `json:"handler"`
		}
		if json.Unmarshal(payload, &p) == nil && p.Handler != "" {
			return workflow.StepStream(p.Handler)
		}
	}
	return workflow.EventsStream
}

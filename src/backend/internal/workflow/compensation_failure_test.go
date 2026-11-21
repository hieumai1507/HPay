package workflow

// Wave 2 / ADR-0009: a compensation that fails must end in a state operators can find
// (compensation_failed), and a compensation that never reported back must be recovered.

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// undoHandler succeeds going forward; its compensation fails while fail is set.
type undoHandler struct {
	fail        atomic.Bool
	compensated atomic.Int64
}

func (h *undoHandler) Name() string { return "undo" }
func (h *undoHandler) Execute(_ context.Context, in StepInput) (StepOutput, error) {
	if in.Compensating {
		h.compensated.Add(1)
		if h.fail.Load() {
			return StepOutput{}, fmt.Errorf("reverse posting refused")
		}
	}
	return StepOutput{}, nil
}

const compDef = `[
  {"name":"A","type":"auto","handler":"undo","retry":1,"timeout_ms":2000,"on_failure":"fail","config":{"compensatable":true}},
  {"name":"B","type":"auto","handler":"boom","retry":1,"timeout_ms":2000,"on_failure":"compensate"}]`

func TestCompensationFailureIsItsOwnState(t *testing.T) {
	undo := &undoHandler{}
	undo.fail.Store(true)
	h := newHarness(t, undo, boomHandler{})
	name := uniqueName("compfail")
	h.insertDefinition(t, name, compDef)

	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	h.drain(t, 30)

	if got := h.instanceStatus(t, instID); got != store.WfStatusCompensationFailed {
		t.Fatalf("a failed compensation must end as compensation_failed, got %s", got)
	}
	if undo.compensated.Load() == 0 {
		t.Fatal("the compensation never ran, the test proves nothing")
	}
}

func TestCompensationSuccessStillCompensated(t *testing.T) {
	undo := &undoHandler{}
	h := newHarness(t, undo, boomHandler{})
	name := uniqueName("compok")
	h.insertDefinition(t, name, compDef)
	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	h.drain(t, 30)
	if got := h.instanceStatus(t, instID); got != store.WfStatusCompensated {
		t.Fatalf("want compensated, got %s", got)
	}
}

// A compensation step whose result event was lost is re-dispatched by the sweeper,
// exactly once even with two sweepers.
func TestStuckCompensationIsRecovered(t *testing.T) {
	undo := &undoHandler{}
	h := newHarness(t, undo, boomHandler{})
	name := uniqueName("compstuck")
	h.insertDefinition(t, name, compDef)
	instID, err := h.engine.Start(context.Background(), nil, name, uuid.Nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// run forward until the instance is compensating, then lose every pending event
	for i := 0; i < 30 && h.instanceStatus(t, instID) != store.WfStatusCompensating; i++ {
		h.pumpOutbox(t)
	}
	if got := h.instanceStatus(t, instID); got != store.WfStatusCompensating {
		t.Fatalf("setup: want compensating, got %s", got)
	}
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `UPDATE outbox SET published_at = now() WHERE payload->>'instance_id' = $1 AND published_at IS NULL`, instID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `UPDATE workflow_instances SET updated_at = now() - interval '10 minutes' WHERE id = $1`, instID); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rec := NewRecovery(h.pool, nil, h.reg, h.engine, h.worker, log)
	_ = rec.SweepStuck(ctx)
	_ = rec.SweepStuck(ctx) // the second sweep must find nothing: the first refreshed updated_at

	var pending int
	if err := h.pool.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE event_type=$1 AND payload->>'instance_id'=$2 AND published_at IS NULL`, EventStepStart, instID.String()).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("want exactly one re-dispatched compensation, got %d", pending)
	}
	h.drain(t, 30)
	if got := h.instanceStatus(t, instID); got != store.WfStatusCompensated {
		t.Fatalf("after recovery want compensated, got %s", got)
	}
}

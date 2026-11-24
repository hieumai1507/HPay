package admin

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hieumai1507/hpay-backend/internal/recon"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/test/testkit"
)

// uniqueReconDay returns a distinct historical day per test run so runs never
// collide on the run_date UNIQUE constraint (the DB is shared + persistent).
func uniqueReconDay() time.Time {
	off := int(time.Now().UnixNano() / 1000 % 2_000_000) // ~5479 years of spread
	return time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, off)
}

// mkTopup inserts a completed topup for a wallet at the given completion time
// and returns its ref_code (so a matching bank statement can be crafted).
func mkTopup(t *testing.T, e *testkit.Env, wallet uuid.UUID, amount int64, at time.Time) string {
	t.Helper()
	ref := "MPT" + uuid.NewString()[:10]
	cashIn := seedSystemWalletID(t, e, "SYSTEM_CASH_IN")
	if _, err := e.Pool.Exec(context.Background(), `
		INSERT INTO transactions (ref_code, idempotency_key, type, status,
		  sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by, completed_at)
		VALUES ($1, $2, 'topup', 'completed', $3, $4, $5, 0,
		  (SELECT id FROM users LIMIT 1), $6)`,
		ref, uuid.NewString(), cashIn, wallet, amount, at); err != nil {
		t.Fatal(err)
	}
	return ref
}

func mkStatement(t *testing.T, e *testkit.Env, ref string, amount int64, stmtDate time.Time) {
	t.Helper()
	if _, err := e.Pool.Exec(context.Background(),
		`INSERT INTO bank_statements (ref_code, amount, stmt_date) VALUES ($1,$2,$3)`,
		ref, amount, stmtDate); err != nil {
		t.Fatal(err)
	}
}

func seedSystemWalletID(t *testing.T, e *testkit.Env, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.Pool.QueryRow(context.Background(),
		`SELECT id FROM wallets WHERE system_code=$1`, code).Scan(&id); err != nil {
		t.Fatalf("system wallet %s: %v", code, err)
	}
	return id
}

// TestEdge_E16_ReconCutoffNoFalseMismatch: a topup completed at 23:59 on day D
// with a bank statement dated D+1 (same ref) must reconcile as MATCHED with a
// cutoff_shift marker — never a false mismatch — on day D's run.
func TestEdge_E16_ReconCutoffNoFalseMismatch(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	_, w, _ := e.UserWithMoney(t, 0)

	day := uniqueReconDay()
	nextDay := day.AddDate(0, 0, 1)

	ref := mkTopup(t, e, w, 500_000, day.Add(23*time.Hour+59*time.Minute+30*time.Second))
	mkStatement(t, e, ref, 500_000, nextDay) // statement slipped to the next day

	if _, err := e.Rsvc.RunForDate(ctx, day); err != nil {
		t.Fatalf("run day D: %v", err)
	}
	_, items, err := e.Rsvc.GetRun(ctx, day)
	if err != nil {
		t.Fatal(err)
	}

	var found bool
	for _, it := range items {
		if it.Result == store.ReconResultMatched && string(it.Detail) != "" &&
			testkit.ContainsAll(string(it.Detail), ref, "cutoff_shift") {
			found = true
		}
		if it.Result == store.ReconResultMissingExternal && testkit.Contains(string(it.Detail), ref) {
			t.Fatalf("E16 violated: ref %s reported missing_external despite a next-day statement", ref)
		}
	}
	if !found {
		t.Fatalf("expected a matched item with cutoff_shift for ref %s; items=%d", ref, len(items))
	}
}

// TestReconFourCategoriesAndIdempotent covers matched / amount_mismatch /
// missing_internal / missing_external in one run, produces an xlsx, and refuses
// a second run for the same date.
func TestReconFourCategoriesAndIdempotent(t *testing.T) {
	e := testkit.NewEnv(t, "")
	ctx := context.Background()
	_, w, _ := e.UserWithMoney(t, 0)
	day := uniqueReconDay().Add(12 * time.Hour)

	refMatch := mkTopup(t, e, w, 100_000, day.Add(9*time.Hour))
	mkStatement(t, e, refMatch, 100_000, day)

	refMismatch := mkTopup(t, e, w, 200_000, day.Add(10*time.Hour))
	mkStatement(t, e, refMismatch, 199_000, day) // amount differs

	mkTopup(t, e, w, 300_000, day.Add(11*time.Hour)) // no statement -> missing_external

	mkStatement(t, e, "MPX-orphan-"+uuid.NewString()[:6], 400_000, day) // no txn -> missing_internal

	runID, err := e.Rsvc.RunForDate(ctx, day)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if runID == uuid.Nil {
		t.Fatal("nil run id")
	}

	run, items, err := e.Rsvc.GetRun(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[store.ReconResult]int{}
	for _, it := range items {
		counts[it.Result]++
	}
	if counts[store.ReconResultMatched] < 1 || counts[store.ReconResultAmountMismatch] < 1 ||
		counts[store.ReconResultMissingExternal] < 1 || counts[store.ReconResultMissingInternal] < 1 {
		t.Fatalf("missing a category: %+v", counts)
	}
	if run.AiSummary == "" {
		t.Fatal("expected a mock ai_summary when there are mismatches")
	}
	if run.ReportUrl == "" {
		t.Fatal("expected an xlsx report key")
	}
	if data, err := e.Files.Get(ctx, run.ReportUrl); err != nil || len(data) == 0 {
		t.Fatalf("xlsx not in storage: %v (%d bytes)", err, len(data))
	}

	// Idempotent: a second run for the same day is refused.
	if _, err := e.Rsvc.RunForDate(ctx, day); err != recon.ErrAlreadyRan {
		t.Fatalf("second run: want ErrAlreadyRan, got %v", err)
	}
}

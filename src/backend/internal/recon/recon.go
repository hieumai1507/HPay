// Package recon reconciles internal topup transactions against the mock bank
// statements each day (docs 00 §8.2). Matching is by ref_code globally — a
// statement that slipped to a neighbouring day still matches its transaction
// (E16). It also verifies the internal invariant wallets.balance = SUM(ledger).
package recon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/ai"
	"github.com/hieumai1507/hpay-backend/internal/storage"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

var (
	ErrAlreadyRan   = errors.New("reconciliation already ran for this date")
	ErrRunNotFound  = errors.New("reconciliation run not found")
	ErrBalanceDrift = errors.New("internal ledger invariant violated")
)

type Service struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	files  storage.Store
	signer *storage.Signer
}

func NewService(pool *pgxpool.Pool, files storage.Store, signer *storage.Signer) *Service {
	return &Service{pool: pool, q: store.New(pool), files: files, signer: signer}
}

// classifiedItem is one reconciliation outcome before it's persisted.
type classifiedItem struct {
	txnID       uuid.UUID
	statementID pgtype.Int8
	result      store.ReconResult
	detail      map[string]any
}

// RunForDate reconciles a single day and persists the run + items in one tx.
// run_date UNIQUE makes it idempotent — a second run for the same date fails
// cleanly with ErrAlreadyRan.
func (s *Service) RunForDate(ctx context.Context, day time.Time) (uuid.UUID, error) {
	topups, err := s.q.ListCompletedTopupsByDate(ctx, pgtype.Date{Time: day, Valid: true})
	if err != nil {
		return uuid.Nil, err
	}
	refCodes := make([]string, 0, len(topups))
	for _, t := range topups {
		refCodes = append(refCodes, t.RefCode)
	}
	// Statements matching our internal refs (any date — E16), plus statements
	// booked on this day (to surface orphans with no internal counterpart).
	matchStmts, err := s.q.ListStatementsByRefCodes(ctx, refCodes)
	if err != nil {
		return uuid.Nil, err
	}
	dayStmts, err := s.q.ListStatementsByDate(ctx, pgtype.Date{Time: day, Valid: true})
	if err != nil {
		return uuid.Nil, err
	}

	items := classify(day, topups, matchStmts, dayStmts)

	// Internal invariant: any wallet whose balance != SUM(ledger) is a bug.
	drift, err := s.q.ListWalletLedgerMismatches(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	for _, d := range drift {
		items = append(items, classifiedItem{
			result: store.ReconResultAmountMismatch,
			detail: map[string]any{"internal_drift": true, "wallet_id": d.WalletID.String(),
				"balance": d.Balance, "ledger_balance": d.LedgerBalance},
		})
	}

	matched, mismatched := countResults(items)
	summary := s.summarize(ctx, day, items, matched, mismatched)
	reportURL := s.buildReport(ctx, day, items)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	run, err := qtx.InsertReconRun(ctx, store.InsertReconRunParams{
		RunDate:    pgtype.Date{Time: day, Valid: true},
		Total:      int32(len(items)),
		Matched:    int32(matched),
		Mismatched: int32(mismatched),
		AiSummary:  pgtype.Text{String: summary, Valid: summary != ""},
		ReportUrl:  pgtype.Text{String: reportURL, Valid: reportURL != ""},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, ErrAlreadyRan
		}
		return uuid.Nil, err
	}
	for _, it := range items {
		detail, _ := json.Marshal(it.detail)
		if err := qtx.InsertReconItem(ctx, store.InsertReconItemParams{
			RunID: run.ID, TransactionID: it.txnID, StatementID: it.statementID,
			Result: it.result, Detail: detail,
		}); err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return run.ID, nil
}

// classify implements the E16 matching: pair topups and statements by ref_code
// across all dates, marking a date mismatch as cutoff_shift (not a real
// discrepancy) and a repeated statement ref as a duplicate. matchStmts are
// statements found by internal ref (any date); dayStmts are statements booked
// on the run day, used to surface orphans (missing_internal).
func classify(day time.Time,
	topups []store.ListCompletedTopupsByDateRow,
	matchStmts []store.ListStatementsByRefCodesRow,
	dayStmts []store.ListStatementsByDateRow) []classifiedItem {

	byRef := map[string][]store.ListStatementsByRefCodesRow{}
	for _, st := range matchStmts {
		byRef[st.RefCode] = append(byRef[st.RefCode], st)
	}
	internalRefs := map[string]bool{}

	var items []classifiedItem
	for _, t := range topups {
		internalRefs[t.RefCode] = true
		matches := byRef[t.RefCode]
		switch {
		case len(matches) == 0:
			items = append(items, classifiedItem{
				txnID: t.ID, result: store.ReconResultMissingExternal,
				detail: map[string]any{"ref_code": t.RefCode, "amount": t.Amount,
					"note": "không tìm thấy sao kê ngân hàng cho giao dịch này"},
			})
		default:
			st := matches[0]
			detail := map[string]any{"ref_code": t.RefCode}
			// E16: a statement dated on a different day is a cutoff shift, not a
			// mismatch — the ref_code is the source of truth.
			if !sameDate(st.StmtDate.Time, day) {
				detail["cutoff_shift"] = true
				detail["stmt_date"] = st.StmtDate.Time.Format("2006-01-02")
			}
			if len(matches) > 1 {
				detail["duplicate_statements"] = len(matches)
			}
			result := store.ReconResultMatched
			if st.Amount != t.Amount {
				result = store.ReconResultAmountMismatch
				detail["internal_amount"] = t.Amount
				detail["statement_amount"] = st.Amount
			}
			items = append(items, classifiedItem{
				txnID: t.ID, statementID: pgtype.Int8{Int64: st.ID, Valid: true},
				result: result, detail: detail,
			})
		}
	}

	// Statements booked today whose ref has no completed internal topup at all.
	for _, st := range dayStmts {
		if internalRefs[st.RefCode] {
			continue
		}
		items = append(items, classifiedItem{
			statementID: pgtype.Int8{Int64: st.ID, Valid: true},
			result:      store.ReconResultMissingInternal,
			detail:      map[string]any{"ref_code": st.RefCode, "statement_amount": st.Amount},
		})
	}
	return items
}

func countResults(items []classifiedItem) (matched, mismatched int) {
	for _, it := range items {
		if it.result == store.ReconResultMatched {
			matched++
		} else {
			mismatched++
		}
	}
	return
}

func (s *Service) summarize(ctx context.Context, day time.Time, items []classifiedItem, matched, mismatched int) string {
	if mismatched == 0 {
		return ""
	}
	var b []map[string]any
	for _, it := range items {
		if it.result != store.ReconResultMatched {
			b = append(b, map[string]any{"result": string(it.result), "detail": it.detail})
		}
	}
	evidence, _ := json.Marshal(b)
	user := fmt.Sprintf("Đối soát ngày %s: %d khớp, %d lệch. Chi tiết các mục lệch: %s. "+
		"Tóm tắt ngắn gọn nguyên nhân lệch bằng tiếng Việt cho nhân viên vận hành.",
		day.Format("2006-01-02"), matched, mismatched, string(evidence))

	// Fetch dynamic active setting for fraud_advising
	setting, err := s.q.GetActiveAISetting(ctx, "fraud_advising")
	if err != nil {
		return ""
	}

	var activeAdvisor ai.Advisor
	switch setting.Provider {
	case "mock":
		activeAdvisor = ai.MockAdvisor{}
	case "anthropic":
		if !setting.KeyActive.Bool || !setting.KeyActive.Valid {
			return ""
		}
		activeAdvisor = ai.NewClaudeAdvisor(setting.ApiKey.String)
	case "openai":
		if !setting.KeyActive.Bool || !setting.KeyActive.Valid {
			return ""
		}
		activeAdvisor = ai.NewOpenAIAdvisor(setting.ApiKey.String, setting.ModelName)
	default:
		activeAdvisor = ai.MockAdvisor{}
	}

	text, err := activeAdvisor.Complete(ctx,
		"Bạn là trợ lý đối soát tài chính. Chỉ tóm tắt, không đề xuất hành động thay đổi số dư.", user)
	if err != nil {
		return ""
	}
	return text
}

func (s *Service) GetRun(ctx context.Context, day time.Time) (store.GetReconRunByDateRow, []store.ListReconItemsRow, error) {
	d := pgtype.Date{Time: day, Valid: true}
	run, err := s.q.GetReconRunByDate(ctx, d)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.GetReconRunByDateRow{}, nil, ErrRunNotFound
		}
		return store.GetReconRunByDateRow{}, nil, err
	}
	items, err := s.q.ListReconItems(ctx, d)
	if err != nil {
		return store.GetReconRunByDateRow{}, nil, err
	}
	return run, items, nil
}

// SignReportURL turns the stored xlsx key into a short-lived signed link.
func (s *Service) SignReportURL(key string) string {
	if key == "" {
		return ""
	}
	return s.signer.SignURL(key, 5*time.Minute)
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

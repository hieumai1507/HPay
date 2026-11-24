package fraud

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/listing"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

var (
	ErrFlagNotFound    = errors.New("fraud flag not found")
	ErrAlreadyResolved = errors.New("fraud flag already resolved")
)

// AdminService is the review side of fraud flags (docs 00 §9: a human clears
// or confirms; the rule engine only surfaces).
type AdminService struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewAdminService(pool *pgxpool.Pool) *AdminService {
	return &AdminService{pool: pool, q: store.New(pool)}
}

type FlagItem = store.ListFraudFlagsRow

// List returns one page of flags plus the total matching the same filter.
// status="all" spans every status; "" keeps the historical default of open.
func (s *AdminService) List(ctx context.Context, status string, f listing.Filter) ([]FlagItem, int64, error) {
	f = f.Normalize()
	if status == "all" {
		rows, err := s.q.ListAllFraudFlags(ctx, store.ListAllFraudFlagsParams{
			RefCode:   f.Text(),
			FromAt:    f.FromTS(),
			ToAt:      f.ToTS(),
			SortOrder: f.SortOrder(),
			Lim:       f.Limit,
			Off:       f.Offset,
		})
		if err != nil {
			return nil, 0, err
		}
		items := make([]FlagItem, 0, len(rows))
		for _, r := range rows {
			items = append(items, FlagItem{
				ID: r.ID, TransactionID: r.TransactionID, RefCode: r.RefCode,
				RuleCode: r.RuleCode, Score: r.Score, Reasons: r.Reasons,
				Status: r.Status, CreatedAt: r.CreatedAt,
			})
		}
		total, err := s.q.CountAllFraudFlags(ctx, store.CountAllFraudFlagsParams{
			RefCode: f.Text(), FromAt: f.FromTS(), ToAt: f.ToTS(),
		})
		if err != nil {
			return nil, 0, err
		}
		return items, total, nil
	}
	st, err := parseFlagStatus(status)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListFraudFlags(ctx, store.ListFraudFlagsParams{
		Status:    st,
		RefCode:   f.Text(),
		FromAt:    f.FromTS(),
		ToAt:      f.ToTS(),
		SortOrder: f.SortOrder(),
		Lim:       f.Limit,
		Off:       f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountFraudFlags(ctx, store.CountFraudFlagsParams{
		Status: st, RefCode: f.Text(), FromAt: f.FromTS(), ToAt: f.ToTS(),
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Resolve moves an open flag to cleared|confirmed_fraud. The guarded UPDATE
// arbitrates two reviewers: the loser gets ErrAlreadyResolved. Confirming does
// NOT auto-lock the account — that is a separate admin approval (F10/A-phases).
func (s *AdminService) Resolve(ctx context.Context, adminID, flagID uuid.UUID, confirm bool, reason string) error {
	newStatus := store.FlagStatusCleared
	action := "fraud.clear"
	if confirm {
		newStatus = store.FlagStatusConfirmedFraud
		action = "fraud.confirm"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	n, err := qtx.ResolveFraudFlagGuarded(ctx, store.ResolveFraudFlagGuardedParams{
		ID: flagID, Status: newStatus, ReviewerID: adminID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		if _, gerr := s.q.GetFraudFlag(ctx, flagID); gerr != nil {
			if errors.Is(gerr, pgx.ErrNoRows) {
				return ErrFlagNotFound
			}
			return gerr
		}
		return ErrAlreadyResolved
	}
	if err := qtx.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: adminID, ActorType: "admin", Action: action,
		Resource: "fraud_flag " + flagID.String(),
		Reason:   pgtype.Text{String: reason, Valid: reason != ""},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func parseFlagStatus(s string) (store.FlagStatus, error) {
	switch s {
	case "", "open":
		return store.FlagStatusOpen, nil
	case "cleared":
		return store.FlagStatusCleared, nil
	case "confirmed_fraud":
		return store.FlagStatusConfirmedFraud, nil
	default:
		return "", errors.New("status must be open|cleared|confirmed_fraud")
	}
}

func pgts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func itoa(n int) string { return strconv.Itoa(n) }

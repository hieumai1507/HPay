// Package offerguard enforces Offer Factory rules before a transaction enters
// the workflow engine: service availability inside the caller's offer, amount
// bounds, rolling daily/monthly volume and velocity — then resolves which auth
// method the confirm step must demand (docs 05 §4; audit gaps G2/G3).
//
// It fails closed on purpose: if we cannot establish the caller's limits, the
// transaction is refused rather than allowed through unmetered.
package offerguard

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// Sentinel violations — the API layer maps each to a stable ERR_* code.
var (
	ErrServiceNotInOffer = errors.New("dịch vụ không có trong gói của bạn")
	ErrServiceDisabled   = errors.New("dịch vụ đang tắt trong gói của bạn")
	ErrBelowMin          = errors.New("số tiền dưới mức tối thiểu của dịch vụ")
	ErrLimitPerTxn       = errors.New("vượt hạn mức mỗi giao dịch")
	ErrLimitDaily        = errors.New("vượt hạn mức trong ngày")
	ErrLimitMonthly      = errors.New("vượt hạn mức trong tháng")
	ErrTooFrequent       = errors.New("giao dịch quá nhanh, vui lòng thử lại sau")
)

// Decision is the guard's verdict for an allowed transaction.
type Decision struct {
	OfferServiceID uuid.UUID
	AuthMethod     string // none | pin | otp | 2fa
}

// DefaultOfferCode is used when a user has no explicit user_offers row, so the
// transaction is still metered by a real package instead of running unlimited.
// Override with DEFAULT_OFFER_CODE.
const DefaultOfferCode = "OFFER_CUST_L2"

type Guard struct {
	q            *store.Queries
	defaultOffer string
}

func New(pool *pgxpool.Pool) *Guard { return NewWithQueries(store.New(pool)) }

// NewWithQueries lets callers share an existing Queries handle (tests, tx reuse).
func NewWithQueries(q *store.Queries) *Guard {
	code := os.Getenv("DEFAULT_OFFER_CODE")
	if code == "" {
		code = DefaultOfferCode
	}
	return &Guard{q: q, defaultOffer: code}
}

// Check runs every offer rule for (user, service, amount). On success it returns
// the auth method required for this amount tier.
func (g *Guard) Check(ctx context.Context, userID uuid.UUID, serviceCode string, txnType store.TxnType, amount int64) (Decision, error) {
	cfg, err := g.q.GetOfferServiceForUser(ctx, store.GetOfferServiceForUserParams{
		UserID: userID,
		Code:   serviceCode,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// No explicit assignment — fall back to the platform default package.
		cfg, err = g.q.GetOfferServiceByOfferCode(ctx, store.GetOfferServiceByOfferCodeParams{
			Code:   g.defaultOffer,
			Code_2: serviceCode,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return Decision{}, ErrServiceNotInOffer
		}
	}
	if err != nil {
		return Decision{}, err
	}
	if !cfg.IsEnabled {
		return Decision{}, ErrServiceDisabled
	}
	if amount < cfg.MinAmount {
		return Decision{}, ErrBelowMin
	}
	if cfg.MaxAmountPerTxn > 0 && amount > cfg.MaxAmountPerTxn {
		return Decision{}, ErrLimitPerTxn
	}

	now := time.Now()
	if cfg.DailyLimit > 0 {
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		used, err := g.volumeSince(ctx, userID, txnType, startOfDay)
		if err != nil {
			return Decision{}, err
		}
		if used+amount > cfg.DailyLimit {
			return Decision{}, ErrLimitDaily
		}
	}
	if cfg.MonthlyLimit > 0 {
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		used, err := g.volumeSince(ctx, userID, txnType, startOfMonth)
		if err != nil {
			return Decision{}, err
		}
		if used+amount > cfg.MonthlyLimit {
			return Decision{}, ErrLimitMonthly
		}
	}
	if cfg.IntervalSeconds > 0 {
		last, err := g.q.LastUserTxnAt(ctx, store.LastUserTxnAtParams{InitiatedBy: userID, Type: txnType})
		if err != nil {
			return Decision{}, err
		}
		if last.Valid && now.Sub(last.Time) < time.Duration(cfg.IntervalSeconds)*time.Second {
			return Decision{}, ErrTooFrequent
		}
	}

	return Decision{OfferServiceID: cfg.ID, AuthMethod: g.authFor(ctx, cfg.ID, amount)}, nil
}

func (g *Guard) volumeSince(ctx context.Context, userID uuid.UUID, txnType store.TxnType, since time.Time) (int64, error) {
	return g.q.SumUserTxnAmountSince(ctx, store.SumUserTxnAmountSinceParams{
		InitiatedBy: userID,
		Type:        txnType,
		CreatedAt:   pgtype.Timestamptz{Time: since, Valid: true},
	})
}

// authFor picks the matrix tier covering amount (G3 — configured in DB, not
// hardcoded thresholds). Falls back to pin when no matrix row is configured.
func (g *Guard) authFor(ctx context.Context, offerServiceID uuid.UUID, amount int64) string {
	rows, err := g.q.ListAuthMatrix(ctx, offerServiceID)
	if err != nil || len(rows) == 0 {
		return "pin"
	}
	for _, r := range rows {
		if amount < r.MinAmount {
			continue
		}
		if r.MaxAmount.Valid && amount >= r.MaxAmount.Int64 {
			continue
		}
		return string(r.AuthMethod)
	}
	// Amount above every configured ceiling — use the strictest (highest) tier.
	return string(rows[len(rows)-1].AuthMethod)
}

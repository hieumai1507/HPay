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
	ErrOfferInactive     = errors.New("offer is inactive")
)

// Decision is the guard's verdict for an allowed transaction.
type Decision struct {
	OfferServiceID uuid.UUID
	AuthMethod     string // none | pin | otp | 2fa
}

// DefaultOfferCode is used when a user has no explicit user_offers row, so the
// transaction is still metered by a real package instead of running unlimited.
// It is the most restrictive customer offer (docs/specs/w3-offer-limits).
// Override with DEFAULT_OFFER_CODE.
const DefaultOfferCode = "OFFER_CUST_L0"

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
	lim, err := g.resolve(ctx, userID, serviceCode)
	if err != nil {
		return Decision{}, err
	}
	if !lim.offerActive {
		return Decision{}, ErrOfferInactive
	}
	cfg := lim
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

// limits is the resolved offer_services row plus whether its offer is active.
type limits struct {
	ID              uuid.UUID
	IsEnabled       bool
	MinAmount       int64
	MaxAmountPerTxn int64
	DailyLimit      int64
	MonthlyLimit    int64
	IntervalSeconds int32
	offerActive     bool
	OfferCode       string
}

// resolve finds the limits for (user, service). A user WITH an offer row is judged by
// that offer only: a missing service row is ErrServiceNotInOffer and an inactive offer
// blocks; neither falls back to the default offer. Only a user with NO offer row uses
// the default (most restrictive) offer.
func (g *Guard) resolve(ctx context.Context, userID uuid.UUID, serviceCode string) (limits, error) {
	assigned, err := g.q.HasUserOffer(ctx, userID)
	if err != nil {
		return limits{}, err
	}
	if assigned {
		row, err := g.q.GetOfferServiceForUser(ctx, store.GetOfferServiceForUserParams{UserID: userID, Code: serviceCode})
		if errors.Is(err, pgx.ErrNoRows) {
			return limits{}, ErrServiceNotInOffer
		}
		if err != nil {
			return limits{}, err
		}
		return limits{ID: row.ID, IsEnabled: row.IsEnabled, MinAmount: row.MinAmount, MaxAmountPerTxn: row.MaxAmountPerTxn,
			DailyLimit: row.DailyLimit, MonthlyLimit: row.MonthlyLimit, IntervalSeconds: row.IntervalSeconds,
			offerActive: row.OfferActive, OfferCode: row.OfferCode}, nil
	}
	row, err := g.q.GetOfferServiceByOfferCode(ctx, store.GetOfferServiceByOfferCodeParams{Code: g.defaultOffer, Code_2: serviceCode})
	if errors.Is(err, pgx.ErrNoRows) {
		return limits{}, ErrServiceNotInOffer
	}
	if err != nil {
		return limits{}, err
	}
	return limits{ID: row.ID, IsEnabled: row.IsEnabled, MinAmount: row.MinAmount, MaxAmountPerTxn: row.MaxAmountPerTxn,
		DailyLimit: row.DailyLimit, MonthlyLimit: row.MonthlyLimit, IntervalSeconds: row.IntervalSeconds,
		offerActive: row.OfferActive, OfferCode: row.OfferCode}, nil
}

// Limits returns the per-transaction, daily and monthly limits that apply to the user for a service.
func (g *Guard) Limits(ctx context.Context, userID uuid.UUID, serviceCode string) (OfferLimits, error) {
	lim, err := g.resolve(ctx, userID, serviceCode)
	if err != nil {
		return OfferLimits{}, err
	}
	if !lim.offerActive {
		return OfferLimits{}, ErrOfferInactive
	}
	return OfferLimits{OfferCode: lim.OfferCode, PerTxn: lim.MaxAmountPerTxn, Daily: lim.DailyLimit, Monthly: lim.MonthlyLimit}, nil
}

// OfferLimits is what the profile endpoint shows.
type OfferLimits struct {
	OfferCode string `json:"code"`
	PerTxn    int64  `json:"per_txn_limit"`
	Daily     int64  `json:"daily_limit"`
	Monthly   int64  `json:"monthly_limit"`
}

// CheckAmount is the per-transaction check only (no volume, no throttle), for paths that
// are not the engine: it still honors the offer, the enabled flag and the minimum.
func (g *Guard) CheckAmount(ctx context.Context, userID uuid.UUID, serviceCode string, amount int64) error {
	lim, err := g.resolve(ctx, userID, serviceCode)
	if err != nil {
		return err
	}
	if !lim.offerActive {
		return ErrOfferInactive
	}
	if !lim.IsEnabled {
		return ErrServiceDisabled
	}
	if amount < lim.MinAmount {
		return ErrBelowMin
	}
	if lim.MaxAmountPerTxn > 0 && amount > lim.MaxAmountPerTxn {
		return ErrLimitPerTxn
	}
	return nil
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

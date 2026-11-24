// Package fee computes transaction fees from fee_policies. Fees are frozen
// onto the transaction row at creation time (docs f5 §5).
package fee

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// Policy is the pure view used by Calc.
type Policy struct {
	FixedFee  int64
	PercentBP int64 // basis points: 10 = 0.10%
	MinFee    int64
	MaxFee    int64 // 0 with HasMax=false means no cap
	HasMax    bool
}

// Calc computes the fee for amount under p. Pure — unit-testable.
func Calc(p Policy, amount int64) int64 {
	f := p.FixedFee + amount*p.PercentBP/10_000
	if f < p.MinFee {
		f = p.MinFee
	}
	if p.HasMax && f > p.MaxFee {
		f = p.MaxFee
	}
	if f < 0 {
		f = 0
	}
	return f
}

type Service struct{ q *store.Queries }

func NewService(pool *pgxpool.Pool) *Service { return &Service{q: store.New(pool)} }

// NewServiceWithQueries lets a caller that already holds a transaction read fee policies on
// that same connection instead of taking a second one from the pool.
func NewServiceWithQueries(q *store.Queries) *Service { return &Service{q: q} }

// For returns the fee for a transaction type given the user's KYC tier.
// No policy row means the transfer is free (explicit product default).
func (s *Service) For(ctx context.Context, txnType store.TxnType, tier store.KycStatus, amount int64) (int64, error) {
	row, err := s.q.GetFeePolicy(ctx, store.GetFeePolicyParams{TxnType: txnType, KycTier: tier})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	p := Policy{FixedFee: row.FixedFee, PercentBP: int64(row.PercentBp), MinFee: row.MinFee}
	if row.MaxFee.Valid {
		p.HasMax, p.MaxFee = true, row.MaxFee.Int64
	}
	return Calc(p, amount), nil
}

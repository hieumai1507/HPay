// Package vault manages savings pockets (savings pots, docs 04 §2.2).
//
// A vault is not a separate balance store: it is a row in wallets with
// purpose='vault', so every ledger invariant applies to it unchanged and money
// only ever moves through the engine (service_code=VAULT_SAVINGS). This package
// owns the metadata side — listing and creating pockets — and nothing else.
package vault

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// Sentinel errors — the API layer maps each to a stable ERR_* code.
var (
	// ErrNotFound covers "no such vault" AND "belongs to another user": the
	// lookup is owner-scoped on purpose, so we never confirm that someone
	// else's vault id exists.
	ErrNotFound         = errors.New("vault not found")
	ErrLabelTaken       = errors.New("vault label already used")
	ErrLabelRequired    = errors.New("vault label is required")
	ErrDirectionInvalid = errors.New("direction must be deposit or withdraw")
)

const maxLabelLen = 60

type Service struct {
	q    *store.Queries
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{q: store.New(pool), pool: pool} }

type Vault struct {
	ID            uuid.UUID `json:"id"`
	Label         string    `json:"label"`
	Balance       int64     `json:"balance"`
	TargetAmount  int64     `json:"target_amount"`
	RoundUpTarget bool      `json:"round_up_target"`
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Vault, error) {
	rows, err := s.q.ListUserVaults(ctx, store.ListUserVaultsParams{
		OwnerID: userID, Currency: "VND",
	})
	if err != nil {
		return nil, err
	}
	out := make([]Vault, 0, len(rows))
	for _, r := range rows {
		out = append(out, Vault{
			ID: r.ID, Label: r.Label.String, Balance: r.Balance, TargetAmount: r.TargetAmount,
			RoundUpTarget: r.RoundUpTarget,
		})
	}
	return out, nil
}

// Create opens a new pocket. Target is optional and purely informational.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, label string, target int64) (Vault, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return Vault{}, ErrLabelRequired
	}
	if len(label) > maxLabelLen {
		label = label[:maxLabelLen]
	}
	row, err := s.q.CreateVaultWallet(ctx, store.CreateVaultWalletParams{
		OwnerID:      userID,
		Currency:     "VND",
		Label:        pgtype.Text{String: label, Valid: true},
		TargetAmount: pgtype.Int8{Int64: target, Valid: target > 0},
	})
	if err != nil {
		// wallet_owner_unique now includes label, so a repeat name for the same
		// owner is a duplicate rather than a second pocket.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Vault{}, ErrLabelTaken
		}
		return Vault{}, err
	}
	return Vault{
		ID: row.ID, Label: row.Label.String, Balance: row.Balance, TargetAmount: row.TargetAmount,
		RoundUpTarget: row.RoundUpTarget,
	}, nil
}

// Owned resolves a vault the caller actually owns, or ErrNotFound.
func (s *Service) Owned(ctx context.Context, userID, vaultID uuid.UUID) (Vault, error) {
	row, err := s.q.GetUserVault(ctx, store.GetUserVaultParams{ID: vaultID, OwnerID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Vault{}, ErrNotFound
		}
		return Vault{}, err
	}
	return Vault{
		ID: row.ID, Label: row.Label.String, Balance: row.Balance, TargetAmount: row.TargetAmount,
		RoundUpTarget: row.RoundUpTarget,
	}, nil
}

// SetRoundUpTarget designates (or un-designates) a vault as the owner's
// round-up destination. Setting a new target atomically unsets any previous
// one first, so the partial unique index (docs plan round-up-vault §4) never
// sees two true rows for the same owner mid-transaction.
func (s *Service) SetRoundUpTarget(ctx context.Context, userID, vaultID uuid.UUID, enabled bool) (Vault, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Vault{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	if enabled {
		if err := qtx.UnsetRoundUpTargets(ctx, userID); err != nil {
			return Vault{}, err
		}
	}
	row, err := qtx.SetRoundUpTarget(ctx, store.SetRoundUpTargetParams{
		ID: vaultID, OwnerID: userID, RoundUpTarget: enabled,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Vault{}, ErrNotFound
		}
		return Vault{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Vault{}, err
	}
	return Vault{
		ID: row.ID, Label: row.Label.String, Balance: row.Balance, TargetAmount: row.TargetAmount,
		RoundUpTarget: row.RoundUpTarget,
	}, nil
}

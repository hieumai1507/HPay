// Package ledger owns the double-entry posting engine. ledger.Post is the ONLY
// code path allowed to change wallet balances (docs plan f3 §1).
package ledger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

var (
	ErrUnbalanced          = errors.New("ledger legs do not balance (sum debit != sum credit)")
	ErrInvalidLeg          = errors.New("ledger leg invalid")
	ErrSelfTransfer        = errors.New("debit and credit on the same wallet only")
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

// Leg is one side of a posting. Amount is always positive; Direction decides sign.
type Leg struct {
	WalletID  uuid.UUID
	Direction store.EntryDirection
	Amount    int64
}

// ValidateLegs enforces the double-entry invariants without touching the DB —
// unit-testable (E14 + balance checks).
func ValidateLegs(legs []Leg) error {
	if len(legs) < 2 {
		return fmt.Errorf("%w: need at least 2 legs, got %d", ErrInvalidLeg, len(legs))
	}
	var debit, credit int64
	wallets := map[uuid.UUID]bool{}
	for _, l := range legs {
		if l.Amount <= 0 {
			return fmt.Errorf("%w: amount must be positive", ErrInvalidLeg)
		}
		if l.WalletID == uuid.Nil {
			return fmt.Errorf("%w: nil wallet id", ErrInvalidLeg)
		}
		switch l.Direction {
		case store.EntryDirectionDebit:
			debit += l.Amount
		case store.EntryDirectionCredit:
			credit += l.Amount
		default:
			return fmt.Errorf("%w: unknown direction %q", ErrInvalidLeg, l.Direction)
		}
		wallets[l.WalletID] = true
	}
	if debit != credit {
		return fmt.Errorf("%w: debit=%d credit=%d", ErrUnbalanced, debit, credit)
	}
	if len(wallets) < 2 {
		return ErrSelfTransfer // E14: money must move between distinct wallets
	}
	return nil
}

// Post applies legs atomically inside the caller's transaction:
// lock all wallets in id order (E03), reject overdrafts on non-system wallets
// (E02), then write balances + append-only entries with running balance_after.
func Post(ctx context.Context, tx pgx.Tx, txnID uuid.UUID, legs []Leg) error {
	return PostWithQueries(ctx, store.New(tx), txnID, legs)
}

// PostWithQueries is Post for callers that already hold a tx-scoped *Queries
// (e.g. an approval executor running inside the finalize transaction). Same
// invariants; the caller must pass queries bound to a transaction.
func PostWithQueries(ctx context.Context, q *store.Queries, txnID uuid.UUID, legs []Leg) error {
	if err := ValidateLegs(legs); err != nil {
		return err
	}

	ids := uniqueSortedIDs(legs)
	rows, err := q.GetWalletsForUpdate(ctx, ids)
	if err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return ErrWalletNotFound
	}

	type wstate struct {
		running  int64
		delta    int64
		isSystem bool
	}
	states := make(map[uuid.UUID]*wstate, len(rows))
	for _, w := range rows {
		states[w.ID] = &wstate{running: w.Balance, isSystem: w.OwnerType == store.OwnerTypeSystem}
	}

	// First pass: final balances must not go negative for non-system wallets.
	for _, l := range legs {
		st := states[l.WalletID]
		if l.Direction == store.EntryDirectionDebit {
			st.delta -= l.Amount
		} else {
			st.delta += l.Amount
		}
	}
	for _, st := range states {
		if !st.isSystem && st.running+st.delta < 0 {
			return ErrInsufficientBalance
		}
	}

	// Second pass: entries with running balance_after, in leg order.
	for _, l := range legs {
		st := states[l.WalletID]
		if l.Direction == store.EntryDirectionDebit {
			st.running -= l.Amount
		} else {
			st.running += l.Amount
		}
		if _, err := q.InsertLedgerEntry(ctx, store.InsertLedgerEntryParams{
			TransactionID: txnID, WalletID: l.WalletID,
			Direction: l.Direction, Amount: l.Amount, BalanceAfter: st.running,
		}); err != nil {
			return err
		}
	}

	// Final pass: one balance update per wallet.
	for id, st := range states {
		if st.delta == 0 {
			continue
		}
		if _, err := q.ApplyWalletDelta(ctx, store.ApplyWalletDeltaParams{ID: id, Balance: st.delta}); err != nil {
			return err
		}
	}
	return nil
}

func uniqueSortedIDs(legs []Leg) []uuid.UUID {
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(legs))
	for _, l := range legs {
		if !seen[l.WalletID] {
			seen[l.WalletID] = true
			ids = append(ids, l.WalletID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
	return ids
}

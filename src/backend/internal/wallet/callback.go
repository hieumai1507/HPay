package wallet

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// ErrCallbackAmountMismatch: the bank notified an amount that differs from the
// transaction. The statement row is still booked so recon sees the mismatch.
var ErrCallbackAmountMismatch = errors.New("callback amount does not match transaction")

// CreatePendingTopup parks a topup at status 'created' awaiting the bank
// callback — no ledger legs are posted here (F10 E15). Used by tests and the
// seeder; the interactive topup path stays synchronous.
func (s *Service) CreatePendingTopup(ctx context.Context, userID uuid.UUID, amount int64, idemKey string) (TopupResult, error) {
	if amount <= 0 {
		return TopupResult{}, ErrAmountInvalid
	}
	userWallet, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: userID, Currency: "VND"})
	if err != nil {
		return TopupResult{}, ErrWalletNotFound
	}
	cashIn, err := s.q.GetSystemWallet(ctx, pgtype.Text{String: "SYSTEM_CASH_IN", Valid: true})
	if err != nil {
		return TopupResult{}, err
	}
	txn, err := s.q.InsertPendingTopup(ctx, store.InsertPendingTopupParams{
		RefCode: NewRefCode(), IdempotencyKey: pgtype.Text{String: idemKey, Valid: idemKey != ""},
		SenderWalletID: cashIn.ID, ReceiverWalletID: userWallet.ID,
		Amount:      amount,
		Note:        pgtype.Text{String: "topup awaiting bank callback", Valid: true},
		InitiatedBy: userID,
	})
	if err != nil {
		return TopupResult{}, err
	}
	return TopupResult{RefCode: txn.RefCode, Status: string(txn.Status), Amount: amount,
		CreatedAt: txn.CreatedAt.Time}, nil
}

type CallbackResult struct {
	RefCode   string `json:"ref_code"`
	Status    string `json:"status"`
	Duplicate bool   `json:"duplicate"`
}

// BankCallback processes a (mock) bank notification for a topup. Idempotent by
// ref_code (E15): the first callback for a 'created' topup posts the ledger
// legs and completes it; any repeat changes no money. EVERY callback books a
// bank_statements row — duplicates surface in reconciliation, mismatched
// amounts surface as amount_mismatch.
func (s *Service) BankCallback(ctx context.Context, refCode string, amount int64, bankRef string) (CallbackResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CallbackResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	txn, err := qtx.GetTopupByRefCodeForUpdate(ctx, refCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CallbackResult{}, ErrTxnNotFound
		}
		return CallbackResult{}, err
	}

	res := CallbackResult{RefCode: refCode, Status: string(txn.Status)}
	credited := false
	if txn.Status == store.TxnStatusCreated && amount == txn.Amount {
		if err := ledger.Post(ctx, tx, txn.ID, ledger.KindForward, []ledger.Leg{
			{WalletID: txn.SenderWalletID, Direction: store.EntryDirectionDebit, Amount: amount},
			{WalletID: txn.ReceiverWalletID, Direction: store.EntryDirectionCredit, Amount: amount},
		}); err != nil {
			return CallbackResult{}, err
		}
		n, err := qtx.CompleteTransactionFromCreated(ctx, txn.ID)
		if err != nil {
			return CallbackResult{}, err
		}
		if n != 1 { // FOR UPDATE makes this unreachable; keep the guard honest
			return CallbackResult{}, errors.New("topup transition lost despite row lock")
		}
		res.Status = string(store.TxnStatusCompleted)
		credited = true
	}
	res.Duplicate = !credited && txn.Status == store.TxnStatusCompleted

	raw, _ := json.Marshal(map[string]any{"bank_ref": bankRef, "duplicate": res.Duplicate})
	if _, err := qtx.InsertBankStatement(ctx, store.InsertBankStatementParams{
		RefCode: refCode, Amount: amount,
		StmtDate: pgtype.Date{Time: time.Now(), Valid: true},
		Raw:      raw,
	}); err != nil {
		return CallbackResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CallbackResult{}, err
	}
	if !credited && txn.Status == store.TxnStatusCreated && amount != txn.Amount {
		return res, ErrCallbackAmountMismatch
	}
	return res, nil
}

// Package wallet provides user-facing wallet operations. Money movement always
// funnels through ledger.Post — this package never updates balances directly.
package wallet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/user"
)

var (
	ErrAmountInvalid  = errors.New("amount must be positive")
	ErrLimitExceeded  = errors.New("amount exceeds kyc tier limit")
	ErrWalletNotFound = errors.New("wallet not found")
	ErrTxnNotFound    = errors.New("transaction not found")
)

// Per-transaction limits are not defined here: they follow the user's offer
// (ADR-0007, docs/specs/w3-offer-limits) and are enforced by offerguard.

type Service struct {
	pool  *pgxpool.Pool
	q     *store.Queries
	guard *offerguard.Guard
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: store.New(pool), guard: offerguard.New(pool)}
}

// EnsureMainWallet is idempotent: safe to call on every register/login
// (self-heal + backfill, docs plan f3 §1).
func (s *Service) EnsureMainWallet(ctx context.Context, userID uuid.UUID) error {
	_, err := s.q.CreateUserWallet(ctx, store.CreateUserWalletParams{
		OwnerID: userID, Currency: "VND", Purpose: "main",
		Label: pgtype.Text{},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // ON CONFLICT DO NOTHING -> no row
			return nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil
		}
		return err
	}
	return nil
}

type TopupResult struct {
	RefCode    string    `json:"ref_code"`
	Status     string    `json:"status"`
	Amount     int64     `json:"amount"`
	NewBalance int64     `json:"new_balance,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	Duplicate  bool      `json:"duplicate,omitempty"`
}

// Topup credits the user's main wallet from SYSTEM_CASH_IN in one atomic tx.
// A repeated Idempotency-Key returns the original transaction (E01).
func (s *Service) Topup(ctx context.Context, userID uuid.UUID, amount int64, idemKey string) (TopupResult, error) {
	hash := RequestHash("topup", strconv.FormatInt(amount, 10))
	old, err := FindReplay(ctx, s.q, userID, idemKey, hash)
	if err != nil {
		return TopupResult{}, err
	}
	if old != nil {
		return topupReplay(old), nil
	}
	if amount <= 0 {
		return TopupResult{}, ErrAmountInvalid
	}
	if err := s.guard.CheckAmount(ctx, userID, "TOPUP_BANK", amount); err != nil {
		return TopupResult{}, err
	}

	userWallet, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: userID, Currency: "VND"})
	if err != nil {
		return TopupResult{}, ErrWalletNotFound
	}
	cashIn, err := s.q.GetSystemWallet(ctx, pgtype.Text{String: "SYSTEM_CASH_IN", Valid: true})
	if err != nil {
		return TopupResult{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TopupResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	txn, err := qtx.InsertTransaction(ctx, store.InsertTransactionParams{
		RefCode: NewRefCode(), IdempotencyKey: KeyText(idemKey),
		Type:           store.TxnTypeTopup,
		SenderWalletID: cashIn.ID, ReceiverWalletID: userWallet.ID,
		Amount: amount, Fee: 0,
		Note: pgtype.Text{String: "topup via mock bank", Valid: true}, InitiatedBy: userID,
		RequestHash: HashText(hash),
	})
	if err != nil {
		if IsIdempotencyConflict(err) {
			old, gerr := FindReplay(ctx, s.q, userID, idemKey, hash)
			if gerr != nil {
				return TopupResult{}, gerr
			}
			if old != nil {
				return topupReplay(old), nil
			}
		}
		return TopupResult{}, err
	}

	if err := ledger.Post(ctx, tx, txn.ID, ledger.KindForward, []ledger.Leg{
		{WalletID: cashIn.ID, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: userWallet.ID, Direction: store.EntryDirectionCredit, Amount: amount},
	}); err != nil {
		return TopupResult{}, err
	}
	if _, err := qtx.CompleteTransaction(ctx, txn.ID); err != nil {
		return TopupResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TopupResult{}, err
	}
	return TopupResult{RefCode: txn.RefCode, Status: "completed", Amount: amount,
		NewBalance: userWallet.Balance + amount, CreatedAt: txn.CreatedAt.Time}, nil
}

func topupReplay(old *store.GetUserTransactionByIdempotencyKeyRow) TopupResult {
	return TopupResult{RefCode: old.RefCode, Status: string(old.Status), Amount: old.Amount,
		CreatedAt: old.CreatedAt.Time, Duplicate: true}
}

func (s *Service) ListWallets(ctx context.Context, userID uuid.UUID) ([]store.ListUserWalletsRow, error) {
	return s.q.ListUserWallets(ctx, userID)
}

// TxnView is the owner-facing transaction DTO (m3-api-support). Counterparty
// identity is ALWAYS masked here — sqlc rows carrying raw name/phone never
// leave this package (fintech-review: no raw PII of other users on the wire).
type TxnView struct {
	ID                uuid.UUID  `json:"id"`
	RefCode           string     `json:"ref_code"`
	Type              string     `json:"type"`
	Status            string     `json:"status"`
	SenderWalletID    uuid.UUID  `json:"sender_wallet_id"`
	ReceiverWalletID  uuid.UUID  `json:"receiver_wallet_id"`
	Amount            int64      `json:"amount"`
	Fee               int64      `json:"fee"`
	Note              string     `json:"note,omitempty"`
	Direction         string     `json:"direction"`          // "in" | "out" relative to the caller's wallet
	CounterpartyName  *string    `json:"counterparty_name"`  // masked; null for system/org side
	CounterpartyPhone *string    `json:"counterparty_phone"` // masked; null for system/org side
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

// counterparty picks the OTHER side's identity relative to myWallet and masks it.
func counterparty(myWallet, senderWallet uuid.UUID, senderName, senderPhone, receiverName, receiverPhone pgtype.Text) (direction string, name, phone *string) {
	otherName, otherPhone := senderName, senderPhone
	direction = "in"
	if senderWallet == myWallet {
		direction = "out"
		otherName, otherPhone = receiverName, receiverPhone
	}
	if otherName.Valid {
		m := user.MaskName(otherName.String)
		name = &m
	}
	if otherPhone.Valid {
		m := user.MaskPhone(otherPhone.String)
		phone = &m
	}
	return direction, name, phone
}

// History returns transactions of the user's main wallet, keyset-paginated.
func (s *Service) History(ctx context.Context, userID uuid.UUID, before time.Time, beforeID uuid.UUID, limit int32) ([]TxnView, error) {
	w, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: userID, Currency: "VND"})
	if err != nil {
		return nil, ErrWalletNotFound
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.q.ListTransactionsByWallet(ctx, store.ListTransactionsByWalletParams{
		SenderWalletID: w.ID, CreatedAt: pgts(before), Column3: beforeID, Limit: limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]TxnView, 0, len(rows))
	for _, r := range rows {
		v := TxnView{
			ID: r.ID, RefCode: r.RefCode, Type: string(r.Type), Status: string(r.Status),
			SenderWalletID: r.SenderWalletID, ReceiverWalletID: r.ReceiverWalletID,
			Amount: r.Amount, Fee: r.Fee, Note: r.Note.String, CreatedAt: r.CreatedAt.Time,
		}
		if r.CompletedAt.Valid {
			t := r.CompletedAt.Time
			v.CompletedAt = &t
		}
		v.Direction, v.CounterpartyName, v.CounterpartyPhone =
			counterparty(w.ID, r.SenderWalletID, r.SenderName, r.SenderPhone, r.ReceiverName, r.ReceiverPhone)
		out = append(out, v)
	}
	return out, nil
}

// Detail enforces ownership: only sender/receiver wallet owner sees the txn.
func (s *Service) Detail(ctx context.Context, userID uuid.UUID, refCode string) (TxnView, error) {
	txn, err := s.q.GetTransactionByRefCode(ctx, refCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TxnView{}, ErrTxnNotFound
		}
		return TxnView{}, err
	}
	w, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: userID, Currency: "VND"})
	if err != nil {
		return TxnView{}, ErrTxnNotFound
	}
	if txn.SenderWalletID != w.ID && txn.ReceiverWalletID != w.ID {
		return TxnView{}, ErrTxnNotFound // 404, does not leak existence
	}
	v := TxnView{
		ID: txn.ID, RefCode: txn.RefCode, Type: string(txn.Type), Status: string(txn.Status),
		SenderWalletID: txn.SenderWalletID, ReceiverWalletID: txn.ReceiverWalletID,
		Amount: txn.Amount, Fee: txn.Fee, Note: txn.Note.String, CreatedAt: txn.CreatedAt.Time,
	}
	if txn.CompletedAt.Valid {
		t := txn.CompletedAt.Time
		v.CompletedAt = &t
	}
	v.Direction, v.CounterpartyName, v.CounterpartyPhone =
		counterparty(w.ID, txn.SenderWalletID, txn.SenderName, txn.SenderPhone, txn.ReceiverName, txn.ReceiverPhone)
	return v, nil
}

// DetailRaw returns the underlying transaction row for internal callers
// (timeline handler needs the txn ID after the ownership check).
func (s *Service) DetailRaw(ctx context.Context, userID uuid.UUID, refCode string) (store.GetTransactionByRefCodeRow, error) {
	txn, err := s.q.GetTransactionByRefCode(ctx, refCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.GetTransactionByRefCodeRow{}, ErrTxnNotFound
		}
		return store.GetTransactionByRefCodeRow{}, err
	}
	w, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: userID, Currency: "VND"})
	if err != nil {
		return store.GetTransactionByRefCodeRow{}, ErrTxnNotFound
	}
	if txn.SenderWalletID != w.ID && txn.ReceiverWalletID != w.ID {
		return store.GetTransactionByRefCodeRow{}, ErrTxnNotFound
	}
	return txn, nil
}

// NewRefCode returns a display code like MP7F3A9C21D4E5 (<= 24 chars).
func NewRefCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "MP" + strings.ToUpper(hex.EncodeToString(b))
}

func pgts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

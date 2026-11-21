// Package wallet provides user-facing wallet operations. Money movement always
// funnels through ledger.Post — this package never updates balances directly.
package wallet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/user"
)

var (
	ErrAmountInvalid  = errors.New("amount must be positive")
	ErrLimitExceeded  = errors.New("amount exceeds kyc tier limit")
	ErrWalletNotFound = errors.New("wallet not found")
	ErrTxnNotFound    = errors.New("transaction not found")
)

// Per-transaction limits by KYC tier (minor units, VND). Moves into a DB
// policy table together with the fee engine in F5 (docs plan f3 §6).
func TxnLimit(kyc store.KycStatus) int64 {
	switch kyc {
	case store.KycStatusVerified, store.KycStatusAutoApproved:
		return 100_000_000
	case store.KycStatusManualReview, store.KycStatusPending:
		return 5_000_000
	default: // none, rejected
		return 2_000_000
	}
}

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: store.New(pool)}
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
	if amount <= 0 {
		return TopupResult{}, ErrAmountInvalid
	}
	ustate, err := s.q.GetUserStatusByID(ctx, userID)
	if err != nil {
		return TopupResult{}, err
	}
	if amount > TxnLimit(ustate.KycStatus) {
		return TopupResult{}, ErrLimitExceeded
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
		RefCode: NewRefCode(), IdempotencyKey: pgtype.Text{String: idemKey, Valid: idemKey != ""},
		Type:           store.TxnTypeTopup,
		SenderWalletID: cashIn.ID, ReceiverWalletID: userWallet.ID,
		Amount: amount, Fee: 0,
		Note: pgtype.Text{String: "topup via mock bank", Valid: true}, InitiatedBy: userID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "idempotency") {
			old, gerr := s.q.GetTransactionByIdempotencyKey(ctx, pgtype.Text{String: idemKey, Valid: true})
			if gerr != nil {
				return TopupResult{}, gerr
			}
			return TopupResult{RefCode: old.RefCode, Status: string(old.Status), Amount: old.Amount,
				CreatedAt: old.CreatedAt.Time, Duplicate: true}, nil
		}
		return TopupResult{}, err
	}

	if err := ledger.Post(ctx, tx, txn.ID, []ledger.Leg{
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
		return TxnView{}, ErrTxnNotFound // 404, không lộ tồn tại
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

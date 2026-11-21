// Package transfer creates P2P transactions and hands them to the workflow
// engine — transaction row + first step event commit atomically (docs f5).
package transfer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/fee"
	"github.com/hieumai1507/hpay-backend/internal/ledger"
	"github.com/hieumai1507/hpay-backend/internal/offerguard"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/vault"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

var ErrReceiverNotFound = errors.New("receiver not found")

type Service struct {
	pool    *pgxpool.Pool
	q       *store.Queries
	engine  *workflow.Engine
	fees    *fee.Service
	defName string
	// dynamicDispatch resolves txn type / workflow def / wallet strategy from the
	// services row per service_code (G1). Off = legacy hardcoded P2P path (rollback).
	dynamicDispatch bool
	// guard enforces Offer Factory limits and resolves the auth method (G2/G3).
	guard   *offerguard.Guard
	guardOn bool
	// vaults resolves savings pockets owner-scoped for the user_to_vault strategy.
	vaults *vault.Service
}

func NewService(pool *pgxpool.Pool, engine *workflow.Engine, fees *fee.Service) *Service {
	return &Service{
		pool: pool, q: store.New(pool), engine: engine, fees: fees, defName: "p2p_transfer",
		dynamicDispatch: os.Getenv("ENGINE_DYNAMIC_DISPATCH") != "false",
		guard:           offerguard.New(pool),
		guardOn:         os.Getenv("OFFERGUARD_ENABLED") != "false",
		vaults:          vault.New(pool),
	}
}

// NewServiceWithDefinition lets tests run against a test definition.
func NewServiceWithDefinition(pool *pgxpool.Pool, engine *workflow.Engine, fees *fee.Service, defName string) *Service {
	s := NewService(pool, engine, fees)
	s.defName = defName
	return s
}

type Result struct {
	RefCode   string    `json:"ref_code"`
	Status    string    `json:"status"`
	Amount    int64     `json:"amount"`
	Fee       int64     `json:"fee"`
	Total     int64     `json:"total"`
	CreatedAt time.Time `json:"created_at"`
	Duplicate bool      `json:"duplicate,omitempty"`
}

// Transfer lookup user by phone first, then calls TransferToUser.
func (s *Service) Transfer(ctx context.Context, senderID uuid.UUID, receiverPhone string, amount int64, note, idemKey string) (Result, error) {
	receiver, err := s.q.GetUserByPhone(ctx, receiverPhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrReceiverNotFound
		}
		return Result{}, err
	}
	return s.TransferToUser(ctx, senderID, receiver.ID, amount, note, idemKey)
}

// TransferToUser validates fast (fail-fast UX), freezes the fee, then creates the
// transaction + workflow start in ONE DB transaction. Deep checks re-run in
// workflow steps (E12) — these soft checks are for immediate user feedback.
func (s *Service) TransferToUser(ctx context.Context, senderID, receiverID uuid.UUID, amount int64, note, idemKey string) (Result, error) {
	if amount <= 0 {
		return Result{}, wallet.ErrAmountInvalid
	}
	sender, err := s.q.GetUserStatusByID(ctx, senderID)
	if err != nil {
		return Result{}, err
	}
	if amount > wallet.TxnLimit(sender.KycStatus) {
		return Result{}, wallet.ErrLimitExceeded
	}
	_, err = s.q.GetUserStatusByID(ctx, receiverID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, ErrReceiverNotFound
		}
		return Result{}, err
	}
	if receiverID == senderID {
		return Result{}, ledger.ErrSelfTransfer
	}
	senderWallet, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: senderID, Currency: "VND"})
	if err != nil {
		return Result{}, wallet.ErrWalletNotFound
	}
	receiverWallet, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: receiverID, Currency: "VND"})
	if err != nil {
		return Result{}, ErrReceiverNotFound
	}

	feeAmt, err := s.fees.For(ctx, store.TxnTypeP2p, sender.KycStatus, amount)
	if err != nil {
		return Result{}, err
	}
	if senderWallet.Balance < amount+feeAmt {
		return Result{}, ledger.ErrInsufficientBalance // fail fast; hard check in MOVE_FUNDS
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	txn, err := qtx.InsertTransaction(ctx, store.InsertTransactionParams{
		RefCode: wallet.NewRefCode(), IdempotencyKey: pgtype.Text{String: idemKey, Valid: idemKey != ""},
		Type:           store.TxnTypeP2p,
		SenderWalletID: senderWallet.ID, ReceiverWalletID: receiverWallet.ID,
		Amount: amount, Fee: feeAmt,
		Note: noteText(note), InitiatedBy: senderID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "idempotency") {
			old, gerr := s.q.GetTransactionByIdempotencyKey(ctx, pgtype.Text{String: idemKey, Valid: true})
			if gerr != nil {
				return Result{}, gerr
			}
			return Result{RefCode: old.RefCode, Status: string(old.Status), Amount: old.Amount,
				Fee: old.Fee, Total: old.Amount + old.Fee, CreatedAt: old.CreatedAt.Time, Duplicate: true}, nil
		}
		return Result{}, err
	}
	if _, err := s.engine.Start(ctx, tx, s.defName, txn.ID, map[string]any{"ref_code": txn.RefCode}); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{RefCode: txn.RefCode, Status: string(txn.Status), Amount: amount,
		Fee: feeAmt, Total: amount + feeAmt, CreatedAt: txn.CreatedAt.Time}, nil
}

// FeePreview returns the fee the caller would pay right now (mobile confirm screen).
func (s *Service) FeePreview(ctx context.Context, userID uuid.UUID, amount int64) (feeAmt, total int64, err error) {
	if amount <= 0 {
		return 0, 0, wallet.ErrAmountInvalid
	}
	u, err := s.q.GetUserStatusByID(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	feeAmt, err = s.fees.For(ctx, store.TxnTypeP2p, u.KycStatus, amount)
	if err != nil {
		return 0, 0, err
	}
	return feeAmt, amount + feeAmt, nil
}

type TimelineStep struct {
	Step       string     `json:"step"`
	Status     string     `json:"status"`
	Attempt    int16      `json:"attempt"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type Timeline struct {
	InstanceStatus string         `json:"instance_status"`
	CurrentStep    string         `json:"current_step,omitempty"`
	FailureCode    string         `json:"failure_code,omitempty"` // stable ERR_* when failed/compensated (m3-api-support)
	Steps          []TimelineStep `json:"steps"`
}

// failureCode maps a stored step error to a stable public ERR_* code via an
// allowlist. Anything unrecognized collapses to ERR_TRANSFER_FAILED — internal
// error text (SQL, saga detail) must never reach the client verbatim.
func failureCode(errText string) string {
	switch {
	case strings.Contains(errText, ledger.ErrInsufficientBalance.Error()):
		return "ERR_INSUFFICIENT_BALANCE"
	case strings.Contains(errText, "blocked by fraud"):
		return "ERR_FRAUD_BLOCKED"
	case strings.Contains(errText, "not active"):
		return "ERR_ACCOUNT_INACTIVE"
	default:
		return "ERR_TRANSFER_FAILED"
	}
}

// TimelineFor returns the public step trail for an owned transaction.
func (s *Service) TimelineFor(ctx context.Context, txnID uuid.UUID) (Timeline, error) {
	inst, err := s.q.GetInstanceByTransactionID(ctx, txnID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Timeline{InstanceStatus: "none"}, nil // e.g. sync topup
		}
		return Timeline{}, err
	}
	execs, err := s.q.ListStepExecutions(ctx, inst.ID)
	if err != nil {
		return Timeline{}, err
	}
	tl := Timeline{InstanceStatus: string(inst.Status), CurrentStep: inst.CurrentStep.String}
	lastErr := ""
	for _, ex := range execs {
		st := TimelineStep{Step: ex.StepName, Status: string(ex.Status), Attempt: ex.Attempt}
		if ex.FinishedAt.Valid {
			t := ex.FinishedAt.Time
			st.FinishedAt = &t
		}
		if ex.Status == store.StepStatusFailed && ex.Error.Valid {
			lastErr = ex.Error.String
		}
		tl.Steps = append(tl.Steps, st)
	}
	if lastErr != "" && (inst.Status == store.WfStatusFailed || inst.Status == store.WfStatusCompensated) {
		tl.FailureCode = failureCode(lastErr)
	}
	return tl, nil
}

func noteText(n string) pgtype.Text {
	if n == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: n, Valid: true}
}

type EngineRequestInput struct {
	ServiceCode   string
	SenderUserID  uuid.UUID
	ReceiverPhone string
	ReceiverID    *uuid.UUID
	Amount        int64
	Note          string
	Idempotency   string
	ExtraParams   map[string]interface{}
}

type EngineRequestResult struct {
	TransactionID uuid.UUID `json:"transaction_id"`
	RefCode       string    `json:"ref_code"`
	Status        string    `json:"status"`
	ServiceCode   string    `json:"service_code"`
	Amount        int64     `json:"amount"`
	Fee           int64     `json:"fee"`
	Total         int64     `json:"total"`
	AuthRequired  string    `json:"auth_required"` // "none", "pin", "otp", "2fa"
	CreatedAt     time.Time `json:"created_at"`
}

type EngineConfirmInput struct {
	TransactionID uuid.UUID
	SenderUserID  uuid.UUID
	PIN           string
	OTP           string
}

func (s *Service) ExecuteEngineRequest(ctx context.Context, input EngineRequestInput) (EngineRequestResult, error) {
	if input.Amount <= 0 {
		return EngineRequestResult{}, wallet.ErrAmountInvalid
	}

	// 1. Fetch Service config dynamically from Database
	srvCode := strings.ToUpper(strings.TrimSpace(input.ServiceCode))
	if srvCode == "" {
		srvCode = "P2P_TRANSFER"
	}
	srv, err := s.q.GetServiceByCode(ctx, srvCode)
	if err != nil {
		// Fallback to default P2P_TRANSFER service if code not found
		srv, err = s.q.GetServiceByCode(ctx, "P2P_TRANSFER")
		if err != nil {
			return EngineRequestResult{}, errors.New("service catalog unavailable")
		}
	}

	if srv.Status != store.ServiceStatusActive {
		return EngineRequestResult{}, errors.New("dịch vụ hiện đang bảo trì hoặc tạm ngưng")
	}

	// Dispatch profile: resolve ledger txn type + workflow definition + wallet
	// strategy from the service row (G1). Flag off falls back to legacy P2P.
	txnType := store.TxnTypeP2p
	defName := s.defName
	strategy := "user_to_user"
	if s.dynamicDispatch {
		txnType = srv.LedgerTxnType
		defName = srv.WorkflowName
		strategy = srv.WalletStrategy
	}

	// 2. OfferGuard: package limits + required auth method (G2/G3). Runs before
	// any wallet work so an over-limit request never reaches the engine.
	authReq := "pin"
	if s.guardOn {
		decision, gerr := s.guard.Check(ctx, input.SenderUserID, srv.Code, txnType, input.Amount)
		if gerr != nil {
			return EngineRequestResult{}, gerr
		}
		authReq = decision.AuthMethod
	}

	// 3. Resolve Sender & Receiver Wallets per strategy
	sender, err := s.q.GetUserStatusByID(ctx, input.SenderUserID)
	if err != nil {
		return EngineRequestResult{}, err
	}

	plan, err := s.planWallets(ctx, strategy, input)
	if err != nil {
		return EngineRequestResult{}, err
	}

	// 3. Fee Preview & Balance Check (fail-fast; hard checks re-run in steps)
	feeAmt, err := s.fees.For(ctx, txnType, sender.KycStatus, input.Amount)
	if err != nil {
		return EngineRequestResult{}, err
	}

	if plan.checkBalance && plan.senderBalance < input.Amount+feeAmt {
		return EngineRequestResult{}, ledger.ErrInsufficientBalance
	}

	// 4. Create Transaction Record
	idemKey := input.Idempotency
	if idemKey == "" {
		idemKey = uuid.NewString()
	}

	refCode := wallet.NewRefCode()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return EngineRequestResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	txn, err := qtx.InsertTransaction(ctx, store.InsertTransactionParams{
		RefCode:          refCode,
		IdempotencyKey:   pgtype.Text{String: idemKey, Valid: idemKey != ""},
		Type:             txnType,
		SenderWalletID:   plan.senderWalletID,
		ReceiverWalletID: plan.receiverWalletID,
		Amount:           input.Amount,
		Fee:              feeAmt,
		Note:             noteText(input.Note),
		InitiatedBy:      input.SenderUserID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "idempotency") {
			old, gerr := s.q.GetTransactionByIdempotencyKey(ctx, pgtype.Text{String: idemKey, Valid: true})
			if gerr != nil {
				return EngineRequestResult{}, gerr
			}
			return EngineRequestResult{
				TransactionID: old.ID,
				RefCode:       old.RefCode,
				Status:        string(old.Status),
				ServiceCode:   srv.Code,
				Amount:        old.Amount,
				Fee:           old.Fee,
				Total:         old.Amount + old.Fee,
				AuthRequired:  authReq,
				CreatedAt:     old.CreatedAt.Time,
			}, nil
		}
		return EngineRequestResult{}, err
	}

	if _, err := s.engine.Start(ctx, tx, defName, txn.ID, map[string]any{
		"ref_code": txn.RefCode,
		// Lets the action_pipeline step resolve this service's configured actions.
		"service_code": srv.Code,
	}); err != nil {
		return EngineRequestResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return EngineRequestResult{}, err
	}

	// Auth requirement already resolved from offer_auth_matrix by OfferGuard (G3).
	return EngineRequestResult{
		TransactionID: txn.ID,
		RefCode:       txn.RefCode,
		Status:        string(txn.Status),
		ServiceCode:   srv.Code,
		Amount:        input.Amount,
		Fee:           feeAmt,
		Total:         input.Amount + feeAmt,
		AuthRequired:  authReq,
		CreatedAt:     txn.CreatedAt.Time,
	}, nil
}

type enginePlan struct {
	senderWalletID   uuid.UUID
	receiverWalletID uuid.UUID
	senderBalance    int64
	checkBalance     bool
}

// planWallets resolves the debit/credit wallets for an engine transaction based
// on the service's wallet_strategy (G1). user_to_user, syscash_to_user (topup)
// and user_to_vault (savings) are wired; org_to_user is still deferred.
func (s *Service) planWallets(ctx context.Context, strategy string, input EngineRequestInput) (enginePlan, error) {
	switch strategy {
	case "user_to_vault": // VAULT_SAVINGS: main <-> one of the caller's vaults
		// Both wallets belong to the caller, so the only thing that can be
		// abused here is the vault id — resolved owner-scoped below.
		rawID, _ := input.ExtraParams["vault_id"].(string)
		vaultID, err := uuid.Parse(rawID)
		if err != nil {
			return enginePlan{}, vault.ErrNotFound
		}
		v, err := s.vaults.Owned(ctx, input.SenderUserID, vaultID)
		if err != nil {
			return enginePlan{}, err
		}
		main, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: input.SenderUserID, Currency: "VND"})
		if err != nil {
			return enginePlan{}, wallet.ErrWalletNotFound
		}

		direction, _ := input.ExtraParams["direction"].(string)
		switch direction {
		case "deposit":
			return enginePlan{
				senderWalletID: main.ID, receiverWalletID: v.ID,
				senderBalance: main.Balance, checkBalance: true,
			}, nil
		case "withdraw":
			return enginePlan{
				senderWalletID: v.ID, receiverWalletID: main.ID,
				senderBalance: v.Balance, checkBalance: true,
			}, nil
		default:
			return enginePlan{}, vault.ErrDirectionInvalid
		}

	case "syscash_to_user": // TOPUP_BANK: SYSTEM_CASH_IN -> user's own wallet
		sys, err := s.q.GetSystemWallet(ctx, pgtype.Text{String: "SYSTEM_CASH_IN", Valid: true})
		if err != nil {
			return enginePlan{}, wallet.ErrWalletNotFound
		}
		usr, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: input.SenderUserID, Currency: "VND"})
		if err != nil {
			return enginePlan{}, wallet.ErrWalletNotFound
		}
		return enginePlan{senderWalletID: sys.ID, receiverWalletID: usr.ID, checkBalance: false}, nil

	case "user_to_user": // P2P_TRANSFER / MONEY_REQUEST: user -> user
		var receiverID uuid.UUID
		if input.ReceiverID != nil {
			receiverID = *input.ReceiverID
		} else if input.ReceiverPhone != "" {
			rcv, err := s.q.GetUserByPhone(ctx, input.ReceiverPhone)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return enginePlan{}, ErrReceiverNotFound
				}
				return enginePlan{}, err
			}
			receiverID = rcv.ID
		} else {
			return enginePlan{}, errors.New("thiếu thông tin người nhận")
		}
		if receiverID == input.SenderUserID {
			return enginePlan{}, ledger.ErrSelfTransfer
		}
		senderWallet, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: input.SenderUserID, Currency: "VND"})
		if err != nil {
			return enginePlan{}, wallet.ErrWalletNotFound
		}
		receiverWallet, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: receiverID, Currency: "VND"})
		if err != nil {
			return enginePlan{}, ErrReceiverNotFound
		}
		return enginePlan{
			senderWalletID:   senderWallet.ID,
			receiverWalletID: receiverWallet.ID,
			senderBalance:    senderWallet.Balance,
			checkBalance:     true,
		}, nil

	default:
		return enginePlan{}, fmt.Errorf("wallet_strategy chưa hỗ trợ qua engine dispatch: %s", strategy)
	}
}

// ExecuteEngineConfirm acknowledges auth (PIN/OTP) for a transaction already
// in flight. It never touches the ledger itself — VALIDATE/DEBIT/CREDIT run
// as workflow steps dispatched by ExecuteEngineRequest's engine.Start, and
// posting here too would double-move funds. This just reports current state.
func (s *Service) ExecuteEngineConfirm(ctx context.Context, input EngineConfirmInput) (Result, error) {
	txn, err := s.q.GetTransactionByWorkflow(ctx, input.TransactionID)
	if err != nil {
		return Result{}, errors.New("không tìm thấy giao dịch")
	}
	return Result{
		RefCode:   txn.RefCode,
		Status:    string(txn.Status),
		Amount:    txn.Amount,
		Fee:       txn.Fee,
		Total:     txn.Amount + txn.Fee,
		CreatedAt: time.Now(),
	}, nil
}

func (s *Service) ExecuteEngineVerify(ctx context.Context, txnID, senderID uuid.UUID, code string) (Result, error) {
	return s.ExecuteEngineConfirm(ctx, EngineConfirmInput{
		TransactionID: txnID,
		SenderUserID:  senderID,
		OTP:           code,
	})
}

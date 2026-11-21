// Package handlers implements the business step handlers for p2p_transfer
// (docs f5). RegisterAll wires them into the worker registry.
package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

// RegisterAll wires the money handlers. The fraud handler is registered
// separately by internal/fraud (it needs an AI advisor + logger) — F9 replaced
// the F5 FraudStub with the real rule engine.
func RegisterAll(reg *workflow.Registry, pool *pgxpool.Pool) {
	reg.Register(&Validator{pool: pool})
	reg.Register(&Wallet{pool: pool})
	reg.Register(&Confirmer{pool: pool})
}

func txnForStep(ctx context.Context, q *store.Queries, in workflow.StepInput) (store.GetTransactionByWorkflowRow, error) {
	return q.GetTransactionByWorkflow(ctx, in.TransactionID)
}

// --- VALIDATE ---------------------------------------------------------------

type Validator struct{ pool *pgxpool.Pool }

func (v *Validator) Name() string { return "validator" }

func (v *Validator) Execute(ctx context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	if in.Compensating {
		return workflow.StepOutput{}, nil // nothing to reverse
	}
	q := store.New(v.pool)
	txn, err := txnForStep(ctx, q, in)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	// Owner-generic status check: user wallets check users.status, org wallets
	// check organizations.status (F6) — same E12 semantics for both.
	senderStatus, err := q.GetOwnerStatusByWalletID(ctx, txn.SenderWalletID)
	if err != nil {
		return workflow.StepOutput{}, fmt.Errorf("sender lookup: %w", err)
	}
	if senderStatus != store.AccountStatusActive {
		return workflow.StepOutput{}, errors.New("sender account not active")
	}
	receiverStatus, err := q.GetOwnerStatusByWalletID(ctx, txn.ReceiverWalletID)
	if err != nil {
		return workflow.StepOutput{}, fmt.Errorf("receiver lookup: %w", err)
	}
	if receiverStatus != store.AccountStatusActive {
		return workflow.StepOutput{}, errors.New("receiver account not active")
	}
	w, err := q.GetWalletByID(ctx, txn.SenderWalletID)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	if w.Balance < txn.Amount+txn.Fee {
		return workflow.StepOutput{}, ledger.ErrInsufficientBalance // soft pre-check; hard check in MOVE_FUNDS
	}
	return workflow.StepOutput{Context: map[string]any{"validated": true}}, nil
}

// FRAUD_CHECK is now internal/fraud.handler (F9) — real rule engine + advisory.

// --- MOVE_FUNDS ---------------------------------------------------------------

type Wallet struct{ pool *pgxpool.Pool }

func (w *Wallet) Name() string { return "wallet" }

// Execute moves amount+fee atomically (forward) or reverses the exact same
// legs (compensating) — both derived from the immutable transaction row.
func (w *Wallet) Execute(ctx context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	defer tx.Rollback(ctx)
	q := store.New(tx)

	txn, err := txnForStep(ctx, q, in)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}

	// E12: statuses may have changed since VALIDATE — re-check inside the tx.
	// Owner-generic (F6): works for user AND organization wallets.
	if !in.Compensating {
		senderStatus, err := q.GetOwnerStatusByWalletID(ctx, txn.SenderWalletID)
		if err != nil || senderStatus != store.AccountStatusActive {
			return workflow.StepOutput{}, errors.New("sender not active at posting time")
		}
		receiverStatus, err := q.GetOwnerStatusByWalletID(ctx, txn.ReceiverWalletID)
		if err != nil || receiverStatus != store.AccountStatusActive {
			return workflow.StepOutput{}, errors.New("receiver not active at posting time")
		}
	}

	legs, err := w.legs(ctx, q, txn, in.Compensating)
	if err != nil {
		return workflow.StepOutput{}, err
	}
	if err := ledger.Post(ctx, tx, txn.ID, legs); err != nil {
		if errors.Is(err, ledger.ErrInsufficientBalance) || errors.Is(err, ledger.ErrWalletNotFound) {
			return workflow.StepOutput{}, err // terminal
		}
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	if err := tx.Commit(ctx); err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	return workflow.StepOutput{Context: map[string]any{"funds_moved": !in.Compensating}}, nil
}

func (w *Wallet) legs(ctx context.Context, q *store.Queries, txn store.GetTransactionByWorkflowRow, compensate bool) ([]ledger.Leg, error) {
	dr, cr := store.EntryDirectionDebit, store.EntryDirectionCredit
	if compensate {
		dr, cr = cr, dr // exact mirror of the forward posting
	}
	legs := []ledger.Leg{
		{WalletID: txn.SenderWalletID, Direction: dr, Amount: txn.Amount + txn.Fee},
		{WalletID: txn.ReceiverWalletID, Direction: cr, Amount: txn.Amount},
	}
	if txn.Fee > 0 {
		feeWallet, err := q.GetSystemWallet(ctx, pgtype.Text{String: "SYSTEM_FEE", Valid: true})
		if err != nil {
			return nil, workflow.RetryableError{Err: err}
		}
		legs = append(legs, ledger.Leg{WalletID: feeWallet.ID, Direction: cr, Amount: txn.Fee})
	}
	return legs, nil
}

// --- CONFIRM ------------------------------------------------------------------

type Confirmer struct{ pool *pgxpool.Pool }

func (c *Confirmer) Name() string { return "confirmer" }

func (c *Confirmer) Execute(ctx context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	if in.Compensating {
		return workflow.StepOutput{}, nil
	}
	q := store.New(c.pool)
	if _, err := q.CompleteTransaction(ctx, in.TransactionID); err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	// F-later: notification push both sides.
	return workflow.StepOutput{Context: map[string]any{"confirmed": true}}, nil
}

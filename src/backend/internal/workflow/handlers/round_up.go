package handlers

import (
	"context"
	"errors"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

// RoundUpUnit is the rounding granularity in minor units (VND). Docs 04 §2.2
// treats round-up as a stretch, system-wide constant rather than a per-user
// setting; ROUND_UP_UNIT_VND lets tests override it.
var RoundUpUnit int64 = roundUpUnitFromEnv()

func roundUpUnitFromEnv() int64 {
	if v := os.Getenv("ROUND_UP_UNIT_VND"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 10_000
}

// RoundUp is the optional ROUND_UP step appended after CONFIRM in
// p2p_transfer v2 (docs 04 §2.2). It sweeps the rounding difference of a P2P
// spend into the sender's designated round-up vault. Every "cannot apply"
// branch is a no-op success — CONFIRM has already completed the transaction
// by the time this step runs, so ROUND_UP must never turn that into a
// failure for a purely optional, best-effort side effect.
type RoundUp struct{ pool *pgxpool.Pool }

// RegisterRoundUp wires the round_up handler. Registered separately from
// RegisterAll for the same reason as action_pipeline: it is optional and only
// meaningful once the workflow definition actually includes a ROUND_UP step.
func RegisterRoundUp(reg *workflow.Registry, pool *pgxpool.Pool) {
	reg.Register(&RoundUp{pool: pool})
}

func (r *RoundUp) Name() string { return "round_up" }

func (r *RoundUp) Execute(ctx context.Context, in workflow.StepInput) (workflow.StepOutput, error) {
	if in.Compensating {
		// The round-up leg is itself best-effort; nothing upstream depends on
		// it, so there is nothing to reverse.
		return workflow.StepOutput{}, nil
	}
	q := store.New(r.pool)
	txn, err := txnForStep(ctx, q, in)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}

	// Round-up only applies to an actual spend (P2P_TRANSFER / MONEY_REQUEST).
	// TOPUP_BANK (system sender) and VAULT_SAVINGS (vault_move) also run this
	// workflow definition but are not "chi tiêu lẻ" — skip them.
	if txn.Type != store.TxnTypeP2p {
		return skipRoundUp("not_a_spend"), nil
	}

	owner, err := q.GetWalletOwner(ctx, txn.SenderWalletID)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	if owner.OwnerType != store.OwnerTypeUser {
		return skipRoundUp("sender_not_user"), nil
	}

	senderWallet, err := q.GetWalletByID(ctx, txn.SenderWalletID)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	if senderWallet.Purpose != "main" {
		return skipRoundUp("sender_not_main"), nil
	}

	diff := RoundUpUnit - (txn.Amount % RoundUpUnit)
	if diff == RoundUpUnit {
		return skipRoundUp("exact"), nil // divides evenly, nothing to round
	}

	target, err := q.GetRoundUpTargetVault(ctx, store.GetRoundUpTargetVaultParams{
		OwnerID: owner.OwnerID, Currency: senderWallet.Currency,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return skipRoundUp("no_target"), nil
		}
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}

	// Balance already reflects MOVE_FUNDS; re-check fresh at post time too
	// (ledger.Post itself rejects an overdraft), this is just to skip quietly
	// instead of retrying a posting that will never succeed.
	if senderWallet.Balance < diff {
		return skipRoundUp("insufficient_balance"), nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	defer tx.Rollback(ctx)

	legs := []ledger.Leg{
		{WalletID: txn.SenderWalletID, Direction: store.EntryDirectionDebit, Amount: diff},
		{WalletID: target.ID, Direction: store.EntryDirectionCredit, Amount: diff},
	}
	if err := ledger.Post(ctx, tx, txn.ID, legs); err != nil {
		if errors.Is(err, ledger.ErrInsufficientBalance) || errors.Is(err, ledger.ErrWalletNotFound) {
			// Balance moved between our check and posting (e.g. a concurrent
			// spend) — skip rather than fail an already-completed transaction.
			return skipRoundUp(err.Error()), nil
		}
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}
	if err := tx.Commit(ctx); err != nil {
		return workflow.StepOutput{}, workflow.RetryableError{Err: err}
	}

	return workflow.StepOutput{Context: map[string]any{
		"round_up_applied": true,
		"round_up_amount":  diff,
	}}, nil
}

func skipRoundUp(reason string) workflow.StepOutput {
	return workflow.StepOutput{Context: map[string]any{
		"round_up_applied":        false,
		"round_up_skipped_reason": reason,
	}}
}

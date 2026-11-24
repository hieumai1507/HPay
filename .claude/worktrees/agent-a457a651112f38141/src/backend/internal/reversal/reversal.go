// Package reversal implements admin transaction reversal as a maker-checker
// approval subject (docs 04 §1: a passive opposite posting, NOT through the
// workflow engine). Admin A files a request; a different admin B approves;
// the executor posts the mirror ledger entries and marks the original
// reversed — all inside the approval finalize transaction.
package reversal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
)

const reversalTTL = 24 * time.Hour

var (
	ErrTxnNotFound     = errors.New("transaction not found")
	ErrNotReversible   = errors.New("only completed transactions can be reversed")
	ErrAlreadyReversed = errors.New("transaction already reversed")
	ErrReversalPending = errors.New("a reversal request is already pending for this transaction")
)

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: store.New(pool)}
}

// RegisterExecutor plugs the reversal effect into the approval engine. The
// executor works purely on the tx-scoped queries the finalize step hands it.
func RegisterExecutor(apr *approval.Service) {
	apr.RegisterExecutor(store.ApprovalSubjectReversal, executor{}.run)
}

type CreateResult struct {
	RequestID uuid.UUID `json:"request_id"`
	RefCode   string    `json:"ref_code"`
}

// CreateRequest is admin A's action: validate the target is reversible and
// file an approval request (single-signature snapshot, no org/policy).
func (s *Service) CreateRequest(ctx context.Context, adminID uuid.UUID, refCode, reason string) (CreateResult, error) {
	txn, err := s.q.GetTransactionByRefCode(ctx, refCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CreateResult{}, ErrTxnNotFound
		}
		return CreateResult{}, err
	}
	if txn.Type == store.TxnTypeReversal {
		return CreateResult{}, ErrNotReversible // a reversal cannot itself be reversed (AC-3)
	}
	switch txn.Status {
	case store.TxnStatusCompleted:
		// reversible
	case store.TxnStatusReversed:
		return CreateResult{}, ErrAlreadyReversed
	default:
		return CreateResult{}, ErrNotReversible
	}

	exists, err := s.q.ExistsPendingReversalForTxn(ctx, txn.ID)
	if err != nil {
		return CreateResult{}, err
	}
	if exists {
		return CreateResult{}, ErrReversalPending
	}

	snap, err := json.Marshal(approval.PolicySnapshot{
		Name: "admin_reversal", Version: 1,
		Tiers: []approval.Tier{{Level: 1, RequiredApprovals: 1}},
	})
	if err != nil {
		return CreateResult{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CreateResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	reqID, err := qtx.InsertApprovalRequest(ctx, store.InsertApprovalRequestParams{
		SubjectType: store.ApprovalSubjectReversal, SubjectID: txn.ID,
		OrgID: uuid.Nil, InstanceID: uuid.Nil, PolicyID: uuid.Nil,
		PolicySnapshot: snap, CreatedBy: adminID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(reversalTTL), Valid: true},
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return CreateResult{}, ErrReversalPending // UNIQUE(subject_type, subject_id)
		}
		return CreateResult{}, err
	}
	if err := qtx.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: adminID, ActorType: "admin", Action: "reversal.request",
		Resource: "transaction " + txn.RefCode,
		Reason:   pgtype.Text{String: reason, Valid: reason != ""},
	}); err != nil {
		return CreateResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreateResult{}, err
	}
	return CreateResult{RequestID: reqID, RefCode: txn.RefCode}, nil
}

// executor performs the passive reversal inside the finalize transaction.
type executor struct{}

func (executor) run(ctx context.Context, q *store.Queries, fin approval.Finalized) error {
	if !fin.Approved {
		return nil // rejected/expired: original stays completed, nothing to do
	}
	// Lock the original and require it to still be completed (AC-4): the state
	// check and the work that depends on it must be atomic.
	orig, err := q.GetTransactionForUpdate(ctx, fin.SubjectID)
	if err != nil {
		return err
	}
	if orig.Status != store.TxnStatusCompleted {
		return fmt.Errorf("%w: %s is %s", ErrNotReversible, orig.RefCode, orig.Status)
	}

	// Insert the reversal transaction: sender/receiver swapped so the display
	// direction reads naturally; the ledger legs below carry the real money.
	// initiated_by reuses the original's user (FK-valid, and the acting admin
	// is already recorded in approval_actions + audit_logs).
	rev, err := q.InsertReversalTransaction(ctx, store.InsertReversalTransactionParams{
		RefCode: wallet.NewRefCode(), IdempotencyKey: pgtype.Text{String: "rev-" + orig.ID.String(), Valid: true},
		SenderWalletID: orig.ReceiverWalletID, ReceiverWalletID: orig.SenderWalletID,
		Amount: orig.Amount, Fee: orig.Fee,
		Note:        pgtype.Text{String: "reversal of " + orig.RefCode, Valid: true},
		InitiatedBy: orig.InitiatedBy, ReversalOf: orig.ID,
	})
	if err != nil {
		return err
	}

	// Mirror the original posting: refund the sender (amount+fee), pull back
	// the amount from the receiver, and pull the fee back out of SYSTEM_FEE —
	// exactly the compensating legs proven in F5's E07 test.
	legs := []ledger.Leg{
		{WalletID: orig.SenderWalletID, Direction: store.EntryDirectionCredit, Amount: orig.Amount + orig.Fee},
		{WalletID: orig.ReceiverWalletID, Direction: store.EntryDirectionDebit, Amount: orig.Amount},
	}
	if orig.Fee > 0 {
		feeWallet, err := q.GetSystemWallet(ctx, pgtype.Text{String: "SYSTEM_FEE", Valid: true})
		if err != nil {
			return err
		}
		legs = append(legs, ledger.Leg{WalletID: feeWallet.ID, Direction: store.EntryDirectionDebit, Amount: orig.Fee})
	}
	if err := ledger.PostWithQueries(ctx, q, rev.ID, ledger.KindForward, legs); err != nil {
		if errors.Is(err, ledger.ErrInsufficientBalance) {
			// The receiver no longer holds the money (BE-H4, ADR-0009/0013): park the
			// request for a person instead of losing the approver's decision.
			return approval.NeedsManual("reversal.needs_manual", "transaction "+orig.RefCode,
				"receiver wallet cannot cover the reversal amount")
		}
		return err
	}

	n, err := q.MarkTransactionReversed(ctx, orig.ID)
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("%w: %s changed state during reversal", ErrNotReversible, orig.RefCode)
	}
	return nil
}

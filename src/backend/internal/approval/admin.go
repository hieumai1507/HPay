package approval

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

	"github.com/hieumai1507/hpay-backend/internal/listing"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// ErrSelfApproval enforces Maker ≠ Checker for admin operations (docs 04 §2.3:
// the separation applies to internal operations too).
var ErrSelfApproval = errors.New("cannot approve your own request")

// AdminInbox lists pending admin-operation requests (reversal / account_lock /
// policy_change) for the admin "My Tasks" queue.
func (s *Service) AdminInbox(ctx context.Context, f listing.Filter) ([]store.ListPendingAdminApprovalsRow, int64, error) {
	f = f.Normalize()
	rows, err := s.q.ListPendingAdminApprovals(ctx, store.ListPendingAdminApprovalsParams{
		RefCode: f.Text(),
		Lim:     f.Limit,
		Off:     f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountPendingAdminApprovals(ctx, f.Text())
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// AdminNeedsManual lists admin requests parked as needs_manual (ADR-0013), oldest first.
// Read-only: resolving them is a separate, audited human step.
func (s *Service) AdminNeedsManual(ctx context.Context, f listing.Filter) ([]store.ListNeedsManualAdminApprovalsRow, int64, error) {
	f = f.Normalize()
	rows, err := s.q.ListNeedsManualAdminApprovals(ctx, store.ListNeedsManualAdminApprovalsParams{
		RefCode: f.Text(), Lim: f.Limit, Off: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountNeedsManualAdminApprovals(ctx, f.Text())
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// AdminDecide is the admin counterpart of Decide: no OTP (the admin is already
// authenticated with an aud=admin token; otp_challenges is user-scoped), but
// the same row-lock serialization, snapshot evaluation and in-tx executor.
// It enforces admin ≠ request creator and refuses org_transaction subjects
// (those go through the org checker flow).
func (s *Service) AdminDecide(ctx context.Context, adminID, requestID uuid.UUID, approve bool, reason string) (DecideResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DecideResult{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	req, err := q.GetApprovalRequestForUpdate(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DecideResult{}, ErrNotFound
		}
		return DecideResult{}, err
	}
	if req.SubjectType == store.ApprovalSubjectOrgTransaction {
		return DecideResult{}, ErrNotFound // admins don't decide org txns; don't reveal it exists
	}
	if req.Status != store.ApprovalStatusPending {
		return DecideResult{}, ErrAlreadyFinalized
	}
	if req.ExpiresAt.Valid && time.Now().After(req.ExpiresAt.Time) {
		return DecideResult{}, ErrExpired // same rule as the org path; the sweeper finalizes it
	}
	if req.CreatedBy == adminID {
		return DecideResult{}, ErrSelfApproval
	}

	action := store.ApprovalDecisionReject
	if approve {
		action = store.ApprovalDecisionApprove
	}
	if _, err := q.InsertApprovalAction(ctx, store.InsertApprovalActionParams{
		RequestID: req.ID, CheckerID: adminID, Level: req.CurrentLevel, Action: action,
	}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return DecideResult{}, ErrAlreadyActed
		}
		return DecideResult{}, err
	}

	actions, err := q.ListApprovalActions(ctx, req.ID)
	if err != nil {
		return DecideResult{}, err
	}
	views := make([]ActionView, 0, len(actions))
	for _, a := range actions {
		views = append(views, ActionView{Level: int(a.Level), Approve: a.Action == store.ApprovalDecisionApprove})
	}
	var snap PolicySnapshot
	if err := json.Unmarshal(req.PolicySnapshot, &snap); err != nil {
		return DecideResult{}, fmt.Errorf("policy snapshot corrupt: %w", err)
	}

	d := Evaluate(snap, views)
	res := DecideResult{Status: string(d.Status), CurrentLevel: d.CurrentLevel}
	switch d.Status {
	case DecisionPending:
		if err := q.SetApprovalCurrentLevel(ctx, store.SetApprovalCurrentLevelParams{
			ID: req.ID, CurrentLevel: int16(d.CurrentLevel),
		}); err != nil {
			return DecideResult{}, err
		}
	case DecisionApproved:
		parked, err := s.finalizeOrPark(ctx, tx, q, adminID, req.ID, reason)
		if err != nil {
			return DecideResult{}, err
		}
		if parked {
			res.Status = string(store.ApprovalStatusNeedsManual)
		}
	case DecisionRejected:
		if err := s.finalize(ctx, q, req.ID, store.ApprovalStatusRejected, false, reason); err != nil {
			return DecideResult{}, err
		}
	}

	verb := "reject"
	if approve {
		verb = "approve"
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: adminID, ActorType: "admin", Action: "approval." + verb,
		Resource: "approval_request " + req.ID.String(),
		Reason:   pgtype.Text{String: reason, Valid: reason != ""},
	}); err != nil {
		return DecideResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DecideResult{}, err
	}
	return res, nil
}

// finalizeOrPark approves the request and runs its executor inside a SAVEPOINT.
// If the executor reports a ManualError (ADR-0013) the savepoint is rolled back,
// which discards everything the executor did, while the approver's action row,
// written before this call in the outer transaction, is kept. The request is
// then parked as needs_manual and an audit row is written. Any other executor
// error still fails the whole decision. It returns parked=true in the manual case.
func (s *Service) finalizeOrPark(ctx context.Context, tx pgx.Tx, q *store.Queries, adminID, requestID uuid.UUID, reason string) (parked bool, err error) {
	sp, err := tx.Begin(ctx) // SAVEPOINT
	if err != nil {
		return false, err
	}
	ferr := s.finalize(ctx, q.WithTx(sp), requestID, store.ApprovalStatusApproved, true, reason)
	if ferr == nil {
		return false, sp.Commit(ctx) // RELEASE SAVEPOINT
	}
	_ = sp.Rollback(ctx) // ROLLBACK TO SAVEPOINT: the transaction stays usable
	me, ok := asManual(ferr)
	if !ok {
		return false, ferr
	}
	if _, err := q.MarkApprovalNeedsManual(ctx, requestID); err != nil {
		return false, err
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: adminID, ActorType: "admin", Action: me.Action,
		Resource: me.Resource + " (approval_request " + requestID.String() + ")",
		Reason:   pgtype.Text{String: me.Reason, Valid: me.Reason != ""},
	}); err != nil {
		return false, err
	}
	s.log.Warn("approval parked as needs_manual", "request", requestID, "action", me.Action, "resource", me.Resource)
	return true, nil
}

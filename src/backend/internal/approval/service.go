package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/auth"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

var (
	ErrNotFound         = errors.New("approval request not found")
	ErrAlreadyFinalized = errors.New("approval request already finalized")
	ErrAlreadyActed     = errors.New("checker already acted on this request")
	ErrWrongLevel       = errors.New("checker level does not match current tier")
	ErrNotChecker       = errors.New("member is not a checker")
	ErrExpired          = errors.New("approval request expired")
	ErrNoPolicy         = errors.New("no active approval policy matches this amount")
)

// Executor runs the subject's effect INSIDE the finalize DB transaction.
// F6 registers org_transaction (resume workflow); F9 adds reversal etc.
type Executor func(ctx context.Context, q *store.Queries, fin Finalized) error

// Finalized describes a request that just left pending state.
type Finalized struct {
	RequestID   uuid.UUID
	SubjectType store.ApprovalSubject
	SubjectID   uuid.UUID
	InstanceID  uuid.UUID // uuid.Nil when the subject has no workflow
	Approved    bool
	Reason      string
}

type Service struct {
	pool  *pgxpool.Pool
	q     *store.Queries
	otp   *auth.Service
	log   *slog.Logger
	execs map[store.ApprovalSubject]Executor
}

func NewService(pool *pgxpool.Pool, otp *auth.Service, log *slog.Logger) *Service {
	s := &Service{
		pool: pool, q: store.New(pool), otp: otp, log: log,
		execs: map[store.ApprovalSubject]Executor{},
	}
	s.execs[store.ApprovalSubjectOrgTransaction] = s.resumeWorkflow
	return s
}

// RegisterExecutor lets later phases (F9: reversal, account_lock,
// policy_change) plug in without touching the core decide path.
func (s *Service) RegisterExecutor(subject store.ApprovalSubject, exec Executor) {
	s.execs[subject] = exec
}

// otpPurpose binds a challenge to one request. purpose is VARCHAR(40):
// "apr:" + 32 hex chars = 36.
func otpPurpose(requestID uuid.UUID) string {
	return "apr:" + strings.ReplaceAll(requestID.String(), "-", "")
}

// --- pause hook (workflow.PauseHook) ----------------------------------------

type stepConfig struct {
	ExpiresH int `json:"expires_h"`
}

// OnPause runs inside the engine's advance transaction when an instance parks
// on a human_approval step: it snapshots the matching policy, creates the
// approval request and flips the transaction to pending_approval — atomically
// with the instance state change, so a paused instance ALWAYS has a request.
func (s *Service) OnPause(ctx context.Context, q *store.Queries, instanceID, transactionID uuid.UUID, step workflow.StepDef) error {
	if transactionID == uuid.Nil {
		return errors.New("human_approval step requires a transaction")
	}
	txn, err := q.GetTransactionByWorkflow(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("load transaction: %w", err)
	}
	owner, err := q.GetWalletOwner(ctx, txn.SenderWalletID)
	if err != nil {
		return fmt.Errorf("load sender wallet owner: %w", err)
	}
	if owner.OwnerType != store.OwnerTypeOrganization {
		return errors.New("human_approval is only wired for organization wallets (F6)")
	}
	pol, err := q.GetActivePolicyForAmount(ctx, store.GetActivePolicyForAmountParams{
		OrgID: owner.OwnerID, Amount: txn.Amount,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoPolicy
		}
		return err
	}
	var tiers []Tier
	if err := json.Unmarshal(pol.Tiers, &tiers); err != nil {
		return fmt.Errorf("policy tiers corrupt: %w", err)
	}
	if err := ValidateTiers(tiers); err != nil {
		return err
	}
	snap, err := json.Marshal(PolicySnapshot{
		PolicyID: pol.ID, Name: pol.Name, Version: int(pol.Version), Tiers: tiers,
	})
	if err != nil {
		return err
	}

	cfg := stepConfig{ExpiresH: 24}
	if len(step.Config) > 0 {
		_ = json.Unmarshal(step.Config, &cfg) // malformed config keeps default
	}
	if cfg.ExpiresH <= 0 {
		cfg.ExpiresH = 24
	}

	if _, err := q.InsertApprovalRequest(ctx, store.InsertApprovalRequestParams{
		SubjectType: store.ApprovalSubjectOrgTransaction, SubjectID: txn.ID,
		OrgID: owner.OwnerID, InstanceID: instanceID, PolicyID: pol.ID,
		PolicySnapshot: snap, CreatedBy: txn.InitiatedBy,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Duration(cfg.ExpiresH) * time.Hour), Valid: true},
	}); err != nil {
		return fmt.Errorf("create approval request: %w", err)
	}
	return q.PendTransactionGuarded(ctx, txn.ID)
}

// --- checker flow ------------------------------------------------------------

// Inbox is the checker's "My Tasks" queue.
func (s *Service) Inbox(ctx context.Context, checkerID uuid.UUID) ([]store.ListPendingApprovalsForCheckerRow, error) {
	return s.q.ListPendingApprovalsForChecker(ctx, checkerID)
}

// Challenge verifies eligibility BEFORE issuing an OTP (DEH's dry-run idea,
// docs 00 §5.1). Returns the code for the handler to expose in dev only.
func (s *Service) Challenge(ctx context.Context, checkerID, requestID uuid.UUID) (string, error) {
	req, err := s.q.GetApprovalRequest(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if err := s.eligibility(ctx, s.q, eligibilityInput{
		RequestID: req.ID, OrgID: req.OrgID, Status: req.Status,
		CurrentLevel: int(req.CurrentLevel), ExpiresAt: req.ExpiresAt.Time,
	}, checkerID); err != nil {
		return "", err
	}
	return s.otp.RequestOTPCode(ctx, checkerID, otpPurpose(requestID))
}

// Cancel lets the maker withdraw their own still-pending request — a
// different action from a checker's Decide, so it deliberately does NOT go
// through eligibility() (that function encodes "which checker may sign at
// this tier", not "who owns this request"). Reuses finalize()/the same
// org_transaction executor as reject, so the transaction fails the same way
// (no ledger entries, step marked failed) — cancelled is only a distinct
// status for reporting, not a distinct code path.
func (s *Service) Cancel(ctx context.Context, makerID, requestID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	req, err := q.GetApprovalRequestForUpdate(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if req.CreatedBy != makerID {
		return ErrNotFound // non-owners must not learn the request exists
	}
	if req.Status != store.ApprovalStatusPending {
		return ErrAlreadyFinalized
	}

	if err := s.finalize(ctx, q, req.ID, store.ApprovalStatusCancelled, false, "cancelled by maker"); err != nil {
		return err
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: makerID, ActorType: "user", Action: "approval.cancel",
		Resource: "approval_request " + req.ID.String(),
		Reason:   pgtype.Text{String: "cancelled by maker", Valid: true},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type eligibilityInput struct {
	RequestID    uuid.UUID
	OrgID        uuid.UUID
	Status       store.ApprovalStatus
	CurrentLevel int
	ExpiresAt    time.Time
}

// eligibility: right org, checker role, level matches the current tier, not
// yet acted, request still pending and not past its deadline.
func (s *Service) eligibility(ctx context.Context, q *store.Queries, in eligibilityInput, checkerID uuid.UUID) error {
	if in.Status != store.ApprovalStatusPending {
		return ErrAlreadyFinalized
	}
	if time.Now().After(in.ExpiresAt) {
		return ErrExpired // sweeper finalizes; decide must not race past it
	}
	m, err := q.GetOrgMember(ctx, store.GetOrgMemberParams{OrgID: in.OrgID, UserID: checkerID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound // non-members must not learn the request exists
		}
		return err
	}
	if m.Role != store.OrgRoleChecker {
		return ErrNotChecker
	}
	if int(m.CheckerLevel) != in.CurrentLevel {
		return ErrWrongLevel
	}
	acted, err := q.HasApprovalAction(ctx, store.HasApprovalActionParams{
		RequestID: in.RequestID, CheckerID: checkerID,
	})
	if err != nil {
		return err
	}
	if acted {
		return ErrAlreadyActed
	}
	return nil
}

// SignatureView is one recorded checker action, with the checker's display
// name resolved for the mobile timeline (maker status / checker signatures).
type SignatureView struct {
	CheckerID   uuid.UUID `json:"checker_id"`
	CheckerName string    `json:"checker_name"`
	Level       int       `json:"level"`
	Action      string    `json:"action"`
	ActedAt     time.Time `json:"acted_at"`
}

// Detail is the full status + signature history for one approval request,
// returned to the maker (created_by) or any member of the owning org.
type Detail struct {
	ID                 uuid.UUID       `json:"id"`
	Status             string          `json:"status"`
	CurrentLevel       int             `json:"current_level"`
	OrgID              uuid.UUID       `json:"org_id"`
	OrgName            string          `json:"org_name"`
	RefCode            string          `json:"ref_code"`
	Amount             int64           `json:"amount"`
	Fee                int64           `json:"fee"`
	MakerID            uuid.UUID       `json:"maker_id"`
	MakerName          string          `json:"maker_name"`
	CreatedAt          time.Time       `json:"created_at"`
	ExpiresAt          time.Time       `json:"expires_at"`
	FinalizedAt        *time.Time      `json:"finalized_at,omitempty"`
	PolicyName         string          `json:"policy_name"`
	SignaturesRequired int             `json:"signatures_required"`
	Signatures         []SignatureView `json:"signatures"`
}

// detailRow is the common shape of GetApprovalRequestDetail and
// GetApprovalRequestDetailByRefCode — both sqlc-generated row types have the
// same fields but are distinct Go types, so callers adapt into this one.
type detailRow struct {
	ID             uuid.UUID
	OrgID          uuid.UUID
	OrgName        string
	PolicySnapshot []byte
	Status         store.ApprovalStatus
	CurrentLevel   int16
	CreatedBy      uuid.UUID
	MakerName      string
	ExpiresAt      pgtype.Timestamptz
	CreatedAt      pgtype.Timestamptz
	FinalizedAt    pgtype.Timestamptz
	RefCode        string
	Amount         int64
	Fee            int64
}

// Detail loads one request for GET /v1/org/approvals/{id}. Only the request's
// maker or a member of its org may view it — anyone else gets ErrNotFound so
// existence is never revealed to unrelated users (same contract as
// eligibility()). Viewing is deliberately looser than deciding: any org
// member can see the signature history regardless of their own checker
// level, since Decide's stricter eligibility() check still gates the actual
// approve/reject action.
func (s *Service) Detail(ctx context.Context, viewerID, requestID uuid.UUID) (Detail, error) {
	row, err := s.q.GetApprovalRequestDetail(ctx, requestID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, ErrNotFound
		}
		return Detail{}, err
	}
	return s.buildDetail(ctx, viewerID, detailRow{
		ID: row.ID, OrgID: row.OrgID, OrgName: row.OrgName, PolicySnapshot: row.PolicySnapshot,
		Status: row.Status, CurrentLevel: row.CurrentLevel, CreatedBy: row.CreatedBy, MakerName: row.MakerName,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt, FinalizedAt: row.FinalizedAt,
		RefCode: row.RefCode, Amount: row.Amount, Fee: row.Fee,
	})
}

// DetailByRefCode is Detail keyed by the transaction ref_code instead of the
// approval request id — CreateDisbursement returns the ref_code synchronously
// but the approval_request row is only created later by the pause hook once
// the engine reaches WAIT_APPROVAL, so the maker's own client has no request
// id to call Detail with right after creating the disbursement. Returns
// ErrNotFound both when the transaction doesn't exist and when its approval
// request hasn't been created yet — the mobile client treats both the same
// (briefly shows a "processing" state and retries).
func (s *Service) DetailByRefCode(ctx context.Context, viewerID uuid.UUID, refCode string) (Detail, error) {
	row, err := s.q.GetApprovalRequestDetailByRefCode(ctx, refCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Detail{}, ErrNotFound
		}
		return Detail{}, err
	}
	return s.buildDetail(ctx, viewerID, detailRow{
		ID: row.ID, OrgID: row.OrgID, OrgName: row.OrgName, PolicySnapshot: row.PolicySnapshot,
		Status: row.Status, CurrentLevel: row.CurrentLevel, CreatedBy: row.CreatedBy, MakerName: row.MakerName,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt, FinalizedAt: row.FinalizedAt,
		RefCode: row.RefCode, Amount: row.Amount, Fee: row.Fee,
	})
}

func (s *Service) buildDetail(ctx context.Context, viewerID uuid.UUID, row detailRow) (Detail, error) {
	if viewerID != row.CreatedBy {
		if _, err := s.q.GetOrgMember(ctx, store.GetOrgMemberParams{OrgID: row.OrgID, UserID: viewerID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Detail{}, ErrNotFound
			}
			return Detail{}, err
		}
	}

	var snap PolicySnapshot
	if err := json.Unmarshal(row.PolicySnapshot, &snap); err != nil {
		return Detail{}, fmt.Errorf("policy snapshot corrupt: %w", err)
	}
	required := 0
	for _, t := range snap.Tiers {
		required += t.RequiredApprovals
	}

	actions, err := s.q.ListApprovalActionsWithNames(ctx, row.ID)
	if err != nil {
		return Detail{}, err
	}
	sigs := make([]SignatureView, 0, len(actions))
	for _, a := range actions {
		sigs = append(sigs, SignatureView{
			CheckerID: a.CheckerID, CheckerName: a.CheckerName,
			Level: int(a.Level), Action: string(a.Action), ActedAt: a.ActedAt.Time,
		})
	}

	var finalizedAt *time.Time
	if row.FinalizedAt.Valid {
		finalizedAt = &row.FinalizedAt.Time
	}

	return Detail{
		ID: row.ID, Status: string(row.Status), CurrentLevel: int(row.CurrentLevel),
		OrgID: row.OrgID, OrgName: row.OrgName,
		RefCode: row.RefCode, Amount: row.Amount, Fee: row.Fee,
		MakerID: row.CreatedBy, MakerName: row.MakerName,
		CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time, FinalizedAt: finalizedAt,
		PolicyName: snap.Name, SignaturesRequired: required, Signatures: sigs,
	}, nil
}

type DecideResult struct {
	Status       string `json:"status"` // pending | approved | rejected
	CurrentLevel int    `json:"current_level,omitempty"`
}

// Decide records one checker action under the request row lock and finalizes
// when the snapshot is satisfied (E08: FOR UPDATE serializes; UNIQUE blocks
// double-act; E09: evaluation reads the snapshot, never the live policy).
func (s *Service) Decide(ctx context.Context, checkerID, requestID uuid.UUID, approve bool, otpCode string) (DecideResult, error) {
	if err := s.otp.VerifyOTP(ctx, checkerID, otpPurpose(requestID), otpCode); err != nil {
		return DecideResult{}, err
	}

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
	if err := s.eligibility(ctx, q, eligibilityInput{
		RequestID: req.ID, OrgID: req.OrgID, Status: req.Status,
		CurrentLevel: int(req.CurrentLevel), ExpiresAt: req.ExpiresAt.Time,
	}, checkerID); err != nil {
		return DecideResult{}, err
	}

	action := store.ApprovalDecisionReject
	if approve {
		action = store.ApprovalDecisionApprove
	}
	if _, err := q.InsertApprovalAction(ctx, store.InsertApprovalActionParams{
		RequestID: req.ID, CheckerID: checkerID, Level: req.CurrentLevel, Action: action,
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
		if err := s.finalize(ctx, q, req.ID, store.ApprovalStatusApproved, true, "approved"); err != nil {
			return DecideResult{}, err
		}
	case DecisionRejected:
		if err := s.finalize(ctx, q, req.ID, store.ApprovalStatusRejected, false, "rejected by checker"); err != nil {
			return DecideResult{}, err
		}
	}

	verb := "reject"
	if approve {
		verb = "approve"
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: checkerID, ActorType: "user", Action: "approval." + verb,
		Resource: "approval_request " + req.ID.String(),
		Reason:   pgtype.Text{String: "level " + fmt.Sprint(req.CurrentLevel) + " -> " + res.Status, Valid: true},
	}); err != nil {
		return DecideResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DecideResult{}, err
	}
	return res, nil
}

// finalize flips pending -> terminal (guarded) and runs the subject executor
// in the SAME transaction — the resume event and the decision commit together.
func (s *Service) finalize(ctx context.Context, q *store.Queries, requestID uuid.UUID, status store.ApprovalStatus, approved bool, reason string) error {
	fin, err := q.FinalizeApprovalRequest(ctx, store.FinalizeApprovalRequestParams{
		ID: requestID, Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAlreadyFinalized // lost the race (E08/E10 claim)
		}
		return err
	}
	exec, ok := s.execs[fin.SubjectType]
	if !ok {
		s.log.Warn("no executor for approval subject; finalized without effect",
			"request", fin.ID, "subject_type", fin.SubjectType)
		return nil
	}
	return exec(ctx, q, Finalized{
		RequestID: fin.ID, SubjectType: fin.SubjectType, SubjectID: fin.SubjectID,
		InstanceID: fin.InstanceID, Approved: approved, Reason: reason,
	})
}

// resumeWorkflow is the org_transaction executor: it records the human gate in
// step_executions (timeline) and emits a standard step-result event through
// the outbox — the engine resumes exactly like after any auto step.
func (s *Service) resumeWorkflow(ctx context.Context, q *store.Queries, fin Finalized) error {
	if fin.InstanceID == uuid.Nil {
		return errors.New("org_transaction approval without workflow instance")
	}
	inst, err := q.GetInstance(ctx, fin.InstanceID)
	if err != nil {
		return err
	}
	if !inst.CurrentStep.Valid {
		return fmt.Errorf("instance %s has no current step to resume", fin.InstanceID)
	}
	stepName := inst.CurrentStep.String

	execStatus := store.StepStatusCompleted
	var errText pgtype.Text
	if !fin.Approved {
		execStatus = store.StepStatusFailed
		errText = pgtype.Text{String: fin.Reason, Valid: true}
	}
	if _, err := q.InsertStepExecution(ctx, store.InsertStepExecutionParams{
		InstanceID: fin.InstanceID, StepName: stepName, Attempt: 1,
		Status: execStatus, Input: nil, Output: nil, Error: errText,
	}); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return err
		}
		// already recorded by a previous finalize attempt — event below is
		// still safe: the engine's status guard drops duplicates (E06).
	}

	if fin.Approved {
		// MOVE_FUNDS / CONFIRM run under the processing status contract.
		if err := q.ResumeTransactionGuarded(ctx, fin.SubjectID); err != nil {
			return err
		}
	}

	payload, err := json.Marshal(workflow.ResultEvent{
		InstanceID: fin.InstanceID, Step: stepName, Attempt: 1,
		OK: fin.Approved, Error: errIfRejected(fin),
	})
	if err != nil {
		return err
	}
	_, err = q.InsertOutbox(ctx, store.InsertOutboxParams{
		Aggregate: "approval", EventType: workflow.EventStepResult, Payload: payload,
		AvailableAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	return err
}

func errIfRejected(fin Finalized) string {
	if fin.Approved {
		return ""
	}
	return fin.Reason
}

// --- expiry (E10) -------------------------------------------------------------

// ExpireDue finalizes overdue pending requests. Each request is claimed by the
// guarded UPDATE inside its own transaction: two sweepers cannot double-fire,
// and a crash loses nothing (claim + resume event commit together).
func (s *Service) ExpireDue(ctx context.Context) (int, error) {
	ids, err := s.q.ListDueApprovalIDs(ctx, 50)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if err := s.expireOne(ctx, id); err != nil {
			if errors.Is(err, ErrAlreadyFinalized) {
				continue // another sweeper won the claim
			}
			return n, err
		}
		n++
	}
	return n, nil
}

func (s *Service) expireOne(ctx context.Context, requestID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if err := s.finalize(ctx, q, requestID, store.ApprovalStatusExpired, false, "approval expired after deadline"); err != nil {
		return err
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: auth.SystemActor, ActorType: "system", Action: "approval.expire",
		Resource: "approval_request " + requestID.String(),
		Reason:   pgtype.Text{String: "expired by sweeper (E10)", Valid: true},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

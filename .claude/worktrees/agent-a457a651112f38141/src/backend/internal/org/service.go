// Package org owns organizations, membership, approval policies and the
// maker side of org disbursements (docs plans f6). Money still moves only
// through the workflow engine + ledger.Post.
package org

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieumai1507/hpay-backend/internal/approval"
	"github.com/hieumai1507/hpay-backend/internal/fee"
	"github.com/hieumai1507/hpay-backend/internal/ledger"
	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
	"github.com/hieumai1507/hpay-backend/internal/wallet"
	"github.com/hieumai1507/hpay-backend/internal/workflow"
)

var (
	ErrNotMember        = errors.New("not a member of this organization")
	ErrForbiddenRole    = errors.New("member role does not allow this action")
	ErrMemberExists     = errors.New("user is already a member")
	ErrUserNotFound     = errors.New("user not found")
	ErrOrgNotFound      = errors.New("organization not found")
	ErrOrgInactive      = errors.New("organization is not active")
	ErrReceiverNotFound = errors.New("receiver not found")
	ErrBadRole          = errors.New("invalid role or checker level")
	// ErrPolicyVersionConflict — createPolicy's next-version computation
	// (SELECT MAX(version)+1) isn't locked, so two concurrent saves for the
	// same (org_id, name) can race; the (org_id, name, version) unique
	// constraint (migration 000013) catches that as a real DB conflict
	// instead of silently inserting duplicate-version rows.
	ErrPolicyVersionConflict = errors.New("policy was changed concurrently — reload and try again")
)

// Per-transaction cap for org topup/disbursement (no KYC tiers for orgs).
const orgTxnLimit int64 = 1_000_000_000 // 1 billion VND

const disbursementDefinition = "org_disbursement"

type Service struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	engine *workflow.Engine
	fees   *fee.Service
}

func NewService(pool *pgxpool.Pool, engine *workflow.Engine, fees *fee.Service) *Service {
	return &Service{pool: pool, q: store.New(pool), engine: engine, fees: fees}
}

// --- org & members -----------------------------------------------------------

type CreateOrgResult struct {
	OrgID    uuid.UUID `json:"org_id"`
	WalletID uuid.UUID `json:"wallet_id"`
	Name     string    `json:"name"`
}

// CreateOrg creates the organization, its main wallet and the creator's admin
// membership in one transaction — no half-created orgs.
func (s *Service) CreateOrg(ctx context.Context, creatorID uuid.UUID, name string) (CreateOrgResult, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 200 {
		return CreateOrgResult{}, fmt.Errorf("%w: name must be 2-200 chars", ErrBadRole)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CreateOrgResult{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	o, err := q.InsertOrganization(ctx, store.InsertOrganizationParams{Name: name, CreatedBy: creatorID})
	if err != nil {
		return CreateOrgResult{}, err
	}
	w, err := q.CreateOrgWallet(ctx, store.CreateOrgWalletParams{OwnerID: o.ID, Currency: "VND"})
	if err != nil {
		return CreateOrgResult{}, err
	}
	if err := q.InsertOrgMember(ctx, store.InsertOrgMemberParams{
		OrgID: o.ID, UserID: creatorID, Role: store.OrgRoleAdmin,
	}); err != nil {
		return CreateOrgResult{}, err
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: creatorID, ActorType: "user", Action: "org.create",
		Resource: "organization " + o.ID.String(),
		Reason:   pgtype.Text{String: name, Valid: true},
	}); err != nil {
		return CreateOrgResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreateOrgResult{}, err
	}
	return CreateOrgResult{OrgID: o.ID, WalletID: w.ID, Name: o.Name}, nil
}

type ListOrgResult struct {
	OrgID        uuid.UUID `json:"org_id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CheckerLevel int       `json:"checker_level"`
	WalletID     uuid.UUID `json:"wallet_id"`
	Balance      int64     `json:"balance"`
}

// ListUserOrganizations retrieves all organizations the user is an active member of.
func (s *Service) ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]ListOrgResult, error) {
	rows, err := s.q.ListUserOrganizations(ctx, userID)
	if err != nil {
		return nil, err
	}
	res := make([]ListOrgResult, len(rows))
	for i, r := range rows {
		var bal int64
		if r.Balance.Valid {
			bal = r.Balance.Int64
		}
		res[i] = ListOrgResult{
			OrgID:        r.OrgID,
			Name:         r.Name,
			Role:         string(r.Role),
			CheckerLevel: int(r.CheckerLevel),
			WalletID:     r.WalletID,
			Balance:      bal,
		}
	}
	return res, nil
}

// AddMember lets an org admin add a user by phone.
func (s *Service) AddMember(ctx context.Context, actorID, orgID uuid.UUID, phone, roleStr string, checkerLevel int) error {
	if err := s.requireRole(ctx, orgID, actorID, store.OrgRoleAdmin); err != nil {
		return err
	}
	var role store.OrgRole
	switch roleStr {
	case "admin":
		role = store.OrgRoleAdmin
	case "maker":
		role = store.OrgRoleMaker
	case "checker":
		role = store.OrgRoleChecker
		if checkerLevel < 1 {
			return fmt.Errorf("%w: checker needs checker_level >= 1", ErrBadRole)
		}
	default:
		return fmt.Errorf("%w: role must be admin|maker|checker", ErrBadRole)
	}

	u, err := s.q.GetUserByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	params := store.InsertOrgMemberParams{OrgID: orgID, UserID: u.ID, Role: role}
	if role == store.OrgRoleChecker {
		params.CheckerLevel = pgtype.Int2{Int16: int16(checkerLevel), Valid: true}
	}
	if err := s.q.InsertOrgMember(ctx, params); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrMemberExists
		}
		return err
	}
	return s.q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: actorID, ActorType: "user", Action: "org.member_add",
		Resource: "organization " + orgID.String(),
		Reason:   pgtype.Text{String: roleStr + " " + u.ID.String(), Valid: true},
	})
}

// --- policies ------------------------------------------------------------------

type PolicyResult struct {
	PolicyID uuid.UUID `json:"policy_id"`
	Version  int       `json:"version"`
}

// CreatePolicy inserts a NEW version and deactivates the old one — in-flight
// requests keep their snapshot (E09); history stays queryable.
func (s *Service) CreatePolicy(ctx context.Context, actorID, orgID uuid.UUID, name string, minAmount int64, maxAmount *int64, tiers []approval.Tier) (PolicyResult, error) {
	if err := s.requireRole(ctx, orgID, actorID, store.OrgRoleAdmin); err != nil {
		return PolicyResult{}, err
	}
	return s.createPolicy(ctx, actorID, "user", orgID, name, minAmount, maxAmount, tiers)
}

// CreatePolicyAsAdmin is the platform-ops override path (hpay-admin "Tổ
// chức & Policy") — the caller is an admin_users row, not an org member, so
// it skips requireRole (an admin is never in org_members) and checks only
// that the org itself is real and active. approval_policies.created_by has
// no FK (migration 000012) specifically so this can record the admin's id
// without needing a matching users row.
func (s *Service) CreatePolicyAsAdmin(ctx context.Context, adminID, orgID uuid.UUID, name string, minAmount int64, maxAmount *int64, tiers []approval.Tier) (PolicyResult, error) {
	if err := s.requireActiveOrg(ctx, orgID); err != nil {
		return PolicyResult{}, err
	}
	return s.createPolicy(ctx, adminID, "admin", orgID, name, minAmount, maxAmount, tiers)
}

func (s *Service) createPolicy(ctx context.Context, actorID uuid.UUID, actorType string, orgID uuid.UUID, name string, minAmount int64, maxAmount *int64, tiers []approval.Tier) (PolicyResult, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return PolicyResult{}, fmt.Errorf("%w: policy name must be 1-100 chars", approval.ErrBadTiers)
	}
	if err := approval.ValidateTiers(tiers); err != nil {
		return PolicyResult{}, err
	}
	if minAmount < 0 || (maxAmount != nil && *maxAmount <= minAmount) {
		return PolicyResult{}, fmt.Errorf("%w: invalid amount range", approval.ErrBadTiers)
	}
	tiersJSON, err := json.Marshal(tiers)
	if err != nil {
		return PolicyResult{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PolicyResult{}, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	version, err := q.GetNextPolicyVersion(ctx, store.GetNextPolicyVersionParams{OrgID: orgID, Name: name})
	if err != nil {
		return PolicyResult{}, err
	}
	if err := q.DeactivateOrgPoliciesByName(ctx, store.DeactivateOrgPoliciesByNameParams{OrgID: orgID, Name: name}); err != nil {
		return PolicyResult{}, err
	}
	params := store.InsertApprovalPolicyParams{
		OrgID: orgID, Name: name, MinAmount: minAmount,
		Tiers: tiersJSON, Version: version, CreatedBy: actorID,
	}
	if maxAmount != nil {
		params.MaxAmount = pgtype.Int8{Int64: *maxAmount, Valid: true}
	}
	row, err := q.InsertApprovalPolicy(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "org_name_version") {
			return PolicyResult{}, ErrPolicyVersionConflict
		}
		return PolicyResult{}, err
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: actorID, ActorType: actorType, Action: "org.policy_change",
		Resource: "approval_policy " + row.ID.String(),
		Reason:   pgtype.Text{String: fmt.Sprintf("%s v%d tiers=%s", name, row.Version, string(tiersJSON)), Valid: true},
	}); err != nil {
		return PolicyResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PolicyResult{}, err
	}
	return PolicyResult{PolicyID: row.ID, Version: int(row.Version)}, nil
}

// --- money ---------------------------------------------------------------------

// Topup credits the org wallet from the mock bank (same sync pattern as user
// topup F3, same idempotency contract E01). Org admins only.
func (s *Service) Topup(ctx context.Context, actorID, orgID uuid.UUID, amount int64, idemKey string) (wallet.TopupResult, error) {
	if err := s.requireRole(ctx, orgID, actorID, store.OrgRoleAdmin); err != nil {
		return wallet.TopupResult{}, err
	}
	if amount <= 0 {
		return wallet.TopupResult{}, wallet.ErrAmountInvalid
	}
	if amount > orgTxnLimit {
		return wallet.TopupResult{}, wallet.ErrLimitExceeded
	}
	if err := s.requireActiveOrg(ctx, orgID); err != nil {
		return wallet.TopupResult{}, err
	}
	// ADR-0011: the key is scoped to the acting user and bound to this request.
	hash := wallet.RequestHash("org_topup", orgID.String(), strconv.FormatInt(amount, 10))
	if old, err := wallet.FindReplay(ctx, s.q, actorID, idemKey, hash); err != nil {
		return wallet.TopupResult{}, err
	} else if old != nil {
		return replayTopup(old), nil
	}
	orgWallet, err := s.q.GetOrgMainWallet(ctx, store.GetOrgMainWalletParams{OwnerID: orgID, Currency: "VND"})
	if err != nil {
		return wallet.TopupResult{}, wallet.ErrWalletNotFound
	}
	cashIn, err := s.q.GetSystemWallet(ctx, pgtype.Text{String: "SYSTEM_CASH_IN", Valid: true})
	if err != nil {
		return wallet.TopupResult{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return wallet.TopupResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	txn, err := qtx.InsertTransaction(ctx, store.InsertTransactionParams{
		RefCode: wallet.NewRefCode(), IdempotencyKey: wallet.KeyText(idemKey), RequestHash: wallet.HashText(hash),
		Type:           store.TxnTypeTopup,
		SenderWalletID: cashIn.ID, ReceiverWalletID: orgWallet.ID,
		Amount: amount, Fee: 0,
		Note: pgtype.Text{String: "org topup via mock bank", Valid: true}, InitiatedBy: actorID,
	})
	if err != nil {
		if wallet.IsIdempotencyConflict(err) { // a concurrent request with the same key won the race
			// the failed INSERT aborted this transaction: give its connection back before the replay lookup asks the pool for another one (a small pool deadlocks otherwise)
			_ = tx.Rollback(ctx)
			old, gerr := wallet.FindReplay(ctx, s.q, actorID, idemKey, hash)
			if gerr != nil {
				return wallet.TopupResult{}, gerr
			}
			if old != nil {
				return replayTopup(old), nil
			}
		}
		return wallet.TopupResult{}, err
	}
	if err := ledger.Post(ctx, tx, txn.ID, ledger.KindForward, []ledger.Leg{
		{WalletID: cashIn.ID, Direction: store.EntryDirectionDebit, Amount: amount},
		{WalletID: orgWallet.ID, Direction: store.EntryDirectionCredit, Amount: amount},
	}); err != nil {
		return wallet.TopupResult{}, err
	}
	if _, err := qtx.CompleteTransaction(ctx, txn.ID); err != nil {
		return wallet.TopupResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return wallet.TopupResult{}, err
	}
	return wallet.TopupResult{RefCode: txn.RefCode, Status: "completed", Amount: amount,
		NewBalance: orgWallet.Balance + amount, CreatedAt: txn.CreatedAt.Time}, nil
}

func replayTopup(old *store.GetUserTransactionByIdempotencyKeyRow) wallet.TopupResult {
	return wallet.TopupResult{RefCode: old.RefCode, Status: string(old.Status), Amount: old.Amount,
		CreatedAt: old.CreatedAt.Time, Duplicate: true}
}

func replayDisbursement(old *store.GetUserTransactionByIdempotencyKeyRow) DisbursementResult {
	return DisbursementResult{RefCode: old.RefCode, Status: string(old.Status), Amount: old.Amount,
		Fee: old.Fee, Total: old.Amount + old.Fee, CreatedAt: old.CreatedAt.Time, Duplicate: true}
}

type DisbursementResult struct {
	RefCode   string    `json:"ref_code"`
	Status    string    `json:"status"`
	Amount    int64     `json:"amount"`
	Fee       int64     `json:"fee"`
	Total     int64     `json:"total"`
	CreatedAt time.Time `json:"created_at"`
	Duplicate bool      `json:"duplicate,omitempty"`
}

// CreateDisbursement is the maker action: it fail-fasts on policy/balance for
// UX, then creates the transaction + workflow start atomically. The engine
// pauses at WAIT_APPROVAL where the pause hook snapshots the policy (F6).
func (s *Service) CreateDisbursement(ctx context.Context, actorID, orgID uuid.UUID, receiverPhone string, amount int64, note, idemKey string) (DisbursementResult, error) {
	if err := s.requireRole(ctx, orgID, actorID, store.OrgRoleMaker); err != nil {
		return DisbursementResult{}, err
	}
	if amount <= 0 {
		return DisbursementResult{}, wallet.ErrAmountInvalid
	}
	if amount > orgTxnLimit {
		return DisbursementResult{}, wallet.ErrLimitExceeded
	}
	if err := s.requireActiveOrg(ctx, orgID); err != nil {
		return DisbursementResult{}, err
	}

	hash := wallet.RequestHash("org_disbursement", orgID.String(), receiverPhone, strconv.FormatInt(amount, 10), note)
	if old, err := wallet.FindReplay(ctx, s.q, actorID, idemKey, hash); err != nil {
		return DisbursementResult{}, err
	} else if old != nil {
		return replayDisbursement(old), nil
	}

	receiver, err := s.q.GetUserByPhone(ctx, receiverPhone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DisbursementResult{}, ErrReceiverNotFound
		}
		return DisbursementResult{}, err
	}
	receiverWallet, err := s.q.GetUserMainWallet(ctx, store.GetUserMainWalletParams{OwnerID: receiver.ID, Currency: "VND"})
	if err != nil {
		return DisbursementResult{}, ErrReceiverNotFound
	}
	orgWallet, err := s.q.GetOrgMainWallet(ctx, store.GetOrgMainWalletParams{OwnerID: orgID, Currency: "VND"})
	if err != nil {
		return DisbursementResult{}, wallet.ErrWalletNotFound
	}

	// Fail fast when no policy could ever match — the pause hook would fail
	// the instance later, but the maker deserves an immediate answer.
	if _, err := s.q.GetActivePolicyForAmount(ctx, store.GetActivePolicyForAmountParams{
		OrgID: orgID, Amount: amount,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DisbursementResult{}, approval.ErrNoPolicy
		}
		return DisbursementResult{}, err
	}

	feeAmt, err := s.fees.For(ctx, store.TxnTypeOrgDisbursement, store.KycStatusNone, amount)
	if err != nil {
		return DisbursementResult{}, err
	}
	if orgWallet.Balance < amount+feeAmt {
		return DisbursementResult{}, ledger.ErrInsufficientBalance // soft; hard check in MOVE_FUNDS
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DisbursementResult{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	txn, err := qtx.InsertTransaction(ctx, store.InsertTransactionParams{
		RefCode: wallet.NewRefCode(), IdempotencyKey: wallet.KeyText(idemKey), RequestHash: wallet.HashText(hash),
		Type:           store.TxnTypeOrgDisbursement,
		SenderWalletID: orgWallet.ID, ReceiverWalletID: receiverWallet.ID,
		Amount: amount, Fee: feeAmt,
		Note: noteText(note), InitiatedBy: actorID,
	})
	if err != nil {
		if wallet.IsIdempotencyConflict(err) { // a concurrent request with the same key won the race
			// the failed INSERT aborted this transaction: give its connection back before the replay lookup asks the pool for another one (a small pool deadlocks otherwise)
			_ = tx.Rollback(ctx)
			old, gerr := wallet.FindReplay(ctx, s.q, actorID, idemKey, hash)
			if gerr != nil {
				return DisbursementResult{}, gerr
			}
			if old != nil {
				return replayDisbursement(old), nil
			}
		}
		return DisbursementResult{}, err
	}
	if _, err := s.engine.Start(ctx, tx, disbursementDefinition, txn.ID, map[string]any{"ref_code": txn.RefCode}); err != nil {
		return DisbursementResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DisbursementResult{}, err
	}
	return DisbursementResult{RefCode: txn.RefCode, Status: string(txn.Status), Amount: amount,
		Fee: feeAmt, Total: amount + feeAmt, CreatedAt: txn.CreatedAt.Time}, nil
}

// --- helpers --------------------------------------------------------------------

func (s *Service) requireRole(ctx context.Context, orgID, userID uuid.UUID, want store.OrgRole) error {
	m, err := s.q.GetOrgMember(ctx, store.GetOrgMemberParams{OrgID: orgID, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotMember
		}
		return err
	}
	if m.Role != want {
		return ErrForbiddenRole
	}
	return nil
}

func (s *Service) requireActiveOrg(ctx context.Context, orgID uuid.UUID) error {
	o, err := s.q.GetOrganization(ctx, orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrgNotFound
		}
		return err
	}
	if o.Status != store.AccountStatusActive {
		return ErrOrgInactive
	}
	return nil
}

func noteText(n string) pgtype.Text {
	if n == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: n, Valid: true}
}

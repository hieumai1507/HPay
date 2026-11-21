package auth

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// AdminService authenticates admin_users (docs 01 §3.1): separate table,
// separate JWT audience — a leaked user token can never call admin APIs.
// Access-only tokens for now; refresh comes with the admin SPA phases.
type AdminService struct {
	q      *store.Queries
	pool   *pgxpool.Pool
	jwt    *JWTIssuer // audience "admin"
	sender Sender     // security notices to users (ADR-0016)
	log    *slog.Logger
}

func NewAdminService(pool *pgxpool.Pool, issuer *JWTIssuer) *AdminService {
	return &AdminService{q: store.New(pool), pool: pool, jwt: issuer, log: slog.Default(), sender: NewSender(os.Getenv("APP_ENV"), slog.Default())}
}

type AdminInfo struct {
	ID       uuid.UUID `json:"id"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
}

func (s *AdminService) Login(ctx context.Context, email, password string) (AdminInfo, string, error) {
	row, err := s.q.GetAdminByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			burnPasswordWork(password)
			return AdminInfo{}, "", ErrInvalidCredentials
		}
		return AdminInfo{}, "", err
	}
	if lockActive(row.LoginLockedUntil.Time, row.LoginLockedUntil.Valid) {
		burnPasswordWork(password)
		return AdminInfo{}, "", ErrLoginLocked
	}
	if err := VerifyPassword(row.PasswordHash, password); err != nil {
		res, ferr := s.q.RecordAdminLoginFailure(ctx, row.ID)
		if ferr != nil {
			return AdminInfo{}, "", ferr
		}
		if lockActive(res.LoginLockedUntil.Time, res.LoginLockedUntil.Valid) {
			s.auditLogin(ctx, row.ID, "admin", "auth.login_locked", res.LoginFailedAttempts)
			return AdminInfo{}, "", ErrLoginLocked
		}
		s.auditLogin(ctx, row.ID, "admin", "auth.login_failed", res.LoginFailedAttempts)
		return AdminInfo{}, "", ErrInvalidCredentials
	}
	if row.LoginFailedAttempts != 0 || row.LoginLockedUntil.Valid {
		if err := s.q.ResetAdminLoginFailures(ctx, row.ID); err != nil {
			return AdminInfo{}, "", err
		}
	}
	if row.Status != store.AccountStatusActive {
		return AdminInfo{}, "", ErrAccountLocked
	}
	token, err := s.jwt.Issue(row.ID, "admin-console", time.Now())
	if err != nil {
		return AdminInfo{}, "", err
	}
	return AdminInfo{ID: row.ID, Email: row.Email, FullName: row.FullName, Role: string(row.Role)}, token, nil
}

// SetSender replaces the delivery channel (tests and wiring).
func (s *AdminService) SetSender(sender Sender) { s.sender = sender }

// ClearPIN is the operator fallback for a user who cannot receive a reset code (ADR-0016): it
// removes the user's PIN after out-of-band identity verification. It never reads or sets a PIN.
// The caller has already checked the pin.reset permission.
func (s *AdminService) ClearPIN(ctx context.Context, adminID, userID uuid.UUID, reason string) error {
	if reason == "" {
		return ErrReasonRequired
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	n, err := q.ClearUserPIN(ctx, userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPINNotSet // an unknown user and a user without a PIN look the same to the operator
	}
	if err := q.InsertAudit(ctx, store.InsertAuditParams{
		ActorID: adminID, ActorType: "admin", Action: "pin.clear", Resource: "user " + userID.String(),
		Reason: pgtype.Text{String: reason, Valid: true},
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if err := s.sender.Notify(ctx, userID, Notice{Kind: NoticePINReset, Text: "Support cleared your transaction PIN. Set a new one in the app. If you did not ask for this, contact support now."}); err != nil {
		s.log.Warn("pin clear notice not delivered", "user", userID, "err", err)
	}
	return nil
}

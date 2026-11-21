package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// AdminService authenticates admin_users (docs 01 §3.1): separate table,
// separate JWT audience — a leaked user token can never call admin APIs.
// Access-only tokens for now; refresh comes with the admin SPA phases.
type AdminService struct {
	q   *store.Queries
	jwt *JWTIssuer // audience "admin"
}

func NewAdminService(pool *pgxpool.Pool, issuer *JWTIssuer) *AdminService {
	return &AdminService{q: store.New(pool), jwt: issuer}
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

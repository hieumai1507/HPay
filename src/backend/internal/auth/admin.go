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
			return AdminInfo{}, "", ErrInvalidCredentials
		}
		return AdminInfo{}, "", err
	}
	if err := VerifyPassword(row.PasswordHash, password); err != nil {
		return AdminInfo{}, "", ErrInvalidCredentials
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

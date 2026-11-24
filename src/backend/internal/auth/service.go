package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	store "github.com/hieumai1507/hpay-backend/internal/store/gen"
)

// SystemActor is the fixed actor id for system-originated audit entries.
var SystemActor = uuid.MustParse("00000000-0000-0000-0000-000000000001")

var (
	ErrPhoneTaken         = errors.New("phone already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrTokenReused        = errors.New("refresh token reused")
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type UserInfo struct {
	ID        uuid.UUID `json:"id"`
	Phone     string    `json:"phone"`
	FullName  string    `json:"full_name"`
	KycStatus string    `json:"kyc_status"`
}

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
	jwt  *JWTIssuer
	log  *slog.Logger
	env  string
	// sender delivers one-time codes and security notices (ADR-0016).
	sender Sender
	// totpKey protects TOTP secrets at rest (ADR-0017); 32 bytes.
	totpKey []byte
	// pins holds the PIN pepper keys (ADR-0018); nil means PIN hashing fails closed.
	pins *PepperSet
}

func NewService(pool *pgxpool.Pool, jwtIssuer *JWTIssuer, log *slog.Logger, env string) *Service {
	return &Service{pool: pool, q: store.New(pool), jwt: jwtIssuer, log: log, env: env, sender: NewSender(env, log)}
}

func (s *Service) Register(ctx context.Context, phone, password, fullName, deviceID string) (UserInfo, TokenPair, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}
	row, err := s.q.InsertUser(ctx, store.InsertUserParams{Phone: phone, PasswordHash: hash, FullName: fullName})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return UserInfo{}, TokenPair{}, ErrPhoneTaken
		}
		return UserInfo{}, TokenPair{}, err
	}
	pair, err := s.issueTokens(ctx, row.ID, deviceID)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}
	return UserInfo{ID: row.ID, Phone: row.Phone, FullName: row.FullName, KycStatus: string(row.KycStatus)}, pair, nil
}

func (s *Service) Login(ctx context.Context, phone, password, deviceID string) (UserInfo, TokenPair, error) {
	row, err := s.q.GetUserByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			burnPasswordWork(password) // same cost as a known account: no timing oracle
			return UserInfo{}, TokenPair{}, ErrInvalidCredentials
		}
		return UserInfo{}, TokenPair{}, err
	}
	if lockActive(row.LoginLockedUntil.Time, row.LoginLockedUntil.Valid) {
		burnPasswordWork(password)
		return UserInfo{}, TokenPair{}, ErrLoginLocked // refused even for the correct password
	}
	if err := VerifyPassword(row.PasswordHash, password); err != nil {
		res, ferr := s.q.RecordUserLoginFailure(ctx, row.ID)
		if ferr != nil {
			return UserInfo{}, TokenPair{}, ferr
		}
		if lockActive(res.LoginLockedUntil.Time, res.LoginLockedUntil.Valid) {
			s.auditLogin(ctx, row.ID, "user", "auth.login_locked", res.LoginFailedAttempts)
			return UserInfo{}, TokenPair{}, ErrLoginLocked
		}
		s.auditLogin(ctx, row.ID, "user", "auth.login_failed", res.LoginFailedAttempts)
		return UserInfo{}, TokenPair{}, ErrInvalidCredentials
	}
	if row.LoginFailedAttempts != 0 || row.LoginLockedUntil.Valid {
		if err := s.q.ResetUserLoginFailures(ctx, row.ID); err != nil {
			return UserInfo{}, TokenPair{}, err
		}
	}
	if row.Status != store.AccountStatusActive {
		return UserInfo{}, TokenPair{}, ErrAccountLocked
	}
	pair, err := s.issueTokens(ctx, row.ID, deviceID)
	if err != nil {
		return UserInfo{}, TokenPair{}, err
	}
	return UserInfo{ID: row.ID, Phone: row.Phone, FullName: row.FullName, KycStatus: string(row.KycStatus)}, pair, nil
}

// Refresh rotates a refresh token. Reuse of an already-used token means theft:
// the whole family is revoked and the event is audited (docs plan f2 §2).
func (s *Service) Refresh(ctx context.Context, rawToken, ip string) (TokenPair, error) {
	row, err := s.q.GetRefreshTokenByHash(ctx, HashRefreshToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TokenPair{}, ErrTokenInvalid
		}
		return TokenPair{}, err
	}

	switch DecideRotation(tsPtr(row.UsedAt), tsPtr(row.RevokedAt), row.ExpiresAt.Time, time.Now()) {
	case RotationRevoked:
		return TokenPair{}, ErrTokenInvalid
	case RotationExpired:
		return TokenPair{}, ErrTokenExpired
	case RotationReused:
		if err := s.q.RevokeRefreshFamily(ctx, row.FamilyID); err != nil {
			return TokenPair{}, err
		}
		_ = s.q.InsertAudit(ctx, store.InsertAuditParams{
			ActorID: row.UserID, ActorType: "user", Action: "auth.token_reuse",
			Resource: "refresh_family " + row.FamilyID.String(),
			Reason:   pgtype.Text{String: "refresh token replayed; family revoked", Valid: true},
			Ip:       pgtype.Text{String: ip, Valid: ip != ""},
		})
		s.log.Warn("refresh token reuse detected", "user_id", row.UserID, "family_id", row.FamilyID)
		return TokenPair{}, ErrTokenReused
	}

	// Rotation OK: mark used + mint successor atomically.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return TokenPair{}, err
	}
	defer tx.Rollback(ctx)
	qtx := s.q.WithTx(tx)

	if _, err := qtx.MarkRefreshTokenUsed(ctx, row.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // concurrent refresh lost the race
			return TokenPair{}, ErrTokenInvalid
		}
		return TokenPair{}, err
	}
	newToken, newHash, err := NewRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	if _, err := qtx.InsertRefreshToken(ctx, store.InsertRefreshTokenParams{
		UserID: row.UserID, FamilyID: row.FamilyID, TokenHash: newHash,
		DeviceID: row.DeviceID, ExpiresAt: ts(time.Now().Add(RefreshTokenTTL)),
	}); err != nil {
		return TokenPair{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TokenPair{}, err
	}

	access, err := s.jwt.Issue(row.UserID, row.DeviceID, time.Now())
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: newToken, ExpiresIn: int64(AccessTokenTTL.Seconds())}, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	row, err := s.q.GetRefreshTokenByHash(ctx, HashRefreshToken(rawToken))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // logout is idempotent
		}
		return err
	}
	return s.q.RevokeRefreshFamily(ctx, row.FamilyID)
}

// RequestOTP creates a challenge; the code never leaves the server except the
// dev-env debug log (docs plan f2 §6).
func (s *Service) RequestOTP(ctx context.Context, userID uuid.UUID, purpose string) error {
	_, err := s.RequestOTPCode(ctx, userID, purpose)
	return err
}

// RequestOTPCode creates a challenge and returns the plaintext code to the
// CALLER ONLY — handlers may expose it in responses strictly when ENV=dev
// (docs 01 §3.1). It is never persisted or logged above debug.
func (s *Service) RequestOTPCode(ctx context.Context, userID uuid.UUID, purpose string) (string, error) {
	if last, err := s.q.GetActiveOTP(ctx, store.GetActiveOTPParams{UserID: userID, Purpose: purpose}); err == nil {
		if time.Since(last.CreatedAt.Time) < OTPCooldown {
			return "", ErrOTPCooldown
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	code, err := GenerateOTPCode()
	if err != nil {
		return "", err
	}
	key := userID.String() + ":" + purpose
	if _, err := s.q.InsertOTP(ctx, store.InsertOTPParams{
		UserID: userID, Purpose: purpose, CodeHash: HashOTP(key, code),
		ExpiresAt: ts(time.Now().Add(OTPTTL)), MaxAttempts: OTPMaxAttempts,
	}); err != nil {
		return "", err
	}
	if s.env == "dev" {
		s.log.Debug("otp issued (dev only)", "purpose", purpose, "code", code)
	}
	return code, nil
}

// VerifyOTP counts the attempt first, then checks — E11: the 3rd wrong attempt
// exhausts the challenge permanently.
func (s *Service) VerifyOTP(ctx context.Context, userID uuid.UUID, purpose, code string) error {
	ch, err := s.q.GetActiveOTP(ctx, store.GetActiveOTPParams{UserID: userID, Purpose: purpose})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOTPExpired
		}
		return err
	}
	inc, err := s.q.IncrementOTPAttempts(ctx, ch.ID)
	if err != nil {
		return err
	}
	key := userID.String() + ":" + purpose
	if err := CheckOTP(OTPChallenge{
		ID: key, CodeHash: ch.CodeHash,
		MaxAttempts: int(ch.MaxAttempts), ExpiresAt: ch.ExpiresAt.Time,
	}, code, int(inc.Attempts), time.Now()); err != nil {
		return err
	}
	return s.q.ConsumeOTP(ctx, ch.ID)
}

func (s *Service) issueTokens(ctx context.Context, userID uuid.UUID, deviceID string) (TokenPair, error) {
	token, hash, err := NewRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	if _, err := s.q.InsertRefreshToken(ctx, store.InsertRefreshTokenParams{
		UserID: userID, FamilyID: uuid.New(), TokenHash: hash,
		DeviceID: deviceID, ExpiresAt: ts(time.Now().Add(RefreshTokenTTL)),
	}); err != nil {
		return TokenPair{}, err
	}
	access, err := s.jwt.Issue(userID, deviceID, time.Now())
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: token, ExpiresIn: int64(AccessTokenTTL.Seconds())}, nil
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

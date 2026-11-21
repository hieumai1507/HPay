-- name: InsertUser :one
INSERT INTO users (phone, password_hash, full_name)
VALUES ($1, $2, $3)
RETURNING id, phone, full_name, kyc_status, status;

-- name: GetUserByPhone :one
SELECT id, phone, password_hash, full_name, kyc_status, status, login_failed_attempts, login_locked_until
FROM users WHERE phone = $1;

-- name: InsertRefreshToken :one
INSERT INTO refresh_tokens (user_id, family_id, token_hash, device_id, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: GetRefreshTokenByHash :one
SELECT id, user_id, family_id, device_id, expires_at, used_at, revoked_at
FROM refresh_tokens WHERE token_hash = $1;

-- name: MarkRefreshTokenUsed :one
-- WHERE used_at IS NULL makes concurrent refresh race-safe: only one caller wins.
UPDATE refresh_tokens SET used_at = now()
WHERE id = $1 AND used_at IS NULL
RETURNING id;

-- name: RevokeRefreshFamily :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: InsertOTP :one
INSERT INTO otp_challenges (user_id, purpose, code_hash, expires_at, max_attempts)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, expires_at;

-- name: GetActiveOTP :one
SELECT id, code_hash, attempts, max_attempts, expires_at, created_at
FROM otp_challenges
WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL
ORDER BY created_at DESC LIMIT 1;

-- name: IncrementOTPAttempts :one
UPDATE otp_challenges SET attempts = attempts + 1
WHERE id = $1
RETURNING attempts, max_attempts;

-- name: ConsumeOTP :exec
UPDATE otp_challenges SET consumed_at = now() WHERE id = $1;

-- name: InsertAudit :exec
INSERT INTO audit_logs (actor_id, actor_type, action, resource, reason, ip)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetUserStatusByID :one
SELECT kyc_status, status FROM users WHERE id = $1;

-- name: GetUserProfileByID :one
-- Own profile for GET /v1/users/me (m3-api-support) — no password_hash.
SELECT id, phone, full_name, kyc_status, status FROM users WHERE id = $1;

-- name: GetAdminByEmail :one
SELECT id, email, password_hash, full_name, role, status, login_failed_attempts, login_locked_until
FROM admin_users WHERE email = $1;

-- name: GetUserPINState :one
-- Credentials for PIN verification. Never selected by list or profile queries.
SELECT pin_hash, pin_failed_attempts, pin_locked_until, password_hash
FROM users WHERE id = $1;

-- name: SetUserPIN :exec
-- Setting or changing the PIN clears the attempt counter and any lock.
UPDATE users
SET pin_hash = $2, pin_failed_attempts = 0, pin_locked_until = NULL, updated_at = now()
WHERE id = $1;

-- name: RecordPINFailure :one
-- Atomic increment. If an earlier lock has expired the count restarts at 1. The
-- fifth consecutive failure locks PIN use for 15 minutes.
UPDATE users SET
  pin_failed_attempts = CASE WHEN pin_locked_until IS NOT NULL AND pin_locked_until <= now()
                             THEN 1 ELSE pin_failed_attempts + 1 END,
  pin_locked_until = CASE
    WHEN (CASE WHEN pin_locked_until IS NOT NULL AND pin_locked_until <= now()
               THEN 1 ELSE pin_failed_attempts + 1 END) >= 5
      THEN now() + interval '15 minutes'
    WHEN pin_locked_until IS NOT NULL AND pin_locked_until <= now() THEN NULL
    ELSE pin_locked_until END
WHERE id = $1
RETURNING pin_failed_attempts, pin_locked_until;

-- name: ResetPINFailures :exec
UPDATE users SET pin_failed_attempts = 0, pin_locked_until = NULL
WHERE id = $1 AND (pin_failed_attempts <> 0 OR pin_locked_until IS NOT NULL);

-- name: GetAdminByID :one
-- Loaded on every admin request (ADR-0015): the role and status come from the
-- database, never from the token, so a lock or demotion takes effect immediately.
SELECT id, email, full_name, role, status FROM admin_users WHERE id = $1;

-- name: RecordUserLoginFailure :one
-- Atomic increment. An expired lock restarts the count at 1; the 5th consecutive failure
-- locks login for 15 minutes.
UPDATE users SET
  login_failed_attempts = CASE WHEN login_locked_until IS NOT NULL AND login_locked_until <= now()
                               THEN 1 ELSE login_failed_attempts + 1 END,
  login_locked_until = CASE
    WHEN (CASE WHEN login_locked_until IS NOT NULL AND login_locked_until <= now()
               THEN 1 ELSE login_failed_attempts + 1 END) >= 5
      THEN now() + interval '15 minutes'
    WHEN login_locked_until IS NOT NULL AND login_locked_until <= now() THEN NULL
    ELSE login_locked_until END
WHERE id = $1
RETURNING login_failed_attempts, login_locked_until;

-- name: ResetUserLoginFailures :exec
UPDATE users SET login_failed_attempts = 0, login_locked_until = NULL
WHERE id = $1 AND (login_failed_attempts <> 0 OR login_locked_until IS NOT NULL);

-- name: RecordAdminLoginFailure :one
UPDATE admin_users SET
  login_failed_attempts = CASE WHEN login_locked_until IS NOT NULL AND login_locked_until <= now()
                               THEN 1 ELSE login_failed_attempts + 1 END,
  login_locked_until = CASE
    WHEN (CASE WHEN login_locked_until IS NOT NULL AND login_locked_until <= now()
               THEN 1 ELSE login_failed_attempts + 1 END) >= 5
      THEN now() + interval '15 minutes'
    WHEN login_locked_until IS NOT NULL AND login_locked_until <= now() THEN NULL
    ELSE login_locked_until END
WHERE id = $1
RETURNING login_failed_attempts, login_locked_until;

-- name: ResetAdminLoginFailures :exec
UPDATE admin_users SET login_failed_attempts = 0, login_locked_until = NULL
WHERE id = $1 AND (login_failed_attempts <> 0 OR login_locked_until IS NOT NULL);

-- name: CountRecentOTPs :one
-- How many codes of this purpose were issued to the user since a point in time (hourly cap).
SELECT count(*)::int AS n FROM otp_challenges
WHERE user_id = $1 AND purpose = $2 AND created_at >= $3;

-- name: ConsumeOTPIfActive :execrows
-- Single-use consume: only one of several parallel confirms can flip consumed_at.
UPDATE otp_challenges SET consumed_at = now()
WHERE id = $1 AND consumed_at IS NULL AND expires_at > now();

-- name: ResetUserPIN :execrows
-- Replace the PIN, clear the attempt counter and lock, start the cooling period.
UPDATE users
SET pin_hash = $2, pin_failed_attempts = 0, pin_locked_until = NULL, pin_reset_at = now(), updated_at = now()
WHERE id = $1;

-- name: ClearUserPIN :execrows
-- Operator fallback: remove the PIN (the user sets a new one). Starts the cooling period.
UPDATE users
SET pin_hash = NULL, pin_failed_attempts = 0, pin_locked_until = NULL, pin_reset_at = now(), updated_at = now()
WHERE id = $1 AND pin_hash IS NOT NULL;

-- name: GetPINResetAt :one
SELECT pin_reset_at FROM users WHERE id = $1;

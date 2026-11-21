-- name: InsertUser :one
INSERT INTO users (phone, password_hash, full_name)
VALUES ($1, $2, $3)
RETURNING id, phone, full_name, kyc_status, status;

-- name: GetUserByPhone :one
SELECT id, phone, password_hash, full_name, kyc_status, status
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
SELECT id, email, password_hash, full_name, role, status
FROM admin_users WHERE email = $1;

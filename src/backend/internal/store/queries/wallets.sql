-- name: GetWalletByID :one
SELECT id, owner_type, currency, purpose, label, balance, version
FROM wallets WHERE id = $1;

-- name: GetWalletsForUpdate :many
-- CRITICAL: ORDER BY id enforces lock ordering (E03). Callers pass ALL wallet
-- ids they intend to touch in one call — never lock in a loop.
-- FOR NO KEY UPDATE, not FOR UPDATE: inserting a transaction row takes a
-- KEY SHARE lock on its wallets through the foreign keys, and FOR UPDATE
-- conflicts with that, so two concurrent postings on one wallet (SYSTEM_CASH_IN
-- during top-ups) each held KEY SHARE and waited for the other: 40P01. A balance
-- change never touches a key column, so the weaker lock is correct and is
-- compatible with KEY SHARE. Two postings still exclude each other.
SELECT id, owner_type, currency, purpose, balance, version
FROM wallets WHERE id = ANY($1::uuid[]) ORDER BY id FOR NO KEY UPDATE;

-- name: CreateUserWallet :one
INSERT INTO wallets (owner_type, owner_id, currency, purpose, label)
VALUES ('user', $1, $2, $3, $4)
ON CONFLICT DO NOTHING
RETURNING id, owner_type, currency, purpose, label, balance, version;

-- name: GetUserMainWallet :one
SELECT id, owner_type, currency, purpose, label, balance, version
FROM wallets
WHERE owner_type = 'user' AND owner_id = $1 AND currency = $2 AND purpose = 'main';

-- name: ListUserWallets :many
SELECT id, currency, purpose, label, balance
FROM wallets
WHERE owner_type = 'user' AND owner_id = $1
ORDER BY purpose, created_at;

-- name: GetSystemWallet :one
SELECT id, owner_type, currency, purpose, balance, version
FROM wallets WHERE system_code = $1;

-- name: ApplyWalletDelta :one
-- The ONLY statement that changes a balance. Caller must hold FOR UPDATE lock.
UPDATE wallets SET balance = balance + $2, version = version + 1, updated_at = now()
WHERE id = $1
RETURNING balance;

-- name: GetUserStatusByWalletID :one
-- E12: money steps re-check the owner's account status inside their own tx.
SELECT u.status, u.kyc_status
FROM users u JOIN wallets w ON w.owner_id = u.id
WHERE w.id = $1 AND w.owner_type = 'user';

-- name: GetOwnerStatusByWalletID :one
-- Generic E12 re-check: works for user AND organization wallets (F6).
-- System wallets are always considered active.
SELECT COALESCE(
  CASE w.owner_type
    WHEN 'user' THEN u.status
    WHEN 'organization' THEN o.status
    ELSE 'active'::account_status
  END, 'active'::account_status)::account_status AS status
FROM wallets w
LEFT JOIN users u ON w.owner_type = 'user' AND u.id = w.owner_id
LEFT JOIN organizations o ON w.owner_type = 'organization' AND o.id = w.owner_id
WHERE w.id = $1;

-- name: GetWalletOwner :one
SELECT owner_type,
       COALESCE(owner_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS owner_id
FROM wallets WHERE id = $1;

-- name: CreateOrgWallet :one
INSERT INTO wallets (owner_type, owner_id, currency, purpose, label)
VALUES ('organization', $1, $2, 'main', NULL)
ON CONFLICT DO NOTHING
RETURNING id, owner_type, currency, purpose, balance, version;

-- name: GetOrgMainWallet :one
SELECT id, owner_type, currency, purpose, balance, version
FROM wallets
WHERE owner_type = 'organization' AND owner_id = $1 AND currency = $2 AND purpose = 'main';

-- name: ListUserVaults :many
-- Vaults belonging to one user. The owner predicate lives here, not in the
-- handler (docs 01 §3.2 layer 2).
SELECT id, label, balance, COALESCE(target_amount, 0)::bigint AS target_amount, round_up_target, created_at
FROM wallets
WHERE owner_type = 'user' AND owner_id = $1 AND currency = $2 AND purpose = 'vault'
ORDER BY created_at;

-- name: GetUserVault :one
-- Owner-scoped lookup: a vault_id belonging to somebody else simply is not found.
SELECT id, label, balance, COALESCE(target_amount, 0)::bigint AS target_amount, round_up_target
FROM wallets
WHERE id = $1 AND owner_type = 'user' AND owner_id = $2 AND purpose = 'vault';

-- name: CreateVaultWallet :one
INSERT INTO wallets (owner_type, owner_id, currency, purpose, label, target_amount)
VALUES ('user', $1, $2, 'vault', $3, $4)
RETURNING id, label, balance, COALESCE(target_amount, 0)::bigint AS target_amount, round_up_target, created_at;

-- name: UnsetRoundUpTargets :exec
-- Clears any existing round-up target(s) for the owner before setting a new
-- one, so the partial unique index never sees two true rows mid-transaction.
UPDATE wallets SET round_up_target = false
WHERE owner_type = 'user' AND owner_id = $1 AND purpose = 'vault' AND round_up_target;

-- name: SetRoundUpTarget :one
-- Owner-scoped: a vault_id belonging to somebody else simply is not found.
UPDATE wallets SET round_up_target = $3
WHERE id = $1 AND owner_type = 'user' AND owner_id = $2 AND purpose = 'vault'
RETURNING id, label, balance, COALESCE(target_amount, 0)::bigint AS target_amount, round_up_target;

-- name: GetRoundUpTargetVault :one
-- Resolved by owner, not by user-supplied id — the round_up workflow step
-- reads this from the sender wallet's owner, never from client input.
SELECT id, label, balance, COALESCE(target_amount, 0)::bigint AS target_amount
FROM wallets
WHERE owner_type = 'user' AND owner_id = $1 AND currency = $2 AND purpose = 'vault' AND round_up_target;

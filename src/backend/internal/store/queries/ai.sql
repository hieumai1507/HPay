-- name: GetActiveAISetting :one
SELECT 
    fs.function_name,
    fs.provider,
    fs.model_name,
    fs.key_id,
    ak.name AS key_name,
    ak.api_key,
    ak.is_active AS key_active
FROM ai_function_settings fs
LEFT JOIN ai_keys ak ON fs.key_id = ak.id
WHERE fs.function_name = $1;

-- name: ListAIKeys :many
SELECT id, name, provider, is_active, created_at, updated_at 
FROM ai_keys
ORDER BY created_at DESC;

-- name: GetAIKey :one
SELECT id, name, provider, api_key, is_active, created_at, updated_at
FROM ai_keys
WHERE id = $1;

-- name: InsertAIKey :one
INSERT INTO ai_keys (name, provider, api_key, is_active)
VALUES ($1, $2, $3, $4)
RETURNING id, name, provider, is_active, created_at, updated_at;

-- name: UpdateAIKey :one
UPDATE ai_keys
SET 
    name = $2,
    api_key = $3,
    is_active = $4,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, provider, is_active, created_at, updated_at;

-- name: ListAISettings :many
SELECT 
    fs.function_name,
    fs.provider,
    fs.model_name,
    fs.key_id,
    ak.name AS key_name,
    ak.is_active AS key_active
FROM ai_function_settings fs
LEFT JOIN ai_keys ak ON fs.key_id = ak.id;

-- name: UpdateAISetting :one
UPDATE ai_function_settings
SET 
    provider = $2,
    model_name = $3,
    key_id = CASE WHEN sqlc.arg('key_id')::uuid = '00000000-0000-0000-0000-000000000000'::uuid THEN NULL ELSE sqlc.arg('key_id')::uuid END,
    updated_at = NOW()
WHERE function_name = $1
RETURNING function_name, provider, model_name, key_id, updated_at;

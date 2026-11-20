-- name: CountAppUsers :one
SELECT COUNT(*) FROM app_users;

-- name: CountRealAppUsers :one
SELECT COUNT(*) FROM app_users WHERE is_default = FALSE;

-- name: GetAppUserByID :one
SELECT * FROM app_users WHERE id = @id LIMIT 1;

-- name: GetAppUserByUsername :one
SELECT * FROM app_users
WHERE lower(username) = lower(@username)
LIMIT 1;

-- name: GetDefaultAppUserForUpdate :one
SELECT * FROM app_users
WHERE is_default = TRUE
FOR UPDATE;

-- name: CreateAppUser :one
INSERT INTO app_users (
    username,
    password_hash,
    is_default,
    created_at,
    updated_at
) VALUES (
    @username,
    @password_hash,
    @is_default,
    @now,
    @now
)
RETURNING *;

-- name: DisableAppUser :exec
UPDATE app_users SET
    disabled_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE id = @id;

-- name: TouchAppUserLogin :exec
UPDATE app_users SET
    last_login_at = @now::timestamptz,
    updated_at = @now::timestamptz
WHERE id = @id;

-- name: CreateAppSession :exec
INSERT INTO app_sessions (
    token_hash,
    user_id,
    csrf_token,
    created_at,
    last_seen_at,
    expires_at
) VALUES (
    @token_hash,
    @user_id,
    @csrf_token,
    @created_at,
    @last_seen_at,
    @expires_at
);

-- name: GetAppSession :one
SELECT
    s.token_hash,
    s.user_id,
    s.csrf_token,
    s.created_at,
    s.last_seen_at,
    s.expires_at,
    u.username,
    u.is_default,
    u.disabled_at
FROM app_sessions s
JOIN app_users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash
LIMIT 1;

-- name: TouchAppSession :exec
UPDATE app_sessions SET last_seen_at = @now
WHERE token_hash = @token_hash;

-- name: DeleteAppSession :exec
DELETE FROM app_sessions WHERE token_hash = @token_hash;

-- name: DeleteAppSessionsForUser :exec
DELETE FROM app_sessions WHERE user_id = @user_id;

-- name: DeleteExpiredAppSessions :exec
DELETE FROM app_sessions
WHERE expires_at < @now
   OR last_seen_at < @idle_cutoff;

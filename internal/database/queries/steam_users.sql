-- name: GetSteamUser :one
SELECT * FROM steam_users WHERE steam_id = $1 LIMIT 1;

-- name: GetSteamUserByName :one
SELECT * FROM steam_users
WHERE lower(name) = lower(sqlc.arg(name))
ORDER BY updated_at DESC
LIMIT 1;

-- name: CreateSteamUser :one
INSERT INTO steam_users (
    steam_id,
    name,
    updated_at,
    created_at
) VALUES (
    $1,
    $2,
    NOW(),
    NOW()
) RETURNING *;

-- name: UpdateSteamUser :one
UPDATE steam_users SET
    name = $2,
    updated_at = NOW()
WHERE steam_id = $1 RETURNING *;

-- name: UpsertSteamUser :one
INSERT INTO steam_users (
    steam_id,
    name,
    updated_at,
    created_at
) VALUES (
    $1,
    $2,
    NOW(),
    NOW()
) ON CONFLICT (steam_id) DO UPDATE SET
    name = $2,
    updated_at = NOW()
RETURNING *;

-- name: DeleteSteamUser :exec
DELETE FROM steam_users WHERE steam_id = $1;

-- name: UpdateSteamUserWishlistPrivate :exec
UPDATE steam_users SET
    wishlist_private = @wishlist_private,
    updated_at = NOW()
WHERE steam_id = @steam_id;

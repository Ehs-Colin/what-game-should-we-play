-- name: GetSteamUser :one
SELECT * FROM steam_users WHERE steam_id = $1 LIMIT 1;

-- name: GetSteamUserByName :one
SELECT * FROM steam_users
WHERE lower(name) = lower(sqlc.arg(name))
ORDER BY updated_at DESC
LIMIT 1;

-- name: SearchSteamUsers :many
SELECT
    su.steam_id,
    su.name,
    su.wishlist_private,
    su.created_at,
    su.updated_at,
    (
        SELECT COUNT(DISTINCT aa.app_id)
        FROM steam_user_apps sua
        JOIN app_alias aa ON aa.alias_id = sua.app_id
        WHERE sua.steam_id = su.steam_id
          AND sua.removed = FALSE
    ) AS owned_app_count,
    (
        SELECT COUNT(DISTINCT aa.app_id)
        FROM steam_user_wishlist suw
        JOIN app_alias aa ON aa.alias_id = suw.app_id
        WHERE suw.steam_id = su.steam_id
          AND suw.removed = FALSE
    ) AS wishlist_app_count,
    latest.status AS latest_import_status,
    latest.finished_at AS latest_import_finished_at,
    EXISTS (
        SELECT 1
        FROM import_jobs active
        WHERE active.steam_id = su.steam_id
          AND active.status IN ('queued', 'running')
    ) AS has_active_job,
    EXISTS (
        SELECT 1
        FROM import_jobs succeeded
        WHERE succeeded.steam_id = su.steam_id
          AND succeeded.status = 'succeeded'
    ) AS has_succeeded_import
FROM steam_users su
LEFT JOIN import_jobs latest ON latest.id = (
    SELECT id
    FROM import_jobs
    WHERE steam_id = su.steam_id
    ORDER BY created_at DESC, id DESC
    LIMIT 1
)
WHERE strpos(lower(su.name), lower(sqlc.arg(name))) > 0
ORDER BY su.name, su.steam_id;

-- name: ListImportedSteamUsers :many
SELECT
    su.steam_id,
    su.name,
    EXISTS (
        SELECT 1
        FROM import_jobs ij
        WHERE ij.steam_id = su.steam_id
          AND ij.status = 'succeeded'
    ) AS has_succeeded_import
FROM steam_users su
WHERE su.steam_id = ANY(@steam_ids::bigint[])
ORDER BY su.steam_id;

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

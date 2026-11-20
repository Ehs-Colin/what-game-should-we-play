-- name: AddSteamUserApp :exec
-- app_id is the ID the user owns. Re-adding a removed app restores it.
INSERT INTO steam_user_apps (
    steam_id,
    app_id,
    removed,
    updated_at,
    created_at
) VALUES (
    $1,
    $2,
    FALSE,
    NOW(),
    NOW()
)
ON CONFLICT (steam_id, app_id) DO UPDATE SET
    removed = FALSE,
    updated_at = NOW();

-- name: MarkSteamUserAppsRemoved :exec
UPDATE steam_user_apps SET
    removed = TRUE,
    updated_at = NOW()
WHERE steam_id = @steam_id
    AND removed = FALSE
    AND NOT (app_id = ANY(@owned_ids::int[]));

-- name: RemoveSteamUserApp :exec
DELETE FROM steam_user_apps WHERE steam_id = $1 AND app_id = $2;

-- name: ClearSteamUserApps :exec
DELETE FROM steam_user_apps WHERE steam_id = $1;

-- name: ListAppsForSteamUser :many
SELECT DISTINCT a.* FROM apps a
JOIN app_alias aa ON aa.app_id = a.steam_appid
JOIN steam_user_apps sua ON sua.app_id = aa.alias_id
WHERE sua.steam_id = $1
ORDER BY a.name;

-- name: ListActiveSteamUserApps :many
SELECT DISTINCT a.* FROM apps a
JOIN app_alias aa ON aa.app_id = a.steam_appid
JOIN steam_user_apps sua ON sua.app_id = aa.alias_id
WHERE sua.steam_id = $1 AND sua.removed = FALSE
ORDER BY a.name;

-- name: CountActiveSteamUserApps :one
SELECT COUNT(DISTINCT aa.app_id)
FROM steam_user_apps sua
JOIN app_alias aa ON aa.alias_id = sua.app_id
WHERE sua.steam_id = $1
  AND sua.removed = FALSE;

-- name: ListActiveSteamUserAppsPage :many
SELECT DISTINCT a.* FROM apps a
JOIN app_alias aa ON aa.app_id = a.steam_appid
JOIN steam_user_apps sua ON sua.app_id = aa.alias_id
WHERE sua.steam_id = @steam_id
  AND sua.removed = FALSE
ORDER BY a.name NULLS LAST, a.steam_appid
LIMIT @page_size OFFSET @row_offset;

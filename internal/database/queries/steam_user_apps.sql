-- name: AddSteamUserApp :exec
INSERT INTO steam_user_apps (
    steam_id,
    app_id,
    updated_at,
    created_at
) VALUES (
    $1,
    $2,
    NOW(),
    NOW()
)
ON CONFLICT DO NOTHING;

-- name: RemoveSteamUserApp :exec
DELETE FROM steam_user_apps WHERE steam_id = $1 AND app_id = $2;

-- name: ClearSteamUserApps :exec
DELETE FROM steam_user_apps WHERE steam_id = $1;

-- name: ListAppsForSteamUser :many
SELECT a.* FROM apps a
JOIN steam_user_apps sua ON sua.app_id = a.steam_appid
WHERE sua.steam_id = $1
ORDER BY a.name;

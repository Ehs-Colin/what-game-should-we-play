-- name: AddSteamUserApp :exec
INSERT INTO steam_user_apps (steam_id, app_id) VALUES ($1, $2)
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

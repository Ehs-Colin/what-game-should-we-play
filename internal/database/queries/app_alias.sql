-- name: GetAppAlias :one
SELECT * FROM app_alias WHERE alias_id = $1 LIMIT 1;

-- name: ListAppAliases :many
SELECT * FROM app_alias ORDER BY app_id, alias_id;

-- name: UpsertAppAlias :one
INSERT INTO app_alias (
    alias_id,
    app_id,
    created_at,
    updated_at
) VALUES
    ($1,
    $2,
    NOW(),
    NOW())
ON CONFLICT (alias_id) DO UPDATE SET
    app_id = EXCLUDED.app_id,
    updated_at = NOW()
RETURNING *;

-- name: DeleteAppAlias :exec
DELETE FROM app_alias WHERE alias_id = $1;

-- name: ClearAppAliases :exec
DELETE FROM app_alias WHERE app_id = $1;

-- name: ListAliasesForApp :many
SELECT * FROM app_alias WHERE app_id = $1 ORDER BY alias_id;

-- name: GetAppByAlias :one
SELECT a.* FROM apps a
JOIN app_alias aa ON aa.app_id = a.steam_appid
WHERE aa.alias_id = $1
LIMIT 1;

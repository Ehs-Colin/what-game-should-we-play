-- name: GetSteamUser :one
SELECT * FROM steam_users WHERE steam_id = $1 LIMIT 1;

-- name: CreateSteamUser :one
INSERT INTO steam_users (steam_id, name) VALUES ($1, $2) RETURNING *;

-- name: UpdateSteamUser :one
UPDATE steam_users SET name = $2 WHERE steam_id = $1 RETURNING *;

-- name: DeleteSteamUser :exec
DELETE FROM steam_users WHERE steam_id = $1;

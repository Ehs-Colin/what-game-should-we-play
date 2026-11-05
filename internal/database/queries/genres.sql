-- name: GetGenre :one
SELECT * FROM genres WHERE id = $1 LIMIT 1;

-- name: ListGenres :many
SELECT * FROM genres ORDER BY description;

-- name: UpsertGenre :one
INSERT INTO genres (id, description) VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET description = EXCLUDED.description
RETURNING *;

-- name: DeleteGenre :exec
DELETE FROM genres WHERE id = $1;

-- name: AddAppGenre :exec
INSERT INTO app_genres (app_id, genre_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAppGenre :exec
DELETE FROM app_genres WHERE app_id = $1 AND genre_id = $2;

-- name: ClearAppGenres :exec
DELETE FROM app_genres WHERE app_id = $1;

-- name: ListGenresForApp :many
SELECT g.* FROM genres g
JOIN app_genres ag ON ag.genre_id = g.id
WHERE ag.app_id = $1
ORDER BY g.description;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = $1 LIMIT 1;

-- name: ListCategories :many
SELECT * FROM categories ORDER BY description;

-- name: UpsertCategory :one
INSERT INTO categories (id, description) VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET description = EXCLUDED.description
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;

-- name: AddAppCategory :exec
INSERT INTO app_categories (app_id, category_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveAppCategory :exec
DELETE FROM app_categories WHERE app_id = $1 AND category_id = $2;

-- name: ClearAppCategories :exec
DELETE FROM app_categories WHERE app_id = $1;

-- name: ListCategoriesForApp :many
SELECT c.* FROM categories c
JOIN app_categories ac ON ac.category_id = c.id
WHERE ac.app_id = $1
ORDER BY c.description;

-- name: GetApp :one
SELECT * FROM apps WHERE steam_appid = $1 LIMIT 1;

-- name: ListApps :many
SELECT * FROM apps ORDER BY name;

-- name: CreateApp :one
INSERT INTO apps (
    steam_appid,
    name,
    detailed_description,
    about_the_game,
    short_description,
    header_image,
    website,
    price_currency,
    price_initial,
    price_final,
    price_discount_percent,
    price_final_formatted,
    platform_windows,
    platform_mac,
    platform_linux,
    release_date,
    release_coming_soon
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13,
    $14,
    $15,
    $16,
    $17
) RETURNING *;

-- name: UpdateApp :one
UPDATE apps SET
    name = $2,
    detailed_description = $3,
    about_the_game = $4,
    short_description = $5,
    header_image = $6,
    website = $7,
    price_currency = $8,
    price_initial = $9,
    price_final = $10,
    price_discount_percent = $11,
    price_final_formatted = $12,
    platform_windows = $13,
    platform_mac = $14,
    platform_linux = $15,
    release_date = $16,
    release_coming_soon = $17
WHERE steam_appid = $1
RETURNING *;

-- name: UpsertApp :one
INSERT INTO apps (
    steam_appid,
    name,
    detailed_description,
    about_the_game,
    short_description,
    header_image,
    website,
    price_currency,
    price_initial,
    price_final,
    price_discount_percent,
    price_final_formatted,
    platform_windows,
    platform_mac,
    platform_linux,
    release_date,
    release_coming_soon
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11,
    $12,
    $13,
    $14,
    $15,
    $16,
    $17
)
ON CONFLICT (steam_appid) DO UPDATE SET
    name = EXCLUDED.name,
    detailed_description = EXCLUDED.detailed_description,
    about_the_game = EXCLUDED.about_the_game,
    short_description = EXCLUDED.short_description,
    header_image = EXCLUDED.header_image,
    website = EXCLUDED.website,
    price_currency = EXCLUDED.price_currency,
    price_initial = EXCLUDED.price_initial,
    price_final = EXCLUDED.price_final,
    price_discount_percent = EXCLUDED.price_discount_percent,
    price_final_formatted = EXCLUDED.price_final_formatted,
    platform_windows = EXCLUDED.platform_windows,
    platform_mac = EXCLUDED.platform_mac,
    platform_linux = EXCLUDED.platform_linux,
    release_date = EXCLUDED.release_date,
    release_coming_soon = EXCLUDED.release_coming_soon
RETURNING *;

-- name: DeleteApp :exec
DELETE FROM apps WHERE steam_appid = $1;

-- name: AddSteamUserWishlistApp :exec
INSERT INTO steam_user_wishlist (
    steam_id,
    app_id,
    date_added,
    removed,
    created_at,
    updated_at
) VALUES (
    $1,
    $2,
    $3,
    FALSE,
    NOW(),
    NOW()
)
ON CONFLICT (steam_id, app_id) DO UPDATE SET
    date_added = COALESCE(EXCLUDED.date_added, steam_user_wishlist.date_added),
    removed = FALSE,
    updated_at = NOW();

-- name: MarkSteamUserWishlistAppsRemoved :exec
UPDATE steam_user_wishlist SET
    removed = TRUE,
    updated_at = NOW()
WHERE steam_id = @steam_id
    AND removed = FALSE
    AND NOT (app_id = ANY(@wishlist_ids::int[]));

-- name: RemoveSteamUserWishlistApp :exec
DELETE FROM steam_user_wishlist WHERE steam_id = $1 AND app_id = $2;

-- name: ClearSteamUserWishlist :exec
DELETE FROM steam_user_wishlist WHERE steam_id = $1;

-- name: ListActiveSteamUserWishlist :many
SELECT DISTINCT a.* FROM apps a
JOIN app_alias aa ON aa.app_id = a.steam_appid
JOIN steam_user_wishlist suw ON suw.app_id = aa.alias_id
WHERE suw.steam_id = $1 AND suw.removed = FALSE
ORDER BY a.name;

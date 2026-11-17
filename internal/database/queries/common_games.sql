-- name: ListCommonGames :many
WITH active_owned AS (
    SELECT DISTINCT sua.steam_id, aa.app_id
    FROM steam_user_apps sua
    JOIN app_alias aa ON aa.alias_id = sua.app_id
    WHERE sua.steam_id = ANY(@steam_ids::bigint[])
      AND sua.removed = FALSE
),
active_wishlist AS (
    SELECT DISTINCT suw.steam_id, aa.app_id
    FROM steam_user_wishlist suw
    JOIN app_alias aa ON aa.alias_id = suw.app_id
    WHERE suw.steam_id = ANY(@steam_ids::bigint[])
      AND suw.removed = FALSE
),
per_user AS (
    SELECT
        steam_id,
        app_id,
        BOOL_OR(owned) AS owned,
        BOOL_OR(wishlisted) AS wishlisted
    FROM (
        SELECT steam_id, app_id, TRUE AS owned, FALSE AS wishlisted
        FROM active_owned
        UNION ALL
        SELECT steam_id, app_id, FALSE AS owned, TRUE AS wishlisted
        FROM active_wishlist
    ) lists
    GROUP BY steam_id, app_id
)
SELECT
    a.steam_appid,
    a.name,
    a.header_image,
    a.release_date,
    a.release_date_parsed,
    COUNT(*) FILTER (WHERE pu.owned)::bigint AS owned_count,
    COUNT(*) FILTER (WHERE pu.wishlisted)::bigint AS wishlist_count,
    COUNT(*)::bigint AS total_common_users
FROM per_user pu
JOIN apps a ON a.steam_appid = pu.app_id
WHERE EXISTS (
    SELECT 1
    FROM app_categories ac
    WHERE ac.app_id = a.steam_appid
      AND ac.category_id = 1
)
  AND a.release_coming_soon IS NOT TRUE
  AND a.release_date_parsed IS NOT NULL
  AND a.release_date_parsed <= CURRENT_DATE
GROUP BY
    a.steam_appid,
    a.name,
    a.header_image,
    a.release_date,
    a.release_date_parsed
ORDER BY
    total_common_users DESC,
    owned_count DESC,
    wishlist_count DESC,
    a.release_date_parsed DESC,
    a.steam_appid ASC
LIMIT @row_limit;

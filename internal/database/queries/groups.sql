-- name: CreateGroup :one
INSERT INTO steam_user_groups (
    name,
    created_at,
    updated_at
) VALUES (
    $1,
    NOW(),
    NOW()
) RETURNING *;

-- name: GetGroup :one
SELECT * FROM steam_user_groups WHERE id = $1;

-- name: GetGroupByName :one
SELECT * FROM steam_user_groups
WHERE lower(name) = lower(sqlc.arg(name));

-- name: RenameGroup :one
UPDATE steam_user_groups SET
    name = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteGroup :execrows
DELETE FROM steam_user_groups WHERE id = $1;

-- name: TouchGroup :exec
UPDATE steam_user_groups SET updated_at = NOW() WHERE id = $1;

-- name: AddGroupMember :exec
INSERT INTO steam_user_group_members (
    group_id,
    steam_id,
    created_at
) VALUES (
    $1,
    $2,
    NOW()
) ON CONFLICT (group_id, steam_id) DO NOTHING;

-- name: RemoveGroupMember :execrows
DELETE FROM steam_user_group_members
WHERE group_id = $1
  AND steam_id = $2;

-- name: SearchGroups :many
SELECT
    g.id,
    g.name,
    g.created_at,
    g.updated_at,
    (
        SELECT COUNT(*)
        FROM steam_user_group_members gm
        WHERE gm.group_id = g.id
    ) AS member_count,
    (
        SELECT COUNT(*)
        FROM steam_user_group_members gm
        WHERE gm.group_id = g.id
          AND NOT EXISTS (
              SELECT 1
              FROM import_jobs ij
              WHERE ij.steam_id = gm.steam_id
                AND ij.status = 'succeeded'
          )
    ) AS unavailable_member_count
FROM steam_user_groups g
WHERE strpos(lower(g.name), lower(sqlc.arg(name))) > 0
ORDER BY g.name, g.id;

-- name: ListGroupsByIDs :many
SELECT
    g.id,
    g.name,
    g.created_at,
    g.updated_at,
    (
        SELECT COUNT(*)
        FROM steam_user_group_members gm
        WHERE gm.group_id = g.id
    ) AS member_count,
    (
        SELECT COUNT(*)
        FROM steam_user_group_members gm
        WHERE gm.group_id = g.id
          AND NOT EXISTS (
              SELECT 1
              FROM import_jobs ij
              WHERE ij.steam_id = gm.steam_id
                AND ij.status = 'succeeded'
          )
    ) AS unavailable_member_count
FROM steam_user_groups g
WHERE g.id = ANY(@group_ids::bigint[])
ORDER BY g.name, g.id;

-- name: GetGroupCounts :one
SELECT
    (
        SELECT COUNT(*)
        FROM steam_user_group_members gm
        WHERE gm.group_id = @group_id
    ) AS member_count,
    (
        SELECT COUNT(*)
        FROM steam_user_group_members gm
        WHERE gm.group_id = @group_id
          AND NOT EXISTS (
              SELECT 1
              FROM import_jobs ij
              WHERE ij.steam_id = gm.steam_id
                AND ij.status = 'succeeded'
          )
    ) AS unavailable_member_count;

-- name: ListGroupsForSteamUser :many
SELECT
    g.id,
    g.name
FROM steam_user_groups g
JOIN steam_user_group_members gm ON gm.group_id = g.id
WHERE gm.steam_id = $1
ORDER BY g.name, g.id;

-- name: ListGroupMemberLookups :many
SELECT
    gm.group_id,
    su.steam_id,
    su.name,
    EXISTS (
        SELECT 1
        FROM import_jobs ij
        WHERE ij.steam_id = su.steam_id
          AND ij.status = 'succeeded'
    ) AS has_succeeded_import
FROM steam_user_group_members gm
JOIN steam_users su ON su.steam_id = gm.steam_id
WHERE gm.group_id = ANY(@group_ids::bigint[])
ORDER BY gm.group_id, su.name, su.steam_id;

-- name: ListGroupMembers :many
-- The SELECT list must stay identical to SearchSteamUsers so the rows convert
-- to dbgen.SearchSteamUsersRow.
SELECT
    su.steam_id,
    su.name,
    su.wishlist_private,
    su.personaname,
    su.profileurl,
    su.avatar,
    su.avatar_medium,
    su.avatar_full,
    su.created_at,
    su.updated_at,
    (
        SELECT COUNT(DISTINCT aa.app_id)
        FROM steam_user_apps sua
        JOIN app_alias aa ON aa.alias_id = sua.app_id
        WHERE sua.steam_id = su.steam_id
          AND sua.removed = FALSE
    ) AS owned_app_count,
    (
        SELECT COUNT(DISTINCT aa.app_id)
        FROM steam_user_wishlist suw
        JOIN app_alias aa ON aa.alias_id = suw.app_id
        WHERE suw.steam_id = su.steam_id
          AND suw.removed = FALSE
    ) AS wishlist_app_count,
    latest.status AS latest_import_status,
    latest.finished_at AS latest_import_finished_at,
    EXISTS (
        SELECT 1
        FROM import_jobs active
        WHERE active.steam_id = su.steam_id
          AND active.status IN ('queued', 'running')
    ) AS has_active_job,
    EXISTS (
        SELECT 1
        FROM import_jobs succeeded
        WHERE succeeded.steam_id = su.steam_id
          AND succeeded.status = 'succeeded'
    ) AS has_succeeded_import
FROM steam_users su
JOIN steam_user_group_members gm ON gm.steam_id = su.steam_id
LEFT JOIN import_jobs latest ON latest.id = (
    SELECT id
    FROM import_jobs
    WHERE steam_id = su.steam_id
    ORDER BY created_at DESC, id DESC
    LIMIT 1
)
WHERE gm.group_id = @group_id
ORDER BY su.name, su.steam_id;

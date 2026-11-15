-- name: GetDashboardStats :one
SELECT
    (SELECT COUNT(*) FROM steam_users) AS total_steam_users,
    (SELECT COUNT(*) FROM apps) AS total_apps,
    (SELECT COUNT(*) FROM import_jobs WHERE status = 'running') AS running_jobs,
    (SELECT COUNT(*) FROM import_jobs WHERE status = 'queued') AS queued_jobs;

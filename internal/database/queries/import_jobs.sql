-- name: CreateImportJob :one
INSERT INTO import_jobs (
    steam_id,
    status,
    created_at
) VALUES (
    $1,
    'queued',
    $2
) RETURNING *;

-- name: GetImportJob :one
SELECT * FROM import_jobs WHERE id = $1 LIMIT 1;

-- name: GetActiveImportJobForUser :one
SELECT * FROM import_jobs
WHERE steam_id = $1 AND status IN ('queued', 'running')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetLatestSucceededImportJobForUser :one
SELECT * FROM import_jobs
WHERE steam_id = $1 AND status = 'succeeded'
ORDER BY finished_at DESC, id DESC
LIMIT 1;

-- name: MarkImportJobRunning :one
-- Counts are reset because a resumed job starts over from the first app.
UPDATE import_jobs SET
    status = 'running',
    total_apps = 0,
    processed_apps = 0,
    skipped_apps = 0,
    failed_apps = 0,
    wishlist_total_apps = 0,
    wishlist_processed_apps = 0,
    wishlist_skipped_apps = 0,
    wishlist_failed_apps = 0,
    error = NULL,
    started_at = @started_at,
    finished_at = NULL
WHERE id = @id
RETURNING *;

-- name: UpdateImportJobProgress :exec
UPDATE import_jobs SET
    total_apps = @total_apps,
    processed_apps = @processed_apps,
    skipped_apps = @skipped_apps,
    failed_apps = @failed_apps,
    wishlist_total_apps = @wishlist_total_apps,
    wishlist_processed_apps = @wishlist_processed_apps,
    wishlist_skipped_apps = @wishlist_skipped_apps,
    wishlist_failed_apps = @wishlist_failed_apps
WHERE id = @id;

-- name: FinishImportJob :one
UPDATE import_jobs SET
    status = @status,
    total_apps = @total_apps,
    processed_apps = @processed_apps,
    skipped_apps = @skipped_apps,
    failed_apps = @failed_apps,
    wishlist_total_apps = @wishlist_total_apps,
    wishlist_processed_apps = @wishlist_processed_apps,
    wishlist_skipped_apps = @wishlist_skipped_apps,
    wishlist_failed_apps = @wishlist_failed_apps,
    error = @error,
    finished_at = @finished_at
WHERE id = @id
RETURNING *;

-- name: ListUnfinishedImportJobs :many
SELECT * FROM import_jobs
WHERE status IN ('queued', 'running')
ORDER BY created_at, id;

-- name: GetSteamAPICache :one
SELECT response_body, expires_at > @now::timestamptz AS fresh
FROM steam_api_cache
WHERE cache_key = @cache_key
LIMIT 1;

-- name: UpsertSteamAPICache :one
INSERT INTO steam_api_cache (
    cache_key,
    response_body,
    fetched_at,
    expires_at,
    call_count,
    created_at,
    updated_at
) VALUES (
    @cache_key,
    @response_body,
    @fetched_at,
    @expires_at,
    @call_count,
    @created_at,
    @updated_at
)
ON CONFLICT (cache_key) DO UPDATE SET
    response_body = EXCLUDED.response_body,
    fetched_at = EXCLUDED.fetched_at,
    expires_at = EXCLUDED.expires_at,
    call_count = EXCLUDED.call_count,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: AddSteamAPICacheCall :exec
INSERT INTO steam_api_cache_calls (cache_key, fetched_at)
VALUES (@cache_key, @fetched_at);

-- name: CountSteamAPICacheCallsSince :one
SELECT COUNT(*) FROM steam_api_cache_calls
WHERE cache_key = @cache_key
    AND fetched_at >= @since::timestamptz;

-- name: DeleteOldSteamAPICacheCalls :exec
DELETE FROM steam_api_cache_calls
WHERE cache_key = @cache_key
    AND fetched_at < @before::timestamptz;

-- name: DeleteStaleSteamAPICache :exec
DELETE FROM steam_api_cache
WHERE expires_at < @before::timestamptz;

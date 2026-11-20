package steamapi

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
)

// PostgresCache stores Steam API responses in Postgres.
type PostgresCache struct {
	database *sql.DB
	queries  *dbgen.Queries
	logger   *logging.Logger
	window   time.Duration
}

func NewPostgresCache(database *sql.DB, logger *logging.Logger) *PostgresCache {
	return &PostgresCache{
		database: database,
		queries:  dbgen.New(database),
		logger:   logger,
		window:   CacheWindow,
	}
}

// Load returns the cached body and whether it is still fresh at now.
func (cache *PostgresCache) Load(ctx context.Context, key string, now time.Time) ([]byte, bool, bool, error) {
	body, fresh, found, err := cache.load(ctx, key, now)
	if err != nil {
		cache.log("steam_cache_load_failed", map[string]any{
			"key":   key,
			"error": err.Error(),
		})
	}
	return body, fresh, found, err
}

func (cache *PostgresCache) load(ctx context.Context, key string, now time.Time) ([]byte, bool, bool, error) {
	row, err := cache.queries.GetSteamAPICache(ctx, dbgen.GetSteamAPICacheParams{
		CacheKey: key,
		Now:      now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, false, nil
	}
	if err != nil {
		return nil, false, false, err
	}
	return row.ResponseBody, row.Fresh, true, nil
}

// Record stores body and records a successful fetch at now.
func (cache *PostgresCache) Record(ctx context.Context, key string, body []byte, now time.Time) error {
	err := cache.record(ctx, key, body, now)
	if err != nil {
		cache.log("steam_cache_record_failed", map[string]any{
			"key":   key,
			"error": err.Error(),
		})
	}
	return err
}

func (cache *PostgresCache) record(ctx context.Context, key string, body []byte, now time.Time) error {
	tx, err := cache.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	queries := cache.queries.WithTx(tx)
	cutoff := now.Add(-cache.window)
	if err := queries.DeleteOldSteamAPICacheCalls(ctx, dbgen.DeleteOldSteamAPICacheCallsParams{
		CacheKey: key,
		Before:   cutoff,
	}); err != nil {
		return err
	}
	count, err := queries.CountSteamAPICacheCallsSince(ctx, dbgen.CountSteamAPICacheCallsSinceParams{
		CacheKey: key,
		Since:    cutoff,
	})
	if err != nil {
		return err
	}
	calls := int(count) + 1
	if _, err := queries.UpsertSteamAPICache(ctx, dbgen.UpsertSteamAPICacheParams{
		CacheKey:     key,
		ResponseBody: body,
		FetchedAt:    now,
		ExpiresAt:    now.Add(CacheTTL(calls)),
		CallCount:    int32(calls),
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		return err
	}
	if err := queries.AddSteamAPICacheCall(ctx, dbgen.AddSteamAPICacheCallParams{
		CacheKey:  key,
		FetchedAt: now,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// Prune removes cache rows that expired more than the sliding window ago.
// Their call events are removed by the foreign key cascade.
func (cache *PostgresCache) Prune(ctx context.Context, now time.Time) error {
	err := cache.queries.DeleteStaleSteamAPICache(ctx, now.Add(-cache.window))
	if err != nil {
		cache.log("steam_cache_prune_failed", map[string]any{
			"error": err.Error(),
		})
	}
	return err
}

func (cache *PostgresCache) log(event string, fields map[string]any) {
	if cache.logger == nil {
		return
	}
	_ = cache.logger.Event(event, fields)
}

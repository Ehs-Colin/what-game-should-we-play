// Package importer fetches a Steam user's library in the background, within
// Steam's store rate limit, and saves it to the database.
package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"

	defaultRetryBackoff = 5 * time.Second
	maxRateLimitRetries = 3
	progressEvery       = 10
	queueSize           = 100
)

// SteamClient is the subset of *steamapi.Client the importer uses.
type SteamClient interface {
	GetOwnedGames(ctx context.Context, steamID int64) (*steamapi.OwnedGamesResponse, error)
	GetWishlistItems(ctx context.Context, steamID string) ([]steamapi.WishlistItem, error)
	GetAppDetail(ctx context.Context, appID int) (*steamapi.AppDetail, error)
}

var _ SteamClient = (*steamapi.Client)(nil)

// Importer runs import jobs on a single background worker.
type Importer struct {
	database        *sql.DB
	queries         *dbgen.Queries
	steam           SteamClient
	logger          *logging.Logger
	refreshInterval time.Duration
	retryBackoff    time.Duration
	now             func() time.Time
	jobs            chan int64
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	// requestMu makes RequestImport's check-then-create atomic, so two
	// simultaneous requests for one user don't both create a job.
	requestMu sync.Mutex
}

func New(database *sql.DB, steam SteamClient, logger *logging.Logger, refreshInterval time.Duration) *Importer {
	return &Importer{
		database:        database,
		queries:         dbgen.New(database),
		steam:           steam,
		logger:          logger,
		refreshInterval: refreshInterval,
		retryBackoff:    defaultRetryBackoff,
		now:             time.Now,
		jobs:            make(chan int64, queueSize),
	}
}

// Start launches the worker and re-enqueues jobs left queued or running by a
// previous run. Rerunning a job is safe because every write is an upsert.
func (importer *Importer) Start(ctx context.Context) error {
	workerCtx, cancel := context.WithCancel(ctx)
	importer.cancel = cancel

	importer.wg.Add(1)
	go importer.work(workerCtx)

	unfinished, err := importer.queries.ListUnfinishedImportJobs(ctx)
	if err != nil {
		return fmt.Errorf("list unfinished import jobs: %w", err)
	}
	for _, job := range unfinished {
		importer.Enqueue(job.ID)
	}
	return nil
}

// RequestImport returns the user's active job, or their last successful job
// if it finished within the refresh interval. Otherwise it creates and
// enqueues a new job, and created is true.
func (importer *Importer) RequestImport(ctx context.Context, steamID int64) (job dbgen.ImportJob, created bool, err error) {
	importer.requestMu.Lock()
	defer importer.requestMu.Unlock()

	job, err = importer.queries.GetActiveImportJobForUser(ctx, steamID)
	if err == nil {
		return job, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return dbgen.ImportJob{}, false, fmt.Errorf("get active import job: %w", err)
	}

	job, err = importer.queries.GetLatestSucceededImportJobForUser(ctx, steamID)
	if err == nil && job.FinishedAt.Valid && importer.now().Sub(job.FinishedAt.Time) < importer.refreshInterval {
		return job, false, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return dbgen.ImportJob{}, false, fmt.Errorf("get latest succeeded import job: %w", err)
	}

	job, err = importer.queries.CreateImportJob(ctx, dbgen.CreateImportJobParams{
		SteamID:   steamID,
		CreatedAt: importer.now(),
	})
	if err != nil {
		return dbgen.ImportJob{}, false, fmt.Errorf("create import job: %w", err)
	}
	importer.Enqueue(job.ID)
	return job, true, nil
}

// Enqueue never blocks. If the queue is full the job stays queued in the
// database and is picked up on the next Start.
func (importer *Importer) Enqueue(jobID int64) {
	select {
	case importer.jobs <- jobID:
	default:
		importer.log("import_queue_full", map[string]any{"jobId": jobID})
	}
}

// Shutdown stops the worker and waits for it to exit, or for ctx to end. A
// job interrupted mid-run stays running and is resumed on the next Start.
func (importer *Importer) Shutdown(ctx context.Context) error {
	if importer.cancel != nil {
		importer.cancel()
	}
	done := make(chan struct{})
	go func() {
		importer.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (importer *Importer) work(ctx context.Context) {
	defer importer.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case jobID := <-importer.jobs:
			importer.runJob(ctx, jobID)
		}
	}
}

type jobCounts struct {
	total, processed, skipped, failed                                 int32
	wishlistTotal, wishlistProcessed, wishlistSkipped, wishlistFailed int32
}

func (importer *Importer) runJob(ctx context.Context, jobID int64) {
	job, err := importer.queries.GetImportJob(ctx, jobID)
	if err != nil {
		importer.log("import_job_failed", map[string]any{"jobId": jobID, "error": err.Error()})
		return
	}
	// Guards against the same job being enqueued twice.
	if job.Status != StatusQueued && job.Status != StatusRunning {
		return
	}
	steamID := job.SteamID
	fields := map[string]any{"jobId": jobID, "steamId": steamID}

	if _, err := importer.queries.MarkImportJobRunning(ctx, dbgen.MarkImportJobRunningParams{
		StartedAt: sql.NullTime{Time: importer.now(), Valid: true},
		ID:        jobID,
	}); err != nil {
		importer.log("import_job_failed", withError(fields, err))
		return
	}
	importer.log("import_job_started", fields)

	var counts jobCounts
	owned, err := importer.steam.GetOwnedGames(ctx, steamID)
	if err != nil {
		if ctx.Err() != nil {
			importer.log("import_job_interrupted", fields)
			return
		}
		importer.fail(ctx, jobID, counts, fmt.Errorf("get owned games: %w", err))
		return
	}

	ownedIDs := uniqueAppIDs(owned.Response.Games)
	counts.total = int32(len(ownedIDs))
	savedApps := make(map[int32]struct{})
	if err := importer.updateProgress(ctx, jobID, counts); err != nil {
		if ctx.Err() != nil {
			importer.log("import_job_interrupted", fields)
			return
		}
		importer.fail(ctx, jobID, counts, fmt.Errorf("update progress: %w", err))
		return
	}

	for i, ownedID := range ownedIDs {
		if ctx.Err() != nil {
			importer.log("import_job_interrupted", fields)
			return
		}
		switch importer.importOwnedApp(ctx, jobID, steamID, ownedID, savedApps) {
		case resultProcessed:
			counts.processed++
		case resultSkipped:
			counts.skipped++
		case resultFailed:
			counts.failed++
		case resultCancelled:
			importer.log("import_job_interrupted", fields)
			return
		}
		if (i+1)%progressEvery == 0 {
			if err := importer.updateProgress(ctx, jobID, counts); err != nil {
				if ctx.Err() != nil {
					importer.log("import_job_interrupted", fields)
					return
				}
				importer.fail(ctx, jobID, counts, fmt.Errorf("update progress: %w", err))
				return
			}
		}
	}

	// A private profile reports no games; don't mark the whole library removed.
	if len(ownedIDs) > 0 {
		if err := importer.queries.MarkSteamUserAppsRemoved(ctx, dbgen.MarkSteamUserAppsRemovedParams{
			SteamID:  steamID,
			OwnedIds: ownedIDs,
		}); err != nil {
			if ctx.Err() != nil {
				importer.log("import_job_interrupted", fields)
				return
			}
			importer.fail(ctx, jobID, counts, fmt.Errorf("mark removed apps: %w", err))
			return
		}
	}

	if err := importer.importWishlist(ctx, jobID, steamID, &counts, savedApps); err != nil {
		if ctx.Err() != nil {
			importer.log("import_job_interrupted", fields)
			return
		}
		importer.fail(ctx, jobID, counts, err)
		return
	}

	if err := importer.finish(ctx, jobID, StatusSucceeded, counts, sql.NullString{}); err != nil {
		importer.log("import_job_failed", withError(fields, err))
		return
	}
	importer.log("import_job_finished", map[string]any{
		"jobId":                 jobID,
		"steamId":               steamID,
		"totalApps":             counts.total,
		"processedApps":         counts.processed,
		"skippedApps":           counts.skipped,
		"failedApps":            counts.failed,
		"wishlistTotalApps":     counts.wishlistTotal,
		"wishlistProcessedApps": counts.wishlistProcessed,
		"wishlistSkippedApps":   counts.wishlistSkipped,
		"wishlistFailedApps":    counts.wishlistFailed,
	})
}

// importWishlist stores the user's wishlist after the owned library. Wishlist
// fetch failures never fail the job: a private wishlist is recorded on the user
// and other errors are logged. A successful empty wishlist marks nothing
// removed, mirroring the empty-library rule.
func (importer *Importer) importWishlist(ctx context.Context, jobID, steamID int64, counts *jobCounts, savedApps map[int32]struct{}) error {
	fields := map[string]any{"jobId": jobID, "steamId": steamID}

	items, err := importer.steam.GetWishlistItems(ctx, strconv.FormatInt(steamID, 10))
	if errors.Is(err, steamapi.ErrWishlistPrivate) {
		if err := importer.setWishlistPrivate(ctx, steamID, true); err != nil {
			return err
		}
		importer.log("wishlist_private", fields)
		return nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		importer.log("wishlist_fetch_failed", withError(fields, err))
		return nil
	}

	if err := importer.setWishlistPrivate(ctx, steamID, false); err != nil {
		return err
	}

	unique := uniqueWishlistItems(items)
	counts.wishlistTotal = int32(len(unique))
	if err := importer.updateProgress(ctx, jobID, *counts); err != nil {
		return fmt.Errorf("update progress: %w", err)
	}

	wishlistIDs := make([]int32, 0, len(unique))
	for i, item := range unique {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		wishlistIDs = append(wishlistIDs, int32(item.AppID))
		switch importer.importWishlistApp(ctx, jobID, steamID, item, savedApps) {
		case resultProcessed:
			counts.wishlistProcessed++
		case resultSkipped:
			counts.wishlistSkipped++
		case resultFailed:
			counts.wishlistFailed++
		case resultCancelled:
			return ctx.Err()
		}
		if (i+1)%progressEvery == 0 {
			if err := importer.updateProgress(ctx, jobID, *counts); err != nil {
				return fmt.Errorf("update progress: %w", err)
			}
		}
	}

	if len(unique) > 0 {
		if err := importer.queries.MarkSteamUserWishlistAppsRemoved(ctx, dbgen.MarkSteamUserWishlistAppsRemovedParams{
			SteamID:     steamID,
			WishlistIds: wishlistIDs,
		}); err != nil {
			return fmt.Errorf("mark removed wishlist apps: %w", err)
		}
	}
	return nil
}

func (importer *Importer) setWishlistPrivate(ctx context.Context, steamID int64, private bool) error {
	if err := importer.queries.UpdateSteamUserWishlistPrivate(ctx, dbgen.UpdateSteamUserWishlistPrivateParams{
		SteamID:         steamID,
		WishlistPrivate: private,
	}); err != nil {
		return fmt.Errorf("update wishlist privacy: %w", err)
	}
	return nil
}

type appResult int

const (
	resultProcessed appResult = iota
	resultSkipped
	resultFailed
	resultCancelled
)

func (importer *Importer) ensureApp(ctx context.Context, fields map[string]any, appID int32, savedApps map[int32]struct{}) appResult {
	// If two IDs map to the same app, the second lookup finds the row saved by
	// the first, so the app is only fetched once per job.
	existing, err := importer.queries.GetAppByAlias(ctx, appID)
	switch {
	case err == nil:
		if _, saved := savedApps[existing.SteamAppid]; saved {
			return resultProcessed
		}
	case !errors.Is(err, sql.ErrNoRows):
		return importer.appFailed(ctx, fields, fmt.Errorf("look up app: %w", err))
	}

	detail, err := importer.fetchAppDetail(ctx, fields, appID)
	if err != nil {
		if errors.Is(err, steamapi.ErrAppNotFound) {
			importer.log("import_app_skipped", withError(fields, err))
			return resultSkipped
		}
		return importer.appFailed(ctx, fields, err)
	}

	if err := importer.saveAppDetails(ctx, appID, detail); err != nil {
		return importer.appFailed(ctx, fields, fmt.Errorf("save app: %w", err))
	}
	savedApps[canonicalAppID(appID, detail)] = struct{}{}
	return resultProcessed
}

func (importer *Importer) importOwnedApp(ctx context.Context, jobID, steamID int64, ownedID int32, savedApps map[int32]struct{}) appResult {
	fields := map[string]any{"jobId": jobID, "steamId": steamID, "appId": ownedID}

	if result := importer.ensureApp(ctx, fields, ownedID, savedApps); result != resultProcessed {
		return result
	}
	if err := importer.queries.AddSteamUserApp(ctx, dbgen.AddSteamUserAppParams{
		SteamID: steamID,
		AppID:   ownedID,
	}); err != nil {
		return importer.appFailed(ctx, fields, fmt.Errorf("link owned app: %w", err))
	}
	return resultProcessed
}

func (importer *Importer) importWishlistApp(ctx context.Context, jobID, steamID int64, item steamapi.WishlistItem, savedApps map[int32]struct{}) appResult {
	appID := int32(item.AppID)
	fields := map[string]any{
		"jobId":   jobID,
		"steamId": steamID,
		"appId":   appID,
		"source":  "wishlist",
	}

	if result := importer.ensureApp(ctx, fields, appID, savedApps); result != resultProcessed {
		return result
	}
	if err := importer.queries.AddSteamUserWishlistApp(ctx, dbgen.AddSteamUserWishlistAppParams{
		SteamID:   steamID,
		AppID:     appID,
		DateAdded: wishlistDateAdded(item.DateAdded),
	}); err != nil {
		return importer.appFailed(ctx, fields, fmt.Errorf("link wishlist app: %w", err))
	}
	return resultProcessed
}

// fetchAppDetail retries rate-limited requests up to maxRateLimitRetries times.
func (importer *Importer) fetchAppDetail(ctx context.Context, fields map[string]any, ownedID int32) (*steamapi.AppDetail, error) {
	backoff := importer.retryBackoff
	for attempt := 0; ; attempt++ {
		detail, err := importer.steam.GetAppDetail(ctx, int(ownedID))
		if err == nil {
			if detail == nil || detail.Data == nil {
				return nil, fmt.Errorf("%w for appID %d", steamapi.ErrAppNotFound, ownedID)
			}
			return detail, nil
		}
		if !errors.Is(err, steamapi.ErrRateLimited) || attempt >= maxRateLimitRetries {
			return nil, err
		}

		delay := backoff
		var rateLimitErr *steamapi.RateLimitError
		if errors.As(err, &rateLimitErr) && rateLimitErr.RetryAfter > 0 {
			delay = rateLimitErr.RetryAfter
		}
		backoff *= 2
		rateLimitFields := map[string]any{
			"jobId":   fields["jobId"],
			"steamId": fields["steamId"],
			"appId":   ownedID,
			"attempt": attempt + 1,
			"delay":   delay.String(),
		}
		if source, ok := fields["source"]; ok {
			rateLimitFields["source"] = source
		}
		importer.log("import_app_rate_limited", rateLimitFields)
		if err := sleep(ctx, delay); err != nil {
			return nil, err
		}
	}
}

func (importer *Importer) saveAppDetails(ctx context.Context, ownedID int32, detail *steamapi.AppDetail) (err error) {
	canonicalID := canonicalAppID(ownedID, detail)

	tx, err := importer.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	queries := importer.queries.WithTx(tx)

	if _, err = queries.UpsertApp(ctx, appParams(canonicalID, detail, importer.now())); err != nil {
		return fmt.Errorf("upsert app %d: %w", canonicalID, err)
	}

	aliases := []int32{canonicalID}
	if ownedID != canonicalID {
		aliases = append(aliases, ownedID)
	}
	for _, aliasID := range aliases {
		if _, err = queries.UpsertAppAlias(ctx, dbgen.UpsertAppAliasParams{
			AliasID: aliasID,
			AppID:   canonicalID,
		}); err != nil {
			return fmt.Errorf("upsert alias %d -> %d: %w", aliasID, canonicalID, err)
		}
	}

	if err = replaceCategories(ctx, queries, canonicalID, detail.Data.Categories); err != nil {
		return err
	}
	if err = replaceGenres(ctx, queries, canonicalID, detail.Data.Genres); err != nil {
		return err
	}

	return tx.Commit()
}

func canonicalAppID(requestedID int32, detail *steamapi.AppDetail) int32 {
	if detail.Data.AppID != 0 {
		return int32(detail.Data.AppID)
	}
	return requestedID
}

func appParams(canonicalID int32, detail *steamapi.AppDetail, now time.Time) dbgen.UpsertAppParams {
	data := detail.Data

	// Free apps have no price_overview; store them as a zero price.
	priceCurrency := "USD"
	priceInitial, priceFinal, priceDiscountPercent := 0, 0, 0
	priceFinalFormatted := "$0.00"
	if price := data.PriceOverview; price != nil {
		priceCurrency = price.Currency
		priceInitial = price.Initial
		priceFinal = price.Final
		priceDiscountPercent = price.DiscountPercent
		priceFinalFormatted = price.FinalFormatted
	}

	return dbgen.UpsertAppParams{
		SteamAppid:           canonicalID,
		Name:                 nullString(data.Name),
		DetailedDescription:  nullString(data.DetailedDescription),
		AboutTheGame:         nullString(data.AboutTheGame),
		ShortDescription:     nullString(data.ShortDescription),
		HeaderImage:          nullString(data.HeaderImage),
		Website:              nullString(data.Website),
		PriceCurrency:        sql.NullString{String: priceCurrency, Valid: true},
		PriceInitial:         sql.NullInt32{Int32: int32(priceInitial), Valid: true},
		PriceFinal:           sql.NullInt32{Int32: int32(priceFinal), Valid: true},
		PriceDiscountPercent: sql.NullInt32{Int32: int32(priceDiscountPercent), Valid: true},
		PriceFinalFormatted:  sql.NullString{String: priceFinalFormatted, Valid: true},
		PlatformWindows:      sql.NullBool{Bool: data.Platforms.Windows, Valid: true},
		PlatformMac:          sql.NullBool{Bool: data.Platforms.Mac, Valid: true},
		PlatformLinux:        sql.NullBool{Bool: data.Platforms.Linux, Valid: true},
		ReleaseDate:          nullString(data.ReleaseDate.Date),
		ReleaseComingSoon:    sql.NullBool{Bool: data.ReleaseDate.ComingSoon, Valid: true},
		UpdatedAt:            now,
	}
}

func replaceCategories(ctx context.Context, queries *dbgen.Queries, appID int32, categories []steamapi.Category) error {
	if err := queries.ClearAppCategories(ctx, appID); err != nil {
		return fmt.Errorf("clear categories for app %d: %w", appID, err)
	}
	for _, category := range categories {
		if _, err := queries.UpsertCategory(ctx, dbgen.UpsertCategoryParams{
			ID:          int32(category.ID),
			Description: sql.NullString{String: category.Description, Valid: true},
		}); err != nil {
			return fmt.Errorf("upsert category %d: %w", category.ID, err)
		}
		if err := queries.AddAppCategory(ctx, dbgen.AddAppCategoryParams{
			AppID:      appID,
			CategoryID: int32(category.ID),
		}); err != nil {
			return fmt.Errorf("add category %d to app %d: %w", category.ID, appID, err)
		}
	}
	return nil
}

func replaceGenres(ctx context.Context, queries *dbgen.Queries, appID int32, genres []steamapi.Genre) error {
	if err := queries.ClearAppGenres(ctx, appID); err != nil {
		return fmt.Errorf("clear genres for app %d: %w", appID, err)
	}
	for _, genre := range genres {
		genreID, err := strconv.Atoi(genre.ID)
		if err != nil {
			return fmt.Errorf("parse genre ID %q: %w", genre.ID, err)
		}
		if _, err := queries.UpsertGenre(ctx, dbgen.UpsertGenreParams{
			ID:          int32(genreID),
			Description: sql.NullString{String: genre.Description, Valid: true},
		}); err != nil {
			return fmt.Errorf("upsert genre %d: %w", genreID, err)
		}
		if err := queries.AddAppGenre(ctx, dbgen.AddAppGenreParams{
			AppID:   appID,
			GenreID: int32(genreID),
		}); err != nil {
			return fmt.Errorf("add genre %d to app %d: %w", genreID, appID, err)
		}
	}
	return nil
}

// appFailed logs a per-app failure, unless the failure was caused by shutdown.
func (importer *Importer) appFailed(ctx context.Context, fields map[string]any, err error) appResult {
	if ctx.Err() != nil {
		return resultCancelled
	}
	importer.log("import_app_failed", withError(fields, err))
	return resultFailed
}

func (importer *Importer) updateProgress(ctx context.Context, jobID int64, counts jobCounts) error {
	return importer.queries.UpdateImportJobProgress(ctx, dbgen.UpdateImportJobProgressParams{
		TotalApps:             counts.total,
		ProcessedApps:         counts.processed,
		SkippedApps:           counts.skipped,
		FailedApps:            counts.failed,
		WishlistTotalApps:     counts.wishlistTotal,
		WishlistProcessedApps: counts.wishlistProcessed,
		WishlistSkippedApps:   counts.wishlistSkipped,
		WishlistFailedApps:    counts.wishlistFailed,
		ID:                    jobID,
	})
}

func (importer *Importer) fail(ctx context.Context, jobID int64, counts jobCounts, cause error) {
	fields := map[string]any{"jobId": jobID, "error": cause.Error()}
	if err := importer.finish(ctx, jobID, StatusFailed, counts, sql.NullString{String: cause.Error(), Valid: true}); err != nil {
		fields["finishError"] = err.Error()
	}
	importer.log("import_job_failed", fields)
}

func (importer *Importer) finish(ctx context.Context, jobID int64, status string, counts jobCounts, jobErr sql.NullString) error {
	_, err := importer.queries.FinishImportJob(ctx, dbgen.FinishImportJobParams{
		Status:                status,
		TotalApps:             counts.total,
		ProcessedApps:         counts.processed,
		SkippedApps:           counts.skipped,
		FailedApps:            counts.failed,
		WishlistTotalApps:     counts.wishlistTotal,
		WishlistProcessedApps: counts.wishlistProcessed,
		WishlistSkippedApps:   counts.wishlistSkipped,
		WishlistFailedApps:    counts.wishlistFailed,
		Error:                 jobErr,
		FinishedAt:            sql.NullTime{Time: importer.now(), Valid: true},
		ID:                    jobID,
	})
	return err
}

func (importer *Importer) log(event string, fields map[string]any) {
	if importer.logger == nil {
		return
	}
	_ = importer.logger.Event(event, fields)
}

func uniqueAppIDs(games []steamapi.Games) []int32 {
	seen := make(map[int32]struct{}, len(games))
	ids := make([]int32, 0, len(games))
	for _, game := range games {
		id := int32(game.AppID)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func uniqueWishlistItems(items []steamapi.WishlistItem) []steamapi.WishlistItem {
	seen := make(map[int32]int, len(items))
	unique := make([]steamapi.WishlistItem, 0, len(items))
	for _, item := range items {
		id := int32(item.AppID)
		if index, ok := seen[id]; ok {
			if item.DateAdded > unique[index].DateAdded {
				unique[index] = item
			}
			continue
		}
		seen[id] = len(unique)
		unique = append(unique, item)
	}
	return unique
}

func wishlistDateAdded(seconds int) sql.NullTime {
	if seconds <= 0 {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: time.Unix(int64(seconds), 0).UTC(), Valid: true}
}

func withError(fields map[string]any, err error) map[string]any {
	out := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		out[key] = value
	}
	out["error"] = err.Error()
	return out
}

func sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

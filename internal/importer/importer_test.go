package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbtest"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

const (
	testSteamID         = int64(76561198000000001)
	testRefreshInterval = time.Hour
)

// fakeApp describes what the fake Steam store returns for an owned ID.
type fakeApp struct {
	canonicalID       int // 0 means the owned ID itself
	name              string
	categories        []steamapi.Category
	genres            []steamapi.Genre
	releaseDate       string
	releaseComingSoon bool
}

type fakeSteam struct {
	mu              sync.Mutex
	owned           map[int64][]int
	ownedErr        error
	wishlist        map[int64][]steamapi.WishlistItem
	wishlistErr     error
	wishlistPrivate bool
	wishlistCalls   int
	apps            map[int]fakeApp
	rateLimits      map[int]int   // 429s to return before succeeding
	appErrs         map[int]error // returned on every call
	block           map[int]bool  // block until ctx is cancelled
	calls           map[int]int
	called          chan int // receives each requested app ID, if set
}

func newFakeSteam() *fakeSteam {
	return &fakeSteam{
		owned:      map[int64][]int{},
		wishlist:   map[int64][]steamapi.WishlistItem{},
		apps:       map[int]fakeApp{},
		rateLimits: map[int]int{},
		appErrs:    map[int]error{},
		block:      map[int]bool{},
		calls:      map[int]int{},
	}
}

func (f *fakeSteam) setOwned(steamID int64, ids ...int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.owned[steamID] = ids
}

func (f *fakeSteam) setWishlist(steamID int64, items ...steamapi.WishlistItem) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wishlist[steamID] = items
}

func (f *fakeSteam) setApp(ownedID int, app fakeApp) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.apps[ownedID] = app
}

func (f *fakeSteam) callCount(appID int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[appID]
}

func (f *fakeSteam) GetOwnedGames(ctx context.Context, steamID int64) (*steamapi.OwnedGamesResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ownedErr != nil {
		return nil, f.ownedErr
	}
	response := &steamapi.OwnedGamesResponse{}
	response.Response.Games = []steamapi.Games{}
	for _, id := range f.owned[steamID] {
		response.Response.Games = append(response.Response.Games, steamapi.Games{AppID: id})
	}
	return response, nil
}

func (f *fakeSteam) GetWishlistItems(ctx context.Context, steamID string) ([]steamapi.WishlistItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wishlistCalls++
	if f.wishlistPrivate {
		return nil, fmt.Errorf("GetWishlistItems: %w", steamapi.ErrWishlistPrivate)
	}
	if f.wishlistErr != nil {
		return nil, f.wishlistErr
	}
	id, err := strconv.ParseInt(steamID, 10, 64)
	if err != nil {
		return nil, err
	}
	return f.wishlist[id], nil
}

func (f *fakeSteam) GetAppDetail(ctx context.Context, appID int) (*steamapi.AppDetail, error) {
	f.mu.Lock()
	f.calls[appID]++
	called, block := f.called, f.block[appID]
	f.mu.Unlock()
	if called != nil {
		called <- appID
	}
	if block {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.appErrs[appID]; err != nil {
		return nil, err
	}
	if f.rateLimits[appID] > 0 {
		f.rateLimits[appID]--
		return nil, fmt.Errorf("GetAppDetail: %w", &steamapi.RateLimitError{
			StatusCode: 429,
			RetryAfter: time.Millisecond,
		})
	}
	app, ok := f.apps[appID]
	if !ok {
		return nil, fmt.Errorf("%w for appID %d", steamapi.ErrAppNotFound, appID)
	}
	return makeDetail(appID, app), nil
}

// makeDetail builds an AppDetail through JSON, because Data is an anonymous
// struct pointer.
func makeDetail(ownedID int, app fakeApp) *steamapi.AppDetail {
	canonicalID := app.canonicalID
	if canonicalID == 0 {
		canonicalID = ownedID
	}
	name := app.name
	if name == "" {
		name = fmt.Sprintf("App %d", canonicalID)
	}
	raw, err := json.Marshal(map[string]any{
		"success": true,
		"data": map[string]any{
			"name":        name,
			"steam_appid": canonicalID,
			"categories":  app.categories,
			"genres":      app.genres,
			"platforms":   map[string]bool{"windows": true},
			"release_date": map[string]any{
				"date":        app.releaseDate,
				"coming_soon": app.releaseComingSoon,
			},
		},
	})
	if err != nil {
		panic(err)
	}
	var detail steamapi.AppDetail
	if err := json.Unmarshal(raw, &detail); err != nil {
		panic(err)
	}
	return &detail
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type testEnv struct {
	db       *sql.DB
	queries  *dbgen.Queries
	steam    *fakeSteam
	clock    *testClock
	importer *Importer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db := dbtest.Open(t)
	steam := newFakeSteam()
	// Postgres stores microseconds; keep the clock on whole seconds.
	clock := &testClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}

	importer := New(db, steam, nil, testRefreshInterval)
	importer.retryBackoff = time.Millisecond
	importer.now = clock.Now

	env := &testEnv{db: db, queries: dbgen.New(db), steam: steam, clock: clock, importer: importer}
	env.createUser(t, testSteamID)
	return env
}

func (env *testEnv) createUser(t *testing.T, steamID int64) {
	t.Helper()
	if _, err := env.queries.UpsertSteamUser(context.Background(), dbgen.UpsertSteamUserParams{
		SteamID: steamID,
		Name:    fmt.Sprintf("user-%d", steamID),
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func (env *testEnv) createJob(t *testing.T, steamID int64) int64 {
	t.Helper()
	job, err := env.queries.CreateImportJob(context.Background(), dbgen.CreateImportJobParams{
		SteamID:   steamID,
		CreatedAt: env.clock.Now(),
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	return job.ID
}

// runImport creates a job and runs it synchronously on the test goroutine.
func (env *testEnv) runImport(t *testing.T, steamID int64) dbgen.ImportJob {
	t.Helper()
	id := env.createJob(t, steamID)
	env.importer.runJob(context.Background(), id)
	return env.getJob(t, id)
}

func (env *testEnv) getJob(t *testing.T, id int64) dbgen.ImportJob {
	t.Helper()
	job, err := env.queries.GetImportJob(context.Background(), id)
	if err != nil {
		t.Fatalf("get job %d: %v", id, err)
	}
	return job
}

func (env *testEnv) waitForStatus(t *testing.T, id int64, status string) dbgen.ImportJob {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job := env.getJob(t, id)
		if job.Status == status {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	job := env.getJob(t, id)
	t.Fatalf("job %d: status %q, want %q", id, job.Status, status)
	return job
}

// userApp returns the removed flag for a user's owned ID, and whether the row
// exists.
func (env *testEnv) userApp(t *testing.T, steamID int64, ownedID int) (removed bool, exists bool) {
	t.Helper()
	err := env.db.QueryRow(
		`SELECT removed FROM steam_user_apps WHERE steam_id = $1 AND app_id = $2`,
		steamID, ownedID,
	).Scan(&removed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, false
	}
	if err != nil {
		t.Fatalf("query steam_user_apps: %v", err)
	}
	return removed, true
}

// userWishlist returns a user's wishlist row and whether it exists.
func (env *testEnv) userWishlist(t *testing.T, steamID int64, appID int) (dbgen.SteamUserWishlist, bool) {
	t.Helper()
	var row dbgen.SteamUserWishlist
	err := env.db.QueryRow(
		`SELECT steam_id, app_id, date_added, removed, created_at, updated_at
		 FROM steam_user_wishlist WHERE steam_id = $1 AND app_id = $2`,
		steamID, appID,
	).Scan(&row.SteamID, &row.AppID, &row.DateAdded, &row.Removed, &row.CreatedAt, &row.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return dbgen.SteamUserWishlist{}, false
	}
	if err != nil {
		t.Fatalf("query steam_user_wishlist: %v", err)
	}
	return row, true
}

func (env *testEnv) wishlistPrivate(t *testing.T, steamID int64) bool {
	t.Helper()
	user, err := env.queries.GetSteamUser(context.Background(), steamID)
	if err != nil {
		t.Fatalf("get steam user: %v", err)
	}
	return user.WishlistPrivate
}

func (env *testEnv) categoryIDs(t *testing.T, appID int32) []int32 {
	t.Helper()
	categories, err := env.queries.ListCategoriesForApp(context.Background(), appID)
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	ids := make([]int32, 0, len(categories))
	for _, category := range categories {
		ids = append(ids, category.ID)
	}
	slices.Sort(ids)
	return ids
}

func (env *testEnv) genreIDs(t *testing.T, appID int32) []int32 {
	t.Helper()
	genres, err := env.queries.ListGenresForApp(context.Background(), appID)
	if err != nil {
		t.Fatalf("list genres: %v", err)
	}
	ids := make([]int32, 0, len(genres))
	for _, genre := range genres {
		ids = append(ids, genre.ID)
	}
	slices.Sort(ids)
	return ids
}

type wantCounts struct {
	total, processed, skipped, failed int32
}

type wantWishlistCounts struct {
	total, processed, skipped, failed int32
}

func assertWishlistCounts(t *testing.T, job dbgen.ImportJob, want wantWishlistCounts) {
	t.Helper()
	got := wantWishlistCounts{
		job.WishlistTotalApps,
		job.WishlistProcessedApps,
		job.WishlistSkippedApps,
		job.WishlistFailedApps,
	}
	if got != want {
		t.Fatalf("wishlist counts = %+v, want %+v (error %q)", got, want, job.Error.String)
	}
}

func assertJob(t *testing.T, job dbgen.ImportJob, status string, want wantCounts) {
	t.Helper()
	got := wantCounts{job.TotalApps, job.ProcessedApps, job.SkippedApps, job.FailedApps}
	if job.Status != status || got != want {
		t.Fatalf("job: status %q counts %+v, want %q %+v (error %q)", job.Status, got, status, want, job.Error.String)
	}
	if status == StatusSucceeded || status == StatusFailed {
		if !job.FinishedAt.Valid {
			t.Fatalf("job finished_at not set")
		}
	}
}

func TestBasicImport(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{
		name:        "Game Ten",
		categories:  []steamapi.Category{{ID: 2, Description: "Single-player"}, {ID: 1, Description: "Multi-player"}},
		genres:      []steamapi.Genre{{ID: "1", Description: "Action"}},
		releaseDate: "21 Nov, 2024",
	})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	if !job.StartedAt.Valid || job.Error.Valid {
		t.Fatalf("job started_at %v, error %v", job.StartedAt, job.Error)
	}

	ctx := context.Background()
	app, err := env.queries.GetAppByAlias(ctx, 10)
	if err != nil {
		t.Fatalf("GetAppByAlias: %v", err)
	}
	if app.SteamAppid != 10 || app.Name.String != "Game Ten" || !app.UpdatedAt.Equal(env.clock.Now()) {
		t.Fatalf("app = %+v", app)
	}
	if !app.ReleaseDateParsed.Valid || !app.ReleaseDateParsed.Time.Equal(time.Date(2024, time.November, 21, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("release_date_parsed = %+v, want 2024-11-21", app.ReleaseDateParsed)
	}
	alias, err := env.queries.GetAppAlias(ctx, 10)
	if err != nil || alias.AppID != 10 {
		t.Fatalf("self-alias = %+v, %v", alias, err)
	}
	if got := env.categoryIDs(t, 10); !slices.Equal(got, []int32{1, 2}) {
		t.Fatalf("categories = %v", got)
	}
	if got := env.genreIDs(t, 10); !slices.Equal(got, []int32{1}) {
		t.Fatalf("genres = %v", got)
	}
	active, err := env.queries.ListActiveSteamUserApps(ctx, testSteamID)
	if err != nil || len(active) != 1 || active[0].SteamAppid != 10 {
		t.Fatalf("active apps = %+v, %v", active, err)
	}
}

func TestComingSoonAppHasNoParsedReleaseDate(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{
		releaseDate:       "21 Nov, 2024",
		releaseComingSoon: true,
	})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})

	app, err := env.queries.GetAppByAlias(context.Background(), 10)
	if err != nil {
		t.Fatalf("GetAppByAlias: %v", err)
	}
	if app.ReleaseDate.String != "21 Nov, 2024" {
		t.Fatalf("release_date = %q", app.ReleaseDate.String)
	}
	if app.ReleaseDateParsed.Valid {
		t.Fatalf("coming-soon release_date_parsed = %+v, want NULL", app.ReleaseDateParsed)
	}
}

func TestAliasResolvesToCanonicalApp(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 100)
	env.steam.setApp(100, fakeApp{canonicalID: 80})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})

	ctx := context.Background()
	for _, aliasID := range []int32{80, 100} {
		alias, err := env.queries.GetAppAlias(ctx, aliasID)
		if err != nil || alias.AppID != 80 {
			t.Fatalf("alias %d = %+v, %v", aliasID, alias, err)
		}
	}
	if _, exists := env.userApp(t, testSteamID, 100); !exists {
		t.Fatalf("user should be linked through owned ID 100")
	}
	if _, exists := env.userApp(t, testSteamID, 80); exists {
		t.Fatalf("user should not be linked through canonical ID 80")
	}
	app, err := env.queries.GetAppByAlias(ctx, 100)
	if err != nil || app.SteamAppid != 80 {
		t.Fatalf("GetAppByAlias(100) = %+v, %v", app, err)
	}
}

func TestTwoOwnedIDsForOneAppFetchOnce(t *testing.T) {
	env := newTestEnv(t)
	// 100 must come first: its fetch creates the alias that lets 80 hit the
	// stored app.
	env.steam.setOwned(testSteamID, 100, 80)
	env.steam.setApp(100, fakeApp{canonicalID: 80})
	env.steam.setApp(80, fakeApp{})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 2, processed: 2})

	if got := env.steam.callCount(100) + env.steam.callCount(80); got != 1 {
		t.Fatalf("GetAppDetail calls = %d, want 1", got)
	}
	for _, ownedID := range []int{100, 80} {
		if _, exists := env.userApp(t, testSteamID, ownedID); !exists {
			t.Fatalf("user not linked through %d", ownedID)
		}
	}
}

func TestDuplicateOwnedIDsCountedOnce(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10, 10)
	env.steam.setApp(10, fakeApp{})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	if got := env.steam.callCount(10); got != 1 {
		t.Fatalf("GetAppDetail calls = %d, want 1", got)
	}
}

func TestNotFoundAppIsSkipped(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 999, 10)
	env.steam.setApp(10, fakeApp{})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 2, processed: 1, skipped: 1})
	if _, exists := env.userApp(t, testSteamID, 999); exists {
		t.Fatalf("user should not be linked to a missing app")
	}
}

func TestNilAppDataIsSkipped(t *testing.T) {
	env := newTestEnv(t)
	env.importer.steam = nilDataSteam{env.steam}
	env.steam.setOwned(testSteamID, 10)

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, skipped: 1})
}

type nilDataSteam struct{ *fakeSteam }

func (nilDataSteam) GetAppDetail(context.Context, int) (*steamapi.AppDetail, error) {
	return &steamapi.AppDetail{Success: true}, nil
}

func TestRateLimitedThenSucceeds(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{})
	env.steam.rateLimits[10] = 2

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	if got := env.steam.callCount(10); got != 3 {
		t.Fatalf("GetAppDetail calls = %d, want 3", got)
	}
}

func TestRepeatedRateLimitCountsAsFailed(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10, 11)
	env.steam.setApp(10, fakeApp{})
	env.steam.setApp(11, fakeApp{})
	env.steam.rateLimits[10] = 100

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 2, processed: 1, failed: 1})
	if got := env.steam.callCount(10); got != 1+maxRateLimitRetries {
		t.Fatalf("GetAppDetail calls = %d, want %d", got, 1+maxRateLimitRetries)
	}
	if _, exists := env.userApp(t, testSteamID, 10); exists {
		t.Fatalf("failed app should not be linked")
	}
}

func TestOtherAppErrorCountsAsFailed(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10, 11)
	env.steam.setApp(11, fakeApp{})
	env.steam.appErrs[10] = errors.New("connection reset")

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 2, processed: 1, failed: 1})
	if got := env.steam.callCount(10); got != 1 {
		t.Fatalf("non-rate-limit errors should not be retried, got %d calls", got)
	}
}

func TestInvalidGenreRollsBackApp(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{genres: []steamapi.Genre{{ID: "not-a-number", Description: "Bad"}}})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, failed: 1})
	if _, err := env.queries.GetAppByAlias(context.Background(), 10); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("app should have been rolled back, got %v", err)
	}
}

func TestAppIsRefetchedAcrossJobs(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{name: "Old Name"})
	env.runImport(t, testSteamID)

	env.clock.Advance(testRefreshInterval - time.Minute)
	env.steam.setApp(10, fakeApp{name: "New Name"})
	job := env.runImport(t, testSteamID)

	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	if got := env.steam.callCount(10); got != 2 {
		t.Fatalf("GetAppDetail calls = %d, want 2; freshness is the client cache's responsibility", got)
	}
	app, err := env.queries.GetAppByAlias(context.Background(), 10)
	if err != nil || app.Name.String != "New Name" {
		t.Fatalf("app = %+v, %v", app, err)
	}
}

func TestStaleAppIsRefetched(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{
		name:       "Old Name",
		categories: []steamapi.Category{{ID: 1, Description: "Multi-player"}, {ID: 2, Description: "Single-player"}},
		genres:     []steamapi.Genre{{ID: "1", Description: "Action"}},
	})
	env.runImport(t, testSteamID)

	env.steam.setApp(10, fakeApp{
		name:       "New Name",
		categories: []steamapi.Category{{ID: 3, Description: "Co-op"}},
		genres:     []steamapi.Genre{{ID: "2", Description: "Strategy"}},
	})
	env.clock.Advance(testRefreshInterval)
	job := env.runImport(t, testSteamID)

	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	if got := env.steam.callCount(10); got != 2 {
		t.Fatalf("GetAppDetail calls = %d, want 2", got)
	}
	app, err := env.queries.GetAppByAlias(context.Background(), 10)
	if err != nil || app.Name.String != "New Name" || !app.UpdatedAt.Equal(env.clock.Now()) {
		t.Fatalf("app = %+v, %v", app, err)
	}
	if got := env.categoryIDs(t, 10); !slices.Equal(got, []int32{3}) {
		t.Fatalf("categories = %v, want [3]", got)
	}
	if got := env.genreIDs(t, 10); !slices.Equal(got, []int32{2}) {
		t.Fatalf("genres = %v, want [2]", got)
	}
}

func TestRemovedGames(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setApp(10, fakeApp{})
	env.steam.setApp(11, fakeApp{})

	env.steam.setOwned(testSteamID, 10, 11)
	env.runImport(t, testSteamID)

	env.steam.setOwned(testSteamID, 10)
	env.runImport(t, testSteamID)
	if removed, _ := env.userApp(t, testSteamID, 11); !removed {
		t.Fatalf("app 11 should be marked removed")
	}
	if removed, _ := env.userApp(t, testSteamID, 10); removed {
		t.Fatalf("app 10 should not be marked removed")
	}
	active, err := env.queries.ListActiveSteamUserApps(context.Background(), testSteamID)
	if err != nil || len(active) != 1 || active[0].SteamAppid != 10 {
		t.Fatalf("active apps = %+v, %v", active, err)
	}

	env.steam.setOwned(testSteamID, 10, 11)
	env.runImport(t, testSteamID)
	if removed, _ := env.userApp(t, testSteamID, 11); removed {
		t.Fatalf("re-owned app 11 should no longer be removed")
	}

	// A private profile or empty library must not mark anything removed.
	env.steam.setOwned(testSteamID)
	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{})
	for _, ownedID := range []int{10, 11} {
		if removed, _ := env.userApp(t, testSteamID, ownedID); removed {
			t.Fatalf("empty library marked app %d removed", ownedID)
		}
	}
}

func TestWishlistImport(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 20, DateAdded: 1700000000})
	env.steam.setApp(20, fakeApp{
		name:       "Wishlisted Game",
		categories: []steamapi.Category{{ID: 2, Description: "Single-player"}, {ID: 1, Description: "Multi-player"}},
		genres:     []steamapi.Genre{{ID: "1", Description: "Action"}},
	})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{})
	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, processed: 1})

	ctx := context.Background()
	app, err := env.queries.GetAppByAlias(ctx, 20)
	if err != nil || app.SteamAppid != 20 || app.Name.String != "Wishlisted Game" {
		t.Fatalf("app = %+v, %v", app, err)
	}
	alias, err := env.queries.GetAppAlias(ctx, 20)
	if err != nil || alias.AppID != 20 {
		t.Fatalf("self-alias = %+v, %v", alias, err)
	}
	if got := env.categoryIDs(t, 20); !slices.Equal(got, []int32{1, 2}) {
		t.Fatalf("categories = %v", got)
	}
	if got := env.genreIDs(t, 20); !slices.Equal(got, []int32{1}) {
		t.Fatalf("genres = %v", got)
	}

	row, exists := env.userWishlist(t, testSteamID, 20)
	if !exists || row.Removed || !row.DateAdded.Valid {
		t.Fatalf("wishlist row = %+v, exists %v", row, exists)
	}
	if want := time.Unix(1700000000, 0).UTC(); !row.DateAdded.Time.Equal(want) {
		t.Fatalf("date_added = %v, want %v", row.DateAdded.Time, want)
	}
	if _, exists := env.userApp(t, testSteamID, 20); exists {
		t.Fatalf("wishlist app should not be linked in steam_user_apps")
	}

	active, err := env.queries.ListActiveSteamUserWishlist(ctx, testSteamID)
	if err != nil || len(active) != 1 || active[0].SteamAppid != 20 {
		t.Fatalf("active wishlist = %+v, %v", active, err)
	}
}

func TestWishlistAliasResolvesToCanonicalApp(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 100})
	env.steam.setApp(100, fakeApp{canonicalID: 80})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{})
	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, processed: 1})

	ctx := context.Background()
	for _, aliasID := range []int32{80, 100} {
		alias, err := env.queries.GetAppAlias(ctx, aliasID)
		if err != nil || alias.AppID != 80 {
			t.Fatalf("alias %d = %+v, %v", aliasID, alias, err)
		}
	}
	if _, exists := env.userWishlist(t, testSteamID, 100); !exists {
		t.Fatalf("wishlist should be linked through Steam ID 100")
	}
	if _, exists := env.userWishlist(t, testSteamID, 80); exists {
		t.Fatalf("wishlist should not be linked through canonical ID 80")
	}
	if _, exists := env.userApp(t, testSteamID, 100); exists {
		t.Fatalf("wishlist app should not be linked in steam_user_apps")
	}
}

func TestWishlistOverlapsOwnedApp(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	env.steam.setApp(10, fakeApp{})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, processed: 1})

	if _, exists := env.userApp(t, testSteamID, 10); !exists {
		t.Fatalf("app should stay in steam_user_apps")
	}
	if _, exists := env.userWishlist(t, testSteamID, 10); !exists {
		t.Fatalf("app should also be in steam_user_wishlist")
	}
	if got := env.steam.callCount(10); got != 1 {
		t.Fatalf("GetAppDetail calls = %d, want 1", got)
	}
}

func TestWishlistAppIsRefetchedAcrossJobs(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	env.steam.setApp(10, fakeApp{name: "Old Name"})
	env.runImport(t, testSteamID)

	env.clock.Advance(testRefreshInterval - time.Minute)
	env.steam.setApp(10, fakeApp{name: "New Name"})
	job := env.runImport(t, testSteamID)

	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, processed: 1})
	if got := env.steam.callCount(10); got != 2 {
		t.Fatalf("GetAppDetail calls = %d, want 2; freshness is the client cache's responsibility", got)
	}
	if _, exists := env.userWishlist(t, testSteamID, 10); !exists {
		t.Fatalf("wishlist app should still be linked")
	}
	app, err := env.queries.GetAppByAlias(context.Background(), 10)
	if err != nil || app.Name.String != "New Name" {
		t.Fatalf("app = %+v, %v", app, err)
	}
}

func TestStaleWishlistAppIsRefetched(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	env.steam.setApp(10, fakeApp{
		categories: []steamapi.Category{{ID: 1, Description: "Multi-player"}, {ID: 2, Description: "Single-player"}},
		genres:     []steamapi.Genre{{ID: "1", Description: "Action"}},
	})
	env.runImport(t, testSteamID)

	env.steam.setApp(10, fakeApp{
		categories: []steamapi.Category{{ID: 3, Description: "Co-op"}},
		genres:     []steamapi.Genre{{ID: "2", Description: "Strategy"}},
	})
	env.clock.Advance(testRefreshInterval)
	job := env.runImport(t, testSteamID)

	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, processed: 1})
	if got := env.steam.callCount(10); got != 2 {
		t.Fatalf("GetAppDetail calls = %d, want 2", got)
	}
	if got := env.categoryIDs(t, 10); !slices.Equal(got, []int32{3}) {
		t.Fatalf("categories = %v, want [3]", got)
	}
	if got := env.genreIDs(t, 10); !slices.Equal(got, []int32{2}) {
		t.Fatalf("genres = %v, want [2]", got)
	}
}

func TestWishlistAppNotFoundIsSkipped(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 999})

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{})
	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, skipped: 1})
	if _, exists := env.userWishlist(t, testSteamID, 999); exists {
		t.Fatalf("a missing app should not be linked to the wishlist")
	}
}

func TestDuplicateWishlistItemsFetchedOnce(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setWishlist(testSteamID,
		steamapi.WishlistItem{AppID: 10, DateAdded: 100},
		steamapi.WishlistItem{AppID: 10, DateAdded: 200},
	)
	env.steam.setApp(10, fakeApp{})

	job := env.runImport(t, testSteamID)
	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, processed: 1})
	if got := env.steam.callCount(10); got != 1 {
		t.Fatalf("GetAppDetail calls = %d, want 1", got)
	}
	row, exists := env.userWishlist(t, testSteamID, 10)
	if !exists || !row.DateAdded.Valid {
		t.Fatalf("wishlist row = %+v, exists %v", row, exists)
	}
	if want := time.Unix(200, 0).UTC(); !row.DateAdded.Time.Equal(want) {
		t.Fatalf("date_added = %v, want the largest value %v", row.DateAdded.Time, want)
	}
}

func TestRemovedWishlistItems(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setApp(10, fakeApp{})
	env.steam.setApp(11, fakeApp{})

	env.steam.setWishlist(testSteamID,
		steamapi.WishlistItem{AppID: 10},
		steamapi.WishlistItem{AppID: 11},
	)
	env.runImport(t, testSteamID)

	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	env.runImport(t, testSteamID)
	if row, _ := env.userWishlist(t, testSteamID, 11); !row.Removed {
		t.Fatalf("wishlist app 11 should be marked removed")
	}
	if row, _ := env.userWishlist(t, testSteamID, 10); row.Removed {
		t.Fatalf("wishlist app 10 should not be marked removed")
	}
	active, err := env.queries.ListActiveSteamUserWishlist(context.Background(), testSteamID)
	if err != nil || len(active) != 1 || active[0].SteamAppid != 10 {
		t.Fatalf("active wishlist = %+v, %v", active, err)
	}

	env.steam.setWishlist(testSteamID,
		steamapi.WishlistItem{AppID: 10},
		steamapi.WishlistItem{AppID: 11},
	)
	env.runImport(t, testSteamID)
	if row, _ := env.userWishlist(t, testSteamID, 11); row.Removed {
		t.Fatalf("re-added wishlist app 11 should no longer be removed")
	}
}

func TestEmptyWishlistDoesNotRemoveItems(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setApp(10, fakeApp{})
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	env.runImport(t, testSteamID)

	env.steam.setWishlist(testSteamID)
	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{})
	assertWishlistCounts(t, job, wantWishlistCounts{})

	if row, exists := env.userWishlist(t, testSteamID, 10); !exists || row.Removed {
		t.Fatalf("empty wishlist should not mark rows removed: %+v, exists %v", row, exists)
	}
}

func TestPrivateWishlist(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{})
	env.steam.setApp(20, fakeApp{})
	env.steam.setApp(21, fakeApp{})
	env.steam.setWishlist(testSteamID,
		steamapi.WishlistItem{AppID: 20},
		steamapi.WishlistItem{AppID: 21},
	)
	env.runImport(t, testSteamID)

	env.steam.wishlistPrivate = true
	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	assertWishlistCounts(t, job, wantWishlistCounts{})

	if !env.wishlistPrivate(t, testSteamID) {
		t.Fatalf("private wishlist should set the user flag")
	}
	for _, appID := range []int{20, 21} {
		if row, _ := env.userWishlist(t, testSteamID, appID); row.Removed {
			t.Fatalf("private wishlist marked app %d removed", appID)
		}
	}
}

func TestWishlistFetchErrorKeepsState(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setApp(10, fakeApp{})
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	env.runImport(t, testSteamID)

	env.steam.wishlistPrivate = true
	env.runImport(t, testSteamID)
	if !env.wishlistPrivate(t, testSteamID) {
		t.Fatalf("private wishlist should set the user flag")
	}

	// A non-privacy error leaves the flag and the stored wishlist unchanged.
	env.steam.wishlistPrivate = false
	env.steam.wishlistErr = errors.New("steam is down")
	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{})
	assertWishlistCounts(t, job, wantWishlistCounts{})

	if !env.wishlistPrivate(t, testSteamID) {
		t.Fatalf("non-privacy error should not clear the private flag")
	}
	if row, exists := env.userWishlist(t, testSteamID, 10); !exists || row.Removed {
		t.Fatalf("fetch error should not mark rows removed: %+v, exists %v", row, exists)
	}
}

func TestWishlistRecoveryClearsPrivateFlag(t *testing.T) {
	env := newTestEnv(t)
	env.steam.wishlistPrivate = true
	env.runImport(t, testSteamID)
	if !env.wishlistPrivate(t, testSteamID) {
		t.Fatalf("private wishlist should set the user flag")
	}

	env.steam.wishlistPrivate = false
	env.steam.setApp(10, fakeApp{})
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	job := env.runImport(t, testSteamID)
	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, processed: 1})

	if env.wishlistPrivate(t, testSteamID) {
		t.Fatalf("successful wishlist fetch should clear the private flag")
	}
	if _, exists := env.userWishlist(t, testSteamID, 10); !exists {
		t.Fatalf("recovered wishlist should be linked")
	}
}

func TestOwnedAndWishlistCountsAreIndependent(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{})
	env.steam.setApp(20, fakeApp{})
	env.steam.appErrs[22] = errors.New("connection reset")
	env.steam.setWishlist(testSteamID,
		steamapi.WishlistItem{AppID: 20},
		steamapi.WishlistItem{AppID: 21}, // not found
		steamapi.WishlistItem{AppID: 22}, // detail failure
	)

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	assertWishlistCounts(t, job, wantWishlistCounts{total: 3, processed: 1, skipped: 1, failed: 1})
}

func TestWishlistCancellationLeavesJobRunning(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setApp(10, fakeApp{})
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	env.steam.block[10] = true
	env.steam.called = make(chan int, 1)

	if err := env.importer.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	id := env.createJob(t, testSteamID)
	env.importer.Enqueue(id)

	select {
	case <-env.steam.called:
	case <-time.After(5 * time.Second):
		t.Fatalf("GetAppDetail was never called")
	}
	shutdown(t, env.importer)

	job := env.getJob(t, id)
	if job.Status != StatusRunning || job.FinishedAt.Valid {
		t.Fatalf("interrupted job = %+v, want running", job)
	}
	if job.WishlistTotalApps != 1 || job.WishlistProcessedApps != 0 {
		t.Fatalf("wishlist counts = %d/%d, want 1/0", job.WishlistTotalApps, job.WishlistProcessedApps)
	}
	if _, exists := env.userWishlist(t, testSteamID, 10); exists {
		t.Fatalf("cancelled wishlist app should not be linked")
	}
}

func TestWishlistRemovalFailureFailsJob(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setApp(10, fakeApp{})
	env.steam.setApp(11, fakeApp{})
	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 10})
	env.runImport(t, testSteamID)

	if _, err := env.db.Exec(`
		CREATE FUNCTION fail_wishlist_update() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'wishlist update blocked';
		END;
		$$ LANGUAGE plpgsql`); err != nil {
		t.Fatalf("create trigger function: %v", err)
	}
	if _, err := env.db.Exec(`
		CREATE TRIGGER fail_wishlist_update
		BEFORE UPDATE ON steam_user_wishlist
		FOR EACH ROW EXECUTE FUNCTION fail_wishlist_update()`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	env.steam.setWishlist(testSteamID, steamapi.WishlistItem{AppID: 11})
	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusFailed, wantCounts{})
	assertWishlistCounts(t, job, wantWishlistCounts{total: 1, processed: 1})
	if !job.Error.Valid || !strings.Contains(job.Error.String, "mark removed wishlist apps") {
		t.Fatalf("job error = %q", job.Error.String)
	}
}

func TestGetOwnedGamesFailureFailsJob(t *testing.T) {
	env := newTestEnv(t)
	env.steam.ownedErr = errors.New("steam is down")

	job := env.runImport(t, testSteamID)
	assertJob(t, job, StatusFailed, wantCounts{})
	if !job.Error.Valid || job.Error.String == "" {
		t.Fatalf("job error not recorded")
	}
}

func TestFinishedJobIsNotRerun(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{})

	job := env.runImport(t, testSteamID)
	env.importer.runJob(context.Background(), job.ID)

	if got := env.steam.callCount(10); got != 1 {
		t.Fatalf("finished job was rerun: %d calls", got)
	}
	if again := env.getJob(t, job.ID); again.Status != StatusSucceeded {
		t.Fatalf("status = %q", again.Status)
	}
}

func TestStartResumesUnfinishedJobs(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{})
	ctx := context.Background()

	queuedID := env.createJob(t, testSteamID)
	runningID := env.createJob(t, testSteamID)
	if _, err := env.queries.MarkImportJobRunning(ctx, dbgen.MarkImportJobRunningParams{
		StartedAt: sql.NullTime{Time: env.clock.Now(), Valid: true},
		ID:        runningID,
	}); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	if err := env.importer.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { shutdown(t, env.importer) })

	for _, id := range []int64{queuedID, runningID} {
		job := env.waitForStatus(t, id, StatusSucceeded)
		assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	}
}

func TestEnqueueRunsJob(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setApp(10, fakeApp{})

	if err := env.importer.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { shutdown(t, env.importer) })

	id := env.createJob(t, testSteamID)
	env.importer.Enqueue(id)
	env.waitForStatus(t, id, StatusSucceeded)
}

func TestShutdownLeavesJobRunning(t *testing.T) {
	env := newTestEnv(t)
	env.steam.setOwned(testSteamID, 10)
	env.steam.block[10] = true
	env.steam.called = make(chan int, 1)

	if err := env.importer.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	id := env.createJob(t, testSteamID)
	env.importer.Enqueue(id)

	select {
	case <-env.steam.called:
	case <-time.After(5 * time.Second):
		t.Fatalf("GetAppDetail was never called")
	}
	shutdown(t, env.importer)

	job := env.getJob(t, id)
	if job.Status != StatusRunning || job.FinishedAt.Valid || job.FailedApps != 0 {
		t.Fatalf("interrupted job = %+v, want running with no failures", job)
	}
}

func TestEnqueueDoesNotBlockWhenFull(t *testing.T) {
	importer := New(nil, newFakeSteam(), nil, testRefreshInterval)
	done := make(chan struct{})
	go func() {
		for i := range queueSize + 5 {
			importer.Enqueue(int64(i))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("Enqueue blocked on a full queue")
	}
	if got := len(importer.jobs); got != queueSize {
		t.Fatalf("queue length = %d, want %d", got, queueSize)
	}
}

func TestRequestImport(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	finish := func(id int64, status string, at time.Time) {
		t.Helper()
		if _, err := env.queries.FinishImportJob(ctx, dbgen.FinishImportJobParams{
			Status:     status,
			FinishedAt: sql.NullTime{Time: at, Valid: true},
			ID:         id,
		}); err != nil {
			t.Fatalf("finish job: %v", err)
		}
	}

	first, created, err := env.importer.RequestImport(ctx, testSteamID)
	if err != nil || !created || first.Status != StatusQueued {
		t.Fatalf("new user: job %+v, created %v, err %v", first, created, err)
	}
	if got := len(env.importer.jobs); got != 1 {
		t.Fatalf("queued %d jobs, want 1", got)
	}

	active, created, err := env.importer.RequestImport(ctx, testSteamID)
	if err != nil || created || active.ID != first.ID {
		t.Fatalf("active job: id %d, created %v, err %v", active.ID, created, err)
	}

	finish(first.ID, StatusSucceeded, env.clock.Now())
	env.clock.Advance(testRefreshInterval - time.Minute)
	fresh, created, err := env.importer.RequestImport(ctx, testSteamID)
	if err != nil || created || fresh.ID != first.ID {
		t.Fatalf("fresh job: id %d, created %v, err %v", fresh.ID, created, err)
	}

	env.clock.Advance(time.Minute)
	stale, created, err := env.importer.RequestImport(ctx, testSteamID)
	if err != nil || !created || stale.ID == first.ID {
		t.Fatalf("stale job: id %d, created %v, err %v", stale.ID, created, err)
	}

	// The latest job failed and the last success is stale: a new job is due.
	finish(stale.ID, StatusFailed, env.clock.Now())
	retry, created, err := env.importer.RequestImport(ctx, testSteamID)
	if err != nil || !created || retry.ID == stale.ID {
		t.Fatalf("after failure: id %d, created %v, err %v", retry.ID, created, err)
	}
}

func TestRequestImportConcurrentCreatesOneJob(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	ids := make(chan int64, 10)
	for range 10 {
		wg.Go(func() {
			job, _, err := env.importer.RequestImport(ctx, testSteamID)
			if err != nil {
				t.Errorf("RequestImport: %v", err)
				return
			}
			ids <- job.ID
		})
	}
	wg.Wait()
	close(ids)

	first := int64(-1)
	for id := range ids {
		if first == -1 {
			first = id
		} else if id != first {
			t.Fatalf("concurrent requests created jobs %d and %d", first, id)
		}
	}
}

func shutdown(t *testing.T, importer *Importer) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := importer.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}

// finishJobAt closes a job with the given status and finished_at time.
func (env *testEnv) finishJobAt(t *testing.T, id int64, status string, at time.Time) {
	t.Helper()
	if _, err := env.queries.FinishImportJob(context.Background(), dbgen.FinishImportJobParams{
		Status:     status,
		FinishedAt: sql.NullTime{Time: at, Valid: true},
		ID:         id,
	}); err != nil {
		t.Fatalf("finish job: %v", err)
	}
}

func TestSnapshotNotImported(t *testing.T) {
	env := newTestEnv(t)

	snapshot, err := env.importer.Snapshot(context.Background(), testSteamID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snapshot.State != StateNotImported || snapshot.Latest != nil || snapshot.HasActiveJob {
		t.Fatalf("snapshot = %+v, want not_imported with no job", snapshot)
	}
	if got := len(env.importer.jobs); got != 0 {
		t.Fatalf("Snapshot enqueued %d jobs, want 0", got)
	}
}

func TestSnapshotActiveJobs(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	id := env.createJob(t, testSteamID)

	snapshot, err := env.importer.Snapshot(ctx, testSteamID)
	if err != nil {
		t.Fatalf("Snapshot queued: %v", err)
	}
	if snapshot.State != StateQueued || !snapshot.HasActiveJob || snapshot.Latest == nil || snapshot.Latest.ID != id {
		t.Fatalf("queued snapshot = %+v, want queued job %d", snapshot, id)
	}

	if _, err := env.queries.MarkImportJobRunning(ctx, dbgen.MarkImportJobRunningParams{
		StartedAt: sql.NullTime{Time: env.clock.Now(), Valid: true},
		ID:        id,
	}); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	snapshot, err = env.importer.Snapshot(ctx, testSteamID)
	if err != nil {
		t.Fatalf("Snapshot running: %v", err)
	}
	if snapshot.State != StateRunning || !snapshot.HasActiveJob || snapshot.Latest == nil || snapshot.Latest.ID != id {
		t.Fatalf("running snapshot = %+v, want running job %d", snapshot, id)
	}
	if got := len(env.importer.jobs); got != 0 {
		t.Fatalf("Snapshot enqueued %d jobs, want 0", got)
	}
}

func TestSnapshotUpToDate(t *testing.T) {
	env := newTestEnv(t)
	id := env.createJob(t, testSteamID)
	env.finishJobAt(t, id, StatusSucceeded, env.clock.Now())

	snapshot, err := env.importer.Snapshot(context.Background(), testSteamID)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snapshot.State != StateUpToDate || snapshot.HasActiveJob || snapshot.Latest == nil || snapshot.Latest.ID != id {
		t.Fatalf("snapshot = %+v, want up_to_date job %d", snapshot, id)
	}
}

func TestSnapshotOutOfDate(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, env *testEnv) int64
	}{
		{
			name: "stale success",
			prepare: func(t *testing.T, env *testEnv) int64 {
				id := env.createJob(t, testSteamID)
				env.finishJobAt(t, id, StatusSucceeded, env.clock.Now())
				env.clock.Advance(testRefreshInterval + time.Minute)
				return id
			},
		},
		{
			name: "success without finished_at",
			prepare: func(t *testing.T, env *testEnv) int64 {
				id := env.createJob(t, testSteamID)
				if _, err := env.db.Exec(
					"UPDATE import_jobs SET status = $1 WHERE id = $2", StatusSucceeded, id,
				); err != nil {
					t.Fatalf("mark succeeded: %v", err)
				}
				return id
			},
		},
		{
			name: "failed latest beats earlier success",
			prepare: func(t *testing.T, env *testEnv) int64 {
				success := env.createJob(t, testSteamID)
				env.finishJobAt(t, success, StatusSucceeded, env.clock.Now())
				failed := env.createJob(t, testSteamID)
				env.clock.Advance(time.Minute)
				env.finishJobAt(t, failed, StatusFailed, env.clock.Now())
				return failed
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newTestEnv(t)
			want := test.prepare(t, env)

			snapshot, err := env.importer.Snapshot(context.Background(), testSteamID)
			if err != nil {
				t.Fatalf("Snapshot: %v", err)
			}
			if snapshot.State != StateOutOfDate || snapshot.HasActiveJob {
				t.Fatalf("snapshot state = %q, active %v, want out_of_date inactive", snapshot.State, snapshot.HasActiveJob)
			}
			if snapshot.Latest == nil || snapshot.Latest.ID != want {
				t.Fatalf("latest = %+v, want job %d", snapshot.Latest, want)
			}
			if got := len(env.importer.jobs); got != 0 {
				t.Fatalf("Snapshot enqueued %d jobs, want 0", got)
			}
		})
	}
}

func TestRequestForcedImportFreshSuccess(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	original := env.createJob(t, testSteamID)
	env.finishJobAt(t, original, StatusSucceeded, env.clock.Now())

	// The non-forced path keeps its fresh-success shortcut.
	reused, created, err := env.importer.RequestImport(ctx, testSteamID)
	if err != nil || created || reused.ID != original {
		t.Fatalf("RequestImport fresh: id %d, created %v, err %v", reused.ID, created, err)
	}

	// The forced path ignores the shortcut and starts a new job.
	forced, created, err := env.importer.RequestForcedImport(ctx, testSteamID)
	if err != nil {
		t.Fatalf("RequestForcedImport: %v", err)
	}
	if !created || forced.ID == original || forced.Status != StatusQueued {
		t.Fatalf("forced job = %+v, created %v, want a new queued job", forced, created)
	}
	if got := len(env.importer.jobs); got != 1 {
		t.Fatalf("queued %d jobs, want 1", got)
	}
}

func TestRequestForcedImportReusesActiveJob(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	active := env.createJob(t, testSteamID)

	forced, created, err := env.importer.RequestForcedImport(ctx, testSteamID)
	if err != nil {
		t.Fatalf("RequestForcedImport: %v", err)
	}
	if created || forced.ID != active {
		t.Fatalf("forced job = %+v, created %v, want active job %d reused", forced, created, active)
	}
	if got := len(env.importer.jobs); got != 0 {
		t.Fatalf("queued %d jobs, want 0", got)
	}
	jobs, err := env.queries.ListUnfinishedImportJobs(ctx)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("%d unfinished jobs, want 1", len(jobs))
	}
}

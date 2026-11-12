package importer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"

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
	canonicalID int // 0 means the owned ID itself
	name        string
	categories  []steamapi.Category
	genres      []steamapi.Genre
}

type fakeSteam struct {
	mu         sync.Mutex
	owned      map[int64][]int
	ownedErr   error
	apps       map[int]fakeApp
	rateLimits map[int]int   // 429s to return before succeeding
	appErrs    map[int]error // returned on every call
	block      map[int]bool  // block until ctx is cancelled
	calls      map[int]int
	called     chan int // receives each requested app ID, if set
}

func newFakeSteam() *fakeSteam {
	return &fakeSteam{
		owned:      map[int64][]int{},
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
	importer.limiter = rate.NewLimiter(rate.Every(time.Millisecond), 1)
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
		name:       "Game Ten",
		categories: []steamapi.Category{{ID: 2, Description: "Single-player"}, {ID: 1, Description: "Multi-player"}},
		genres:     []steamapi.Genre{{ID: "1", Description: "Action"}},
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

func TestFreshAppIsNotRefetched(t *testing.T) {
	env := newTestEnv(t)
	otherUser := int64(76561198000000002)
	env.createUser(t, otherUser)
	env.steam.setOwned(testSteamID, 10)
	env.steam.setOwned(otherUser, 10)
	env.steam.setApp(10, fakeApp{})

	env.runImport(t, testSteamID)
	env.clock.Advance(testRefreshInterval - time.Minute)
	job := env.runImport(t, otherUser)

	assertJob(t, job, StatusSucceeded, wantCounts{total: 1, processed: 1})
	if got := env.steam.callCount(10); got != 1 {
		t.Fatalf("GetAppDetail calls = %d, want 1", got)
	}
	if _, exists := env.userApp(t, otherUser, 10); !exists {
		t.Fatalf("second user should still be linked")
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

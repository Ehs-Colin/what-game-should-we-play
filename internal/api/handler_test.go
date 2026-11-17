package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbtest"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

const (
	testSteamID         = int64(76561198000000001)
	testVanity          = "someplayer"
	testRefreshInterval = time.Hour
)

type fakeResolver struct {
	ids   map[string]int64
	err   error
	calls *int
}

func (f fakeResolver) ResolveVanityURL(_ context.Context, vanityUrl string) (*steamapi.VanityUrl, error) {
	if f.calls != nil {
		(*f.calls)++
	}
	if f.err != nil {
		return nil, f.err
	}
	result := &steamapi.VanityUrl{}
	if id, ok := f.ids[vanityUrl]; ok {
		result.Response.Success = 1
		result.Response.SteamID = strconv.FormatInt(id, 10)
	} else {
		result.Response.Success = 42 // Steam's "no match"
	}
	return result, nil
}

type handlerEnv struct {
	db      *sql.DB
	queries *dbgen.Queries
	mux     *http.ServeMux
}

func newHandlerEnv(t *testing.T, resolver fakeResolver) *handlerEnv {
	t.Helper()
	db := dbtest.Open(t)
	// The importer isn't started, so created jobs stay queued and Steam is
	// never called.
	jobImporter := importer.New(db, nil, nil, testRefreshInterval)
	handler := NewHandler(NewStore(db, "", resolver), jobImporter, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/steamuser", handler.InsertSteamUser)
	mux.HandleFunc("GET /api/dashboard", handler.GetDashboard)
	mux.HandleFunc("GET /api/steamusers", handler.GetSteamUserByName)
	mux.HandleFunc("GET /api/steamusers/search", handler.SearchSteamUsers)
	mux.HandleFunc("GET /api/steamusers/by-name", handler.GetSteamUserView)
	mux.HandleFunc("GET /api/import-jobs/{id}", handler.GetImportJob)
	mux.HandleFunc("POST /api/common-games", handler.GetCommonGames)
	return &handlerEnv{db: db, queries: dbgen.New(db), mux: mux}
}

func defaultResolver() fakeResolver {
	return fakeResolver{ids: map[string]int64{testVanity: testSteamID}}
}

func (env *handlerEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	recorder := httptest.NewRecorder()
	env.mux.ServeHTTP(recorder, request)
	return recorder
}

func (env *handlerEnv) postUser(t *testing.T) (*httptest.ResponseRecorder, insertSteamUserResponse) {
	t.Helper()
	recorder := env.do(t, http.MethodPost, "/api/steamuser", fmt.Sprintf(`{"vanityUrl":%q}`, testVanity))
	var body insertSteamUserResponse
	if recorder.Code < 300 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
		}
	}
	return recorder, body
}

func (env *handlerEnv) finishJob(t *testing.T, id int64, status string, at time.Time) {
	t.Helper()
	if _, err := env.queries.FinishImportJob(context.Background(), dbgen.FinishImportJobParams{
		Status:     status,
		FinishedAt: sql.NullTime{Time: at, Valid: true},
		ID:         id,
	}); err != nil {
		t.Fatalf("finish job: %v", err)
	}
}

func (env *handlerEnv) jobCount(t *testing.T) int {
	t.Helper()
	jobs, err := env.queries.ListUnfinishedImportJobs(context.Background())
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	return len(jobs)
}

func assertCreated(t *testing.T, recorder *httptest.ResponseRecorder, body insertSteamUserResponse) {
	t.Helper()
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (%s)", recorder.Code, recorder.Body.String())
	}
	if want := importJobLocation(body.ImportJob.ID); recorder.Header().Get("Location") != want {
		t.Fatalf("Location = %q, want %q", recorder.Header().Get("Location"), want)
	}
	if body.ImportJob.Status != importer.StatusQueued {
		t.Fatalf("job status = %q, want queued", body.ImportJob.Status)
	}
}

func TestInsertSteamUserNewUser(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())

	recorder, body := env.postUser(t)
	assertCreated(t, recorder, body)
	if want := strconv.FormatInt(testSteamID, 10); body.SteamUser.SteamID != want || body.SteamUser.Name != testVanity {
		t.Fatalf("steamUser = %+v", body.SteamUser)
	}
	if want := strconv.FormatInt(testSteamID, 10); body.ImportJob.SteamID != want {
		t.Fatalf("importJob.steamId = %q, want %q", body.ImportJob.SteamID, want)
	}
}

func TestInsertSteamUserActiveJob(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	_, first := env.postUser(t)

	recorder, body := env.postUser(t)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if body.ImportJob.ID != first.ImportJob.ID {
		t.Fatalf("job ID = %d, want %d", body.ImportJob.ID, first.ImportJob.ID)
	}
	if recorder.Header().Get("Location") != "" {
		t.Fatalf("unexpected Location header")
	}
	if got := env.jobCount(t); got != 1 {
		t.Fatalf("%d unfinished jobs, want 1", got)
	}
}

func TestInsertSteamUserRefreshInterval(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		finishedAt time.Duration // relative to now
		wantCode   int
	}{
		{"recent success", importer.StatusSucceeded, -time.Minute, http.StatusOK},
		{"old success", importer.StatusSucceeded, -2 * testRefreshInterval, http.StatusAccepted},
		{"recent failure", importer.StatusFailed, -time.Minute, http.StatusAccepted},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newHandlerEnv(t, defaultResolver())
			_, first := env.postUser(t)
			env.finishJob(t, first.ImportJob.ID, test.status, time.Now().Add(test.finishedAt))

			recorder, body := env.postUser(t)
			if recorder.Code != test.wantCode {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, test.wantCode, recorder.Body.String())
			}
			if test.wantCode == http.StatusOK {
				if body.ImportJob.ID != first.ImportJob.ID || body.ImportJob.FinishedAt == nil {
					t.Fatalf("importJob = %+v, want finished job %d", body.ImportJob, first.ImportJob.ID)
				}
				return
			}
			assertCreated(t, recorder, body)
			if body.ImportJob.ID == first.ImportJob.ID {
				t.Fatalf("expected a new job")
			}
		})
	}
}

func TestInsertSteamUserBadRequests(t *testing.T) {
	tests := []struct {
		name     string
		resolver fakeResolver
		body     string
		wantCode int
	}{
		{"invalid JSON", defaultResolver(), `{`, http.StatusBadRequest},
		{"missing vanityUrl", defaultResolver(), `{"vanityUrl":"  "}`, http.StatusBadRequest},
		{"unknown user", defaultResolver(), `{"vanityUrl":"nobody"}`, http.StatusNotFound},
		{"steam error", fakeResolver{err: errors.New("boom")}, `{"vanityUrl":"x"}`, http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newHandlerEnv(t, test.resolver)
			recorder := env.do(t, http.MethodPost, "/api/steamuser", test.body)
			if recorder.Code != test.wantCode {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, test.wantCode, recorder.Body.String())
			}
			if got := env.jobCount(t); got != 0 {
				t.Fatalf("%d jobs created, want 0", got)
			}
		})
	}
}

func TestGetImportJob(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	_, created := env.postUser(t)
	// Give owned and wishlist counters distinct values so the response shows
	// that they are reported separately.
	if _, err := env.queries.FinishImportJob(context.Background(), dbgen.FinishImportJobParams{
		Status:                importer.StatusSucceeded,
		TotalApps:             3,
		ProcessedApps:         2,
		SkippedApps:           1,
		WishlistTotalApps:     7,
		WishlistProcessedApps: 5,
		WishlistSkippedApps:   1,
		WishlistFailedApps:    1,
		FinishedAt:            sql.NullTime{Time: time.Now(), Valid: true},
		ID:                    created.ImportJob.ID,
	}); err != nil {
		t.Fatalf("finish job: %v", err)
	}

	recorder := env.do(t, http.MethodGet, importJobLocation(created.ImportJob.ID), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var fields map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &fields); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{
		"status", "totalApps", "processedApps", "skippedApps", "failedApps",
		"wishlistTotalApps", "wishlistProcessedApps", "wishlistSkippedApps", "wishlistFailedApps",
		"error", "createdAt", "startedAt", "finishedAt",
	} {
		if _, ok := fields[key]; !ok {
			t.Errorf("response missing %q: %s", key, recorder.Body.String())
		}
	}
	for key, want := range map[string]float64{
		"totalApps":             3,
		"processedApps":         2,
		"skippedApps":           1,
		"failedApps":            0,
		"wishlistTotalApps":     7,
		"wishlistProcessedApps": 5,
		"wishlistSkippedApps":   1,
		"wishlistFailedApps":    1,
	} {
		if fields[key] != want {
			t.Errorf("%s = %v, want %v (%s)", key, fields[key], want, recorder.Body.String())
		}
	}
	if fields["status"] != importer.StatusSucceeded || fields["finishedAt"] == nil || fields["error"] != nil {
		t.Fatalf("unexpected job: %s", recorder.Body.String())
	}

	if code := env.do(t, http.MethodGet, "/api/import-jobs/abc", "").Code; code != http.StatusBadRequest {
		t.Fatalf("non-numeric ID: status = %d, want 400", code)
	}
	if code := env.do(t, http.MethodGet, "/api/import-jobs/999999", "").Code; code != http.StatusNotFound {
		t.Fatalf("unknown ID: status = %d, want 404", code)
	}
}

func seedSteamUser(t *testing.T, env *handlerEnv, steamID int64, name string) {
	t.Helper()
	if _, err := env.queries.CreateSteamUser(context.Background(), dbgen.CreateSteamUserParams{
		SteamID: steamID,
		Name:    name,
	}); err != nil {
		t.Fatalf("create steam user: %v", err)
	}
}

func TestGetSteamUserByNameFound(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, testSteamID, testVanity)

	recorder := env.do(t, http.MethodGet, "/api/steamusers?name="+testVanity, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}

	var body struct {
		SteamUser steamUserView `json:"steamUser"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	if want := strconv.FormatInt(testSteamID, 10); body.SteamUser.SteamID != want {
		t.Fatalf("steamId = %q, want %q", body.SteamUser.SteamID, want)
	}
	if body.SteamUser.Name != testVanity {
		t.Fatalf("name = %q, want %q", body.SteamUser.Name, testVanity)
	}
}

func TestGetSteamUserByNameIsCaseInsensitive(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, testSteamID, "SomePlayer")

	recorder := env.do(t, http.MethodGet, "/api/steamusers?name=someplayer", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"name":"SomePlayer"`) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestGetSteamUserByNameReturnsNewestMatch(t *testing.T) {
	const (
		olderSteamID = int64(76561198000000002)
		newerSteamID = int64(76561198000000003)
	)
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, olderSteamID, "SomePlayer")
	seedSteamUser(t, env, newerSteamID, "someplayer")

	ctx := context.Background()
	if _, err := env.db.ExecContext(ctx, "UPDATE steam_users SET updated_at = $1 WHERE steam_id = $2", time.Now().Add(-time.Hour), olderSteamID); err != nil {
		t.Fatalf("age older user: %v", err)
	}
	if _, err := env.db.ExecContext(ctx, "UPDATE steam_users SET updated_at = $1 WHERE steam_id = $2", time.Now(), newerSteamID); err != nil {
		t.Fatalf("refresh newer user: %v", err)
	}

	recorder := env.do(t, http.MethodGet, "/api/steamusers?name=SOMEPLAYER", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}

	var body struct {
		SteamUser steamUserView `json:"steamUser"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	if want := strconv.FormatInt(newerSteamID, 10); body.SteamUser.SteamID != want {
		t.Fatalf("steamId = %q, want newest %q", body.SteamUser.SteamID, want)
	}
}

func TestGetSteamUserByNameNotFound(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())

	recorder := env.do(t, http.MethodGet, "/api/steamusers?name=nobody", "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "steam user not found") {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestGetSteamUserByNameRequiresName(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())

	for _, path := range []string{
		"/api/steamusers",
		"/api/steamusers?name=",
		"/api/steamusers?name=%20",
		"/api/steamusers?name=+++",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, path, "")
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "name is required") {
				t.Fatalf("unexpected body: %s", recorder.Body.String())
			}
		})
	}
}

func TestGetSteamUserByNameDoesNotCallSteam(t *testing.T) {
	calls := 0
	env := newHandlerEnv(t, fakeResolver{calls: &calls})
	seedSteamUser(t, env, testSteamID, testVanity)

	recorder := env.do(t, http.MethodGet, "/api/steamusers?name="+testVanity, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	if calls != 0 {
		t.Fatalf("ResolveVanityURL called %d time(s), want 0", calls)
	}
}

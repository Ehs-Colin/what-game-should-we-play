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

	"github.com/Ehs-Colin/what-game-should-we-play/internal/auth"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbtest"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

const (
	testSteamID         = int64(76561198000000001)
	testVanity          = "someplayer"
	testRefreshInterval = time.Hour

	testSteamIDString = "76561198000000001"
	testPersonaName   = "Test Persona"
	testProfileURL    = "https://steamcommunity.com/profiles/" + testSteamIDString + "/"
	testAvatar        = "https://cdn.example.test/avatars/small.jpg"
	testAvatarMedium  = "https://cdn.example.test/avatars/medium.jpg"
	testAvatarFull    = "https://cdn.example.test/avatars/full.jpg"
)

// testPlayer is the profile fakeResolver reports when no override is set.
func testPlayer(steamID string) steamapi.PlayerSummary {
	return steamapi.PlayerSummary{
		SteamID:      steamID,
		PersonaName:  testPersonaName,
		ProfileURL:   "https://steamcommunity.com/profiles/" + steamID + "/",
		Avatar:       testAvatar,
		AvatarMedium: testAvatarMedium,
		AvatarFull:   testAvatarFull,
	}
}

type fakeResolver struct {
	ids   map[string]int64
	err   error
	calls *int

	// summaries overrides the GetPlayerSummaries reply. When nil the resolver
	// returns a single fully populated player.
	summaries *steamapi.GetPlayerSummariesResponse
	// summaryErr is returned by GetPlayerSummaries instead of a profile.
	summaryErr error
}

// stubAuthService lets handler tests build the API handler without a real auth
// manager. The non-auth handlers under test never call it.
type stubAuthService struct{}

func (stubAuthService) Login(context.Context, string, string) (auth.LoginResult, error) {
	return auth.LoginResult{}, auth.ErrInvalidCredentials
}

func (stubAuthService) Logout(context.Context, string) error {
	return nil
}

func (stubAuthService) ResolveSession(context.Context, string) (auth.Principal, error) {
	return auth.Principal{}, auth.ErrNoSession
}

func (stubAuthService) CreateRealAccount(
	context.Context,
	auth.Principal,
	string,
	string,
	string,
) (auth.LoginResult, error) {
	return auth.LoginResult{}, auth.ErrAccountCreationRequired
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

func (f fakeResolver) GetPlayerSummaries(_ context.Context, steamIDs []string) (*steamapi.GetPlayerSummariesResponse, error) {
	if f.summaryErr != nil {
		return nil, f.summaryErr
	}
	if f.summaries != nil {
		return f.summaries, nil
	}
	if len(steamIDs) == 0 {
		return &steamapi.GetPlayerSummariesResponse{}, nil
	}

	steamID := steamIDs[0]
	return &steamapi.GetPlayerSummariesResponse{
		Response: steamapi.GetPlayerSummariesResponseData{
			Players: []steamapi.PlayerSummary{testPlayer(steamID)},
		},
	}, nil
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
	handler := NewHandler(NewStore(db, "", resolver), jobImporter, nil, stubAuthService{}, false, nil, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/steamuser", handler.InsertSteamUser)
	mux.HandleFunc("GET /api/dashboard", handler.GetDashboard)
	mux.HandleFunc("GET /api/steamusers", handler.GetSteamUserByName)
	mux.HandleFunc("GET /api/steamusers/search", handler.SearchSteamUsers)
	mux.HandleFunc("GET /api/steamusers/by-ids", handler.GetSteamUsersByIDs)
	mux.HandleFunc("GET /api/steamusers/by-name", handler.GetSteamUserView)
	mux.HandleFunc("GET /api/import-jobs/{id}", handler.GetImportJob)
	mux.HandleFunc("POST /api/common-games", handler.GetCommonGames)
	mux.HandleFunc("POST /api/groups", handler.CreateGroup)
	mux.HandleFunc("GET /api/groups/by-name", handler.GetGroupView)
	mux.HandleFunc("GET /api/groups/search", handler.SearchGroups)
	mux.HandleFunc("GET /api/groups/by-ids", handler.GetGroupsByIDs)
	mux.HandleFunc("PATCH /api/groups/{id}", handler.RenameGroup)
	mux.HandleFunc("DELETE /api/groups/{id}", handler.DeleteGroup)
	mux.HandleFunc("PUT /api/groups/{id}/members/{steamId}", handler.AddGroupMember)
	mux.HandleFunc("DELETE /api/groups/{id}/members/{steamId}", handler.RemoveGroupMember)
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
	// The vanity slug stays the searchable name; Steam's display name is
	// reported separately.
	if body.SteamUser.PersonaName == nil || *body.SteamUser.PersonaName != testPersonaName {
		t.Fatalf("personaName = %v, want %q", body.SteamUser.PersonaName, testPersonaName)
	}
	if body.SteamUser.ProfileURL == nil || *body.SteamUser.ProfileURL != testProfileURL {
		t.Fatalf("profileUrl = %v, want %q", body.SteamUser.ProfileURL, testProfileURL)
	}
	if body.SteamUser.Avatar == nil || *body.SteamUser.Avatar != testAvatar {
		t.Fatalf("avatar = %v, want %q", body.SteamUser.Avatar, testAvatar)
	}
	if body.SteamUser.AvatarMedium == nil || *body.SteamUser.AvatarMedium != testAvatarMedium {
		t.Fatalf("avatarMedium = %v, want %q", body.SteamUser.AvatarMedium, testAvatarMedium)
	}
	if body.SteamUser.AvatarFull == nil || *body.SteamUser.AvatarFull != testAvatarFull {
		t.Fatalf("avatarFull = %v, want %q", body.SteamUser.AvatarFull, testAvatarFull)
	}

	// The response is read back from the row the handler wrote.
	assertSteamUserProfile(t, env, testSteamID, steamProfileFields{
		personaName:  testPersonaName,
		profileURL:   testProfileURL,
		avatar:       testAvatar,
		avatarMedium: testAvatarMedium,
		avatarFull:   testAvatarFull,
	})
}

func TestInsertSteamUserRefreshesProfile(t *testing.T) {
	// The fake returns this response, so mutating it between posts stands in
	// for Steam reporting different values on the next import.
	summaries := &steamapi.GetPlayerSummariesResponse{
		Response: steamapi.GetPlayerSummariesResponseData{
			Players: []steamapi.PlayerSummary{testPlayer(testSteamIDString)},
		},
	}
	env := newHandlerEnv(t, fakeResolver{
		ids:       map[string]int64{testVanity: testSteamID},
		summaries: summaries,
	})
	recorder, first := env.postUser(t)
	assertCreated(t, recorder, first)
	if first.SteamUser.Avatar == nil {
		t.Fatalf("first insert stored no avatar: %+v", first.SteamUser)
	}

	player := &summaries.Response.Players[0]
	player.PersonaName = "Renamed Persona"
	player.ProfileURL = "https://steamcommunity.com/id/renamed/"
	player.Avatar = ""
	player.AvatarMedium = ""
	player.AvatarFull = ""

	recorder, _ = env.postUser(t)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}

	stored := assertSteamUserProfile(t, env, testSteamID, steamProfileFields{
		personaName: "Renamed Persona",
		profileURL:  "https://steamcommunity.com/id/renamed/",
	})
	if stored.Name != testVanity {
		t.Fatalf("name = %q, want the vanity slug %q", stored.Name, testVanity)
	}
}

func TestInsertSteamUserSummaryFailuresStoreNothing(t *testing.T) {
	tests := []struct {
		name     string
		resolver fakeResolver
		wantCode int
	}{
		{
			name:     "steam error",
			resolver: fakeResolver{ids: map[string]int64{testVanity: testSteamID}, summaryErr: errors.New("boom")},
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "no players",
			resolver: fakeResolver{
				ids:       map[string]int64{testVanity: testSteamID},
				summaries: &steamapi.GetPlayerSummariesResponse{},
			},
			wantCode: http.StatusNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			env := newHandlerEnv(t, test.resolver)
			recorder := env.do(t, http.MethodPost, "/api/steamuser", `{"vanityUrl":"`+testVanity+`"}`)
			if recorder.Code != test.wantCode {
				t.Fatalf("status = %d, want %d (%s)", recorder.Code, test.wantCode, recorder.Body.String())
			}
			if _, err := env.queries.GetSteamUser(context.Background(), testSteamID); !errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("GetSteamUser error = %v, want sql.ErrNoRows", err)
			}
			if got := env.jobCount(t); got != 0 {
				t.Fatalf("%d jobs created, want 0", got)
			}
		})
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

// steamProfileFields are the GetPlayerSummaries values stored on steam_users.
// An empty field is expected to be NULL, which is how InsertSteamUser records
// a value Steam omitted.
type steamProfileFields struct {
	personaName  string
	profileURL   string
	avatar       string
	avatarMedium string
	avatarFull   string
}

// seedSteamUserProfile writes profile data directly, the way InsertSteamUser
// does after a successful GetPlayerSummaries call.
func seedSteamUserProfile(t *testing.T, env *handlerEnv, steamID int64, name string, profile steamProfileFields) {
	t.Helper()
	if _, err := env.queries.UpsertSteamUser(context.Background(), dbgen.UpsertSteamUserParams{
		SteamID:      steamID,
		Name:         name,
		Personaname:  nullString(profile.personaName),
		Profileurl:   nullString(profile.profileURL),
		Avatar:       nullString(profile.avatar),
		AvatarMedium: nullString(profile.avatarMedium),
		AvatarFull:   nullString(profile.avatarFull),
	}); err != nil {
		t.Fatalf("upsert steam user profile: %v", err)
	}
}

// assertSteamUserProfile reads the stored row back and compares its profile
// columns, then returns it for any further assertions.
func assertSteamUserProfile(t *testing.T, env *handlerEnv, steamID int64, want steamProfileFields) dbgen.SteamUser {
	t.Helper()

	stored, err := env.queries.GetSteamUser(context.Background(), steamID)
	if err != nil {
		t.Fatalf("get steam user: %v", err)
	}
	checks := []struct {
		column string
		got    sql.NullString
		want   string
	}{
		{column: "personaname", got: stored.Personaname, want: want.personaName},
		{column: "profileurl", got: stored.Profileurl, want: want.profileURL},
		{column: "avatar", got: stored.Avatar, want: want.avatar},
		{column: "avatar_medium", got: stored.AvatarMedium, want: want.avatarMedium},
		{column: "avatar_full", got: stored.AvatarFull, want: want.avatarFull},
	}
	for _, check := range checks {
		if check.want == "" {
			if check.got.Valid {
				t.Fatalf("%s = %q, want NULL", check.column, check.got.String)
			}
			continue
		}
		if !check.got.Valid || check.got.String != check.want {
			t.Fatalf("%s = %q (valid=%v), want %q", check.column, check.got.String, check.got.Valid, check.want)
		}
	}
	return stored
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

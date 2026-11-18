package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
)

// seedApp creates an app with an optional name. An empty name creates a NULL
// name, which the paginated queries sort last.
func seedApp(t *testing.T, env *handlerEnv, appID int32, name string) {
	t.Helper()
	params := dbgen.CreateAppParams{SteamAppid: appID}
	if name != "" {
		params.Name = sql.NullString{String: name, Valid: true}
	}
	if _, err := env.queries.CreateApp(context.Background(), params); err != nil {
		t.Fatalf("create app %d: %v", appID, err)
	}
}

func seedAlias(t *testing.T, env *handlerEnv, appID, aliasID int32) {
	t.Helper()
	if _, err := env.queries.UpsertAppAlias(context.Background(), dbgen.UpsertAppAliasParams{
		AppID:   appID,
		AliasID: aliasID,
	}); err != nil {
		t.Fatalf("create alias %d -> %d: %v", aliasID, appID, err)
	}
}

func seedOwnedApp(t *testing.T, env *handlerEnv, steamID int64, aliasID int32, removed bool) {
	t.Helper()
	ctx := context.Background()
	if err := env.queries.AddSteamUserApp(ctx, dbgen.AddSteamUserAppParams{
		SteamID: steamID,
		AppID:   aliasID,
	}); err != nil {
		t.Fatalf("add owned app %d for %d: %v", aliasID, steamID, err)
	}
	if removed {
		if _, err := env.db.ExecContext(ctx, "UPDATE steam_user_apps SET removed = TRUE WHERE steam_id = $1 AND app_id = $2", steamID, aliasID); err != nil {
			t.Fatalf("mark owned app removed: %v", err)
		}
	}
}

func seedWishlistApp(t *testing.T, env *handlerEnv, steamID int64, aliasID int32, removed bool) {
	t.Helper()
	ctx := context.Background()
	if err := env.queries.AddSteamUserWishlistApp(ctx, dbgen.AddSteamUserWishlistAppParams{
		SteamID: steamID,
		AppID:   aliasID,
	}); err != nil {
		t.Fatalf("add wishlist app %d for %d: %v", aliasID, steamID, err)
	}
	if removed {
		if _, err := env.db.ExecContext(ctx, "UPDATE steam_user_wishlist SET removed = TRUE WHERE steam_id = $1 AND app_id = $2", steamID, aliasID); err != nil {
			t.Fatalf("mark wishlist app removed: %v", err)
		}
	}
}

func seedImportJob(t *testing.T, env *handlerEnv, steamID int64, status string, createdAt time.Time) dbgen.ImportJob {
	t.Helper()
	ctx := context.Background()
	job, err := env.queries.CreateImportJob(ctx, dbgen.CreateImportJobParams{
		SteamID:   steamID,
		CreatedAt: createdAt,
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	if status == importer.StatusQueued {
		return job
	}

	finishedAt := sql.NullTime{}
	if status == importer.StatusSucceeded || status == importer.StatusFailed {
		finishedAt = sql.NullTime{Time: createdAt.Add(time.Minute), Valid: true}
	}
	if _, err := env.db.ExecContext(ctx, "UPDATE import_jobs SET status = $1, finished_at = $2 WHERE id = $3", status, finishedAt, job.ID); err != nil {
		t.Fatalf("update import job status: %v", err)
	}
	job.Status = status
	job.FinishedAt = finishedAt
	return job
}

func requireStatus(t *testing.T, recorder *httptest.ResponseRecorder, want int) {
	t.Helper()
	if recorder.Code != want {
		t.Fatalf("status = %d, want %d (%s)", recorder.Code, want, recorder.Body.String())
	}
}

func decodeJSON[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return value
}

func decodeRawJSON(t *testing.T, recorder *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	requireStatus(t, recorder, http.StatusOK)
	var value map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return value
}

func assertJSONNumber(t *testing.T, raw json.RawMessage, want int64) {
	t.Helper()
	var got int64
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("value %s is not a JSON integer: %v", raw, err)
	}
	if got != want {
		t.Fatalf("value = %d, want %d", got, want)
	}
}

func assertJSONString(t *testing.T, raw json.RawMessage, want string) {
	t.Helper()
	var got string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("value %s is not a JSON string: %v", raw, err)
	}
	if got != want {
		t.Fatalf("value = %q, want %q", got, want)
	}
}

func summariesByName(users []steamUserSummaryView) map[string]steamUserSummaryView {
	byName := make(map[string]steamUserSummaryView, len(users))
	for _, user := range users {
		byName[user.Name] = user
	}
	return byName
}

func latestImportStatus(user steamUserSummaryView) string {
	if user.LatestImportStatus == nil {
		return ""
	}
	return *user.LatestImportStatus
}

func appIDs(apps []appSummaryView) []int32 {
	ids := make([]int32, len(apps))
	for index, app := range apps {
		ids[index] = app.AppID
	}
	return ids
}

func assertAppIDs(t *testing.T, apps []appSummaryView, want ...int32) {
	t.Helper()
	got := appIDs(apps)
	if len(got) != len(want) {
		t.Fatalf("app IDs = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("app IDs = %v, want %v", got, want)
		}
	}
}

func appName(app appSummaryView) string {
	if app.Name == nil {
		return "<nil>"
	}
	return *app.Name
}

func TestDashboardEmpty(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())

	recorder := env.do(t, http.MethodGet, "/api/dashboard", "")
	requireStatus(t, recorder, http.StatusOK)

	raw := decodeRawJSON(t, recorder)
	for _, key := range []string{"totalSteamUsers", "totalApps", "importJobs"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("response missing %q: %s", key, recorder.Body.String())
		}
	}
	for _, key := range []string{"total_steam_users", "total_apps", "import_jobs"} {
		if _, ok := raw[key]; ok {
			t.Fatalf("response used snake_case key %q: %s", key, recorder.Body.String())
		}
	}
	assertJSONNumber(t, raw["totalSteamUsers"], 0)
	assertJSONNumber(t, raw["totalApps"], 0)

	var jobs map[string]json.RawMessage
	if err := json.Unmarshal(raw["importJobs"], &jobs); err != nil {
		t.Fatalf("decode importJobs %s: %v", raw["importJobs"], err)
	}
	assertJSONNumber(t, jobs["running"], 0)
	assertJSONNumber(t, jobs["queued"], 0)
}

func TestDashboardCounts(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, testSteamID, "Alice")
	seedSteamUser(t, env, testSteamID+1, "Bob")
	seedApp(t, env, 10, "One")
	seedApp(t, env, 20, "Two")
	seedApp(t, env, 30, "Three")

	now := time.Now()
	seedImportJob(t, env, testSteamID, importer.StatusRunning, now.Add(-4*time.Hour))
	seedImportJob(t, env, testSteamID, importer.StatusQueued, now.Add(-3*time.Hour))
	seedImportJob(t, env, testSteamID, importer.StatusSucceeded, now.Add(-2*time.Hour))
	seedImportJob(t, env, testSteamID, importer.StatusFailed, now.Add(-time.Hour))

	recorder := env.do(t, http.MethodGet, "/api/dashboard", "")
	requireStatus(t, recorder, http.StatusOK)

	raw := decodeRawJSON(t, recorder)
	assertJSONNumber(t, raw["totalSteamUsers"], 2)
	assertJSONNumber(t, raw["totalApps"], 3)

	var jobs map[string]json.RawMessage
	if err := json.Unmarshal(raw["importJobs"], &jobs); err != nil {
		t.Fatalf("decode importJobs %s: %v", raw["importJobs"], err)
	}
	assertJSONNumber(t, jobs["running"], 1)
	assertJSONNumber(t, jobs["queued"], 1)
}

func TestSearchSteamUsersPartialCaseInsensitive(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, 1001, "SomePlayer")
	seedSteamUser(t, env, 1002, "PlayerTwo")
	seedSteamUser(t, env, 1003, "Unrelated")

	for _, query := range []string{"play", "PLAY", "Play"} {
		t.Run(query, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, "/api/steamusers/search?name="+url.QueryEscape(query), "")
			requireStatus(t, recorder, http.StatusOK)

			body := decodeJSON[searchSteamUsersResponse](t, recorder)
			if len(body.SteamUsers) != 2 {
				t.Fatalf("users = %d, want 2 (%s)", len(body.SteamUsers), recorder.Body.String())
			}
			if body.SteamUsers[0].Name != "PlayerTwo" || body.SteamUsers[1].Name != "SomePlayer" {
				t.Fatalf("unexpected user order: %+v", body.SteamUsers)
			}
		})
	}
}

func TestSearchSteamUsersTreatsWildcardsLiterally(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, 1101, "100%Valid")
	seedSteamUser(t, env, 1102, "100XValid")
	seedSteamUser(t, env, 1103, "under_score")
	seedSteamUser(t, env, 1104, "underXscore")

	tests := []struct {
		query string
		want  string
	}{
		{query: "%", want: "100%Valid"},
		{query: "_", want: "under_score"},
	}
	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, "/api/steamusers/search?name="+url.QueryEscape(test.query), "")
			requireStatus(t, recorder, http.StatusOK)

			body := decodeJSON[searchSteamUsersResponse](t, recorder)
			if len(body.SteamUsers) != 1 || body.SteamUsers[0].Name != test.want {
				t.Fatalf("users = %+v, want only %q", body.SteamUsers, test.want)
			}
		})
	}
}

func TestSearchSteamUsersCountsDeduplicateAliasesAndSkipRemoved(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, 1201, "Counter")

	seedApp(t, env, 80, "Canonical One")
	seedAlias(t, env, 80, 80)
	seedAlias(t, env, 80, 100)
	seedAlias(t, env, 80, 101)
	seedApp(t, env, 90, "Canonical Two")
	seedAlias(t, env, 90, 90)
	seedAlias(t, env, 90, 91)
	seedApp(t, env, 95, "Removed Only")
	seedAlias(t, env, 95, 95)

	seedOwnedApp(t, env, 1201, 80, false)
	seedOwnedApp(t, env, 1201, 100, false)
	seedOwnedApp(t, env, 1201, 101, true)
	seedOwnedApp(t, env, 1201, 90, false)
	seedOwnedApp(t, env, 1201, 95, true)

	seedWishlistApp(t, env, 1201, 80, false)
	seedWishlistApp(t, env, 1201, 100, false)
	seedWishlistApp(t, env, 1201, 90, false)
	seedWishlistApp(t, env, 1201, 91, true)

	recorder := env.do(t, http.MethodGet, "/api/steamusers/search?name=counter", "")
	requireStatus(t, recorder, http.StatusOK)

	body := decodeJSON[searchSteamUsersResponse](t, recorder)
	if len(body.SteamUsers) != 1 {
		t.Fatalf("users = %d, want 1 (%s)", len(body.SteamUsers), recorder.Body.String())
	}
	user := body.SteamUsers[0]
	if user.OwnedAppCount != 2 || user.WishlistAppCount != 2 {
		t.Fatalf("counts = %d owned, %d wishlist; want 2 and 2", user.OwnedAppCount, user.WishlistAppCount)
	}
}

func TestSearchSteamUsersLatestImportAndActiveJob(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	now := time.Now()

	seedSteamUser(t, env, 1301, "QueuedUser")
	seedImportJob(t, env, 1301, importer.StatusSucceeded, now.Add(-2*time.Hour))
	seedImportJob(t, env, 1301, importer.StatusQueued, now.Add(-time.Hour))

	seedSteamUser(t, env, 1302, "RunningUser")
	seedImportJob(t, env, 1302, importer.StatusRunning, now.Add(-time.Hour))

	seedSteamUser(t, env, 1303, "SucceededUser")
	seedImportJob(t, env, 1303, importer.StatusSucceeded, now.Add(-time.Hour))

	seedSteamUser(t, env, 1304, "FailedUser")
	seedImportJob(t, env, 1304, importer.StatusFailed, now.Add(-time.Hour))

	// The newest job failed, but an older running job is still active.
	seedSteamUser(t, env, 1305, "MixedUser")
	seedImportJob(t, env, 1305, importer.StatusRunning, now.Add(-2*time.Hour))
	seedImportJob(t, env, 1305, importer.StatusFailed, now.Add(-time.Hour))

	recorder := env.do(t, http.MethodGet, "/api/steamusers/search?name=user", "")
	requireStatus(t, recorder, http.StatusOK)

	body := decodeJSON[searchSteamUsersResponse](t, recorder)
	if len(body.SteamUsers) != 5 {
		t.Fatalf("users = %d, want 5 (%s)", len(body.SteamUsers), recorder.Body.String())
	}
	byName := summariesByName(body.SteamUsers)

	tests := []struct {
		name          string
		wantStatus    string
		wantActive    bool
		wantSucceeded bool
	}{
		{name: "QueuedUser", wantStatus: importer.StatusQueued, wantActive: true, wantSucceeded: true},
		{name: "RunningUser", wantStatus: importer.StatusRunning, wantActive: true},
		{name: "SucceededUser", wantStatus: importer.StatusSucceeded, wantActive: false, wantSucceeded: true},
		{name: "FailedUser", wantStatus: importer.StatusFailed, wantActive: false},
		{name: "MixedUser", wantStatus: importer.StatusFailed, wantActive: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			user, ok := byName[test.name]
			if !ok {
				t.Fatalf("missing user %q", test.name)
			}
			if got := latestImportStatus(user); got != test.wantStatus {
				t.Fatalf("latest status = %q, want %q", got, test.wantStatus)
			}
			if user.HasActiveJob != test.wantActive {
				t.Fatalf("hasActiveJob = %v, want %v", user.HasActiveJob, test.wantActive)
			}
			if user.HasSucceededImport != test.wantSucceeded {
				t.Fatalf("hasSucceededImport = %v, want %v", user.HasSucceededImport, test.wantSucceeded)
			}
		})
	}
}

func TestSearchSteamUsersNoMatchesAndBadRequests(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, 1401, "SomePlayer")

	recorder := env.do(t, http.MethodGet, "/api/steamusers/search?name=nobody", "")
	requireStatus(t, recorder, http.StatusOK)
	body := decodeJSON[searchSteamUsersResponse](t, recorder)
	if len(body.SteamUsers) != 0 {
		t.Fatalf("users = %+v, want none", body.SteamUsers)
	}
	if !json.Valid([]byte(recorder.Body.String())) || body.SteamUsers == nil {
		t.Fatalf("empty result should be a JSON array: %s", recorder.Body.String())
	}

	for _, path := range []string{
		"/api/steamusers/search",
		"/api/steamusers/search?name=",
		"/api/steamusers/search?name=%20",
		"/api/steamusers/search?name=+++",
	} {
		t.Run(path, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, path, "")
			requireStatus(t, recorder, http.StatusBadRequest)
		})
	}
}

func TestSearchSteamUsersSteamIDIsString(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, testSteamID, "SomePlayer")

	recorder := env.do(t, http.MethodGet, "/api/steamusers/search?name=someplayer", "")
	raw := decodeRawJSON(t, recorder)

	var users []json.RawMessage
	if err := json.Unmarshal(raw["steamUsers"], &users); err != nil {
		t.Fatalf("decode steamUsers %s: %v", raw["steamUsers"], err)
	}
	if len(users) != 1 {
		t.Fatalf("users = %d, want 1", len(users))
	}
	var user map[string]json.RawMessage
	if err := json.Unmarshal(users[0], &user); err != nil {
		t.Fatalf("decode user %s: %v", users[0], err)
	}
	assertJSONString(t, user["steamId"], strconv.FormatInt(testSteamID, 10))
}

func TestSearchSteamUsersHasSucceededImportField(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	now := time.Now()

	seedSteamUser(t, env, 1501, "NoJobsUser")
	seedSteamUser(t, env, 1502, "ReadyUser")
	seedImportJob(t, env, 1502, importer.StatusSucceeded, now.Add(-time.Hour))
	seedSteamUser(t, env, 1503, "FailedAfterSuccessUser")
	seedImportJob(t, env, 1503, importer.StatusSucceeded, now.Add(-2*time.Hour))
	seedImportJob(t, env, 1503, importer.StatusFailed, now.Add(-time.Hour))

	recorder := env.do(t, http.MethodGet, "/api/steamusers/search?name=user", "")
	raw := decodeRawJSON(t, recorder)

	var users []json.RawMessage
	if err := json.Unmarshal(raw["steamUsers"], &users); err != nil {
		t.Fatalf("decode steamUsers %s: %v", raw["steamUsers"], err)
	}
	if len(users) != 3 {
		t.Fatalf("users = %d, want 3 (%s)", len(users), recorder.Body.String())
	}

	byName := make(map[string]map[string]json.RawMessage, len(users))
	for _, rawUser := range users {
		var user map[string]json.RawMessage
		if err := json.Unmarshal(rawUser, &user); err != nil {
			t.Fatalf("decode user %s: %v", rawUser, err)
		}
		var name string
		if err := json.Unmarshal(user["name"], &name); err != nil {
			t.Fatalf("decode user name %s: %v", user["name"], err)
		}
		byName[name] = user
	}

	tests := map[string]bool{
		"NoJobsUser":             false,
		"ReadyUser":              true,
		"FailedAfterSuccessUser": true,
	}
	for name, want := range tests {
		t.Run(name, func(t *testing.T) {
			user, ok := byName[name]
			if !ok {
				t.Fatalf("missing user %q", name)
			}
			rawValue, ok := user["hasSucceededImport"]
			if !ok {
				t.Fatalf("hasSucceededImport is missing from %s", recorder.Body.String())
			}
			var got bool
			if err := json.Unmarshal(rawValue, &got); err != nil {
				t.Fatalf("hasSucceededImport %s is not a JSON boolean: %v", rawValue, err)
			}
			if got != want {
				t.Fatalf("hasSucceededImport = %v, want %v", got, want)
			}
		})
	}
}

func TestGetSteamUserViewExactMatchAndStringSteamID(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, testSteamID, "SomePlayer")

	recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=someplayer", "")
	requireStatus(t, recorder, http.StatusOK)

	body := decodeJSON[steamUserViewResponse](t, recorder)
	if body.SteamUser.Name != "SomePlayer" {
		t.Fatalf("name = %q, want SomePlayer", body.SteamUser.Name)
	}
	if body.SteamUser.SteamID != strconv.FormatInt(testSteamID, 10) {
		t.Fatalf("steamId = %q, want %q", body.SteamUser.SteamID, strconv.FormatInt(testSteamID, 10))
	}
	if body.Apps == nil {
		t.Fatal("apps should be an empty JSON array, not null")
	}

	raw := decodeRawJSON(t, recorder)
	var user map[string]json.RawMessage
	if err := json.Unmarshal(raw["steamUser"], &user); err != nil {
		t.Fatalf("decode steamUser %s: %v", raw["steamUser"], err)
	}
	assertJSONString(t, user["steamId"], strconv.FormatInt(testSteamID, 10))
}

func TestGetSteamUserViewIncludesAppDetails(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	const steamID = int64(1551)
	const appID = int32(730)
	seedSteamUser(t, env, steamID, "DetailUser")

	_, err := env.queries.CreateApp(context.Background(), dbgen.CreateAppParams{
		SteamAppid:           appID,
		Name:                 sql.NullString{String: "Counter-Strike 2", Valid: true},
		HeaderImage:          sql.NullString{String: "https://example.test/730.jpg", Valid: true},
		ShortDescription:     sql.NullString{String: "A tactical multiplayer shooter.", Valid: true},
		Website:              sql.NullString{String: "https://www.counter-strike.net/", Valid: true},
		PriceCurrency:        sql.NullString{String: "USD", Valid: true},
		PriceInitial:         sql.NullInt32{Int32: 1999, Valid: true},
		PriceFinal:           sql.NullInt32{Int32: 999, Valid: true},
		PriceDiscountPercent: sql.NullInt32{Int32: 50, Valid: true},
		PriceFinalFormatted:  sql.NullString{String: "$9.99", Valid: true},
		PlatformWindows:      sql.NullBool{Bool: true, Valid: true},
		PlatformMac:          sql.NullBool{Bool: false, Valid: true},
		PlatformLinux:        sql.NullBool{Bool: true, Valid: true},
		ReleaseDate:          sql.NullString{String: "21 Nov, 2024", Valid: true},
		ReleaseComingSoon:    sql.NullBool{Bool: false, Valid: true},
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	seedAlias(t, env, appID, appID)
	seedOwnedApp(t, env, steamID, appID, false)

	recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=detailuser", "")
	requireStatus(t, recorder, http.StatusOK)
	body := decodeJSON[steamUserViewResponse](t, recorder)
	if len(body.Apps) != 1 {
		t.Fatalf("apps = %d, want 1", len(body.Apps))
	}

	app := body.Apps[0]
	if app.ShortDescription == nil || *app.ShortDescription != "A tactical multiplayer shooter." {
		t.Fatalf("shortDescription = %v", app.ShortDescription)
	}
	if app.Website == nil || *app.Website != "https://www.counter-strike.net/" {
		t.Fatalf("website = %v", app.Website)
	}
	if app.PriceCurrency == nil || *app.PriceCurrency != "USD" {
		t.Fatalf("priceCurrency = %v", app.PriceCurrency)
	}
	if app.PriceInitial == nil || *app.PriceInitial != 1999 {
		t.Fatalf("priceInitial = %v", app.PriceInitial)
	}
	if app.PriceFinal == nil || *app.PriceFinal != 999 {
		t.Fatalf("priceFinal = %v", app.PriceFinal)
	}
	if app.PriceDiscountPercent == nil || *app.PriceDiscountPercent != 50 {
		t.Fatalf("priceDiscountPercent = %v", app.PriceDiscountPercent)
	}
	if app.PriceFinalFormatted == nil || *app.PriceFinalFormatted != "$9.99" {
		t.Fatalf("priceFinalFormatted = %v", app.PriceFinalFormatted)
	}
	if app.PlatformWindows == nil || !*app.PlatformWindows {
		t.Fatalf("platformWindows = %v", app.PlatformWindows)
	}
	if app.PlatformMac == nil || *app.PlatformMac {
		t.Fatalf("platformMac = %v", app.PlatformMac)
	}
	if app.PlatformLinux == nil || !*app.PlatformLinux {
		t.Fatalf("platformLinux = %v", app.PlatformLinux)
	}
	if app.ReleaseDate == nil || *app.ReleaseDate != "21 Nov, 2024" {
		t.Fatalf("releaseDate = %v", app.ReleaseDate)
	}
	if app.ReleaseComingSoon == nil || *app.ReleaseComingSoon {
		t.Fatalf("releaseComingSoon = %v", app.ReleaseComingSoon)
	}
}

func TestGetSteamUserViewReturnsNewestCaseVariant(t *testing.T) {
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

	recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=SOMEPLAYER", "")
	requireStatus(t, recorder, http.StatusOK)

	body := decodeJSON[steamUserViewResponse](t, recorder)
	if want := strconv.FormatInt(newerSteamID, 10); body.SteamUser.SteamID != want {
		t.Fatalf("steamId = %q, want newest %q", body.SteamUser.SteamID, want)
	}
}

func TestGetSteamUserViewBadRequests(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, testSteamID, "SomePlayer")

	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "missing name", path: "/api/steamusers/by-name", want: http.StatusBadRequest},
		{name: "blank name", path: "/api/steamusers/by-name?name=", want: http.StatusBadRequest},
		{name: "space name", path: "/api/steamusers/by-name?name=%20", want: http.StatusBadRequest},
		{name: "plus name", path: "/api/steamusers/by-name?name=+++", want: http.StatusBadRequest},
		{name: "unknown name", path: "/api/steamusers/by-name?name=nobody", want: http.StatusNotFound},
		{name: "invalid list", path: "/api/steamusers/by-name?name=someplayer&list=wish", want: http.StatusBadRequest},
		{name: "non-numeric page", path: "/api/steamusers/by-name?name=someplayer&page=abc", want: http.StatusBadRequest},
		{name: "zero page", path: "/api/steamusers/by-name?name=someplayer&page=0", want: http.StatusBadRequest},
		{name: "negative page", path: "/api/steamusers/by-name?name=someplayer&page=-1", want: http.StatusBadRequest},
		{name: "fractional page", path: "/api/steamusers/by-name?name=someplayer&page=1.5", want: http.StatusBadRequest},
		{name: "invalid size", path: "/api/steamusers/by-name?name=someplayer&pageSize=10", want: http.StatusBadRequest},
		{name: "zero size", path: "/api/steamusers/by-name?name=someplayer&pageSize=0", want: http.StatusBadRequest},
		{name: "large size", path: "/api/steamusers/by-name?name=someplayer&pageSize=200", want: http.StatusBadRequest},
		{name: "non-numeric size", path: "/api/steamusers/by-name?name=someplayer&pageSize=abc", want: http.StatusBadRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, test.path, "")
			requireStatus(t, recorder, test.want)
		})
	}
}

func TestGetSteamUserViewDefaultsAndPagination(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	const steamID = int64(1501)
	seedSteamUser(t, env, steamID, "PageUser")

	for index := 0; index < 30; index++ {
		appID := int32(2000 + index)
		seedApp(t, env, appID, fmt.Sprintf("Game %02d", index+1))
		seedAlias(t, env, appID, appID)
		seedOwnedApp(t, env, steamID, appID, false)
	}
	seedApp(t, env, 3000, "Wish One")
	seedAlias(t, env, 3000, 3000)
	seedWishlistApp(t, env, steamID, 3000, false)
	seedApp(t, env, 3001, "Wish Two")
	seedAlias(t, env, 3001, 3001)
	seedWishlistApp(t, env, steamID, 3001, false)

	recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=pageuser", "")
	requireStatus(t, recorder, http.StatusOK)
	body := decodeJSON[steamUserViewResponse](t, recorder)
	if body.List != appListOwned || body.Page != 1 || body.PageSize != 25 {
		t.Fatalf("defaults = list %q page %d size %d; want owned/1/25", body.List, body.Page, body.PageSize)
	}
	if body.OwnedAppCount != 30 || body.WishlistAppCount != 2 {
		t.Fatalf("counts = %d owned, %d wishlist; want 30 and 2", body.OwnedAppCount, body.WishlistAppCount)
	}
	if body.Total != 30 || body.TotalPages != 2 {
		t.Fatalf("total = %d, pages = %d; want 30 and 2", body.Total, body.TotalPages)
	}
	if len(body.Apps) != 25 {
		t.Fatalf("page 1 apps = %d, want 25", len(body.Apps))
	}
	if appName(body.Apps[0]) != "Game 01" || appName(body.Apps[24]) != "Game 25" {
		t.Fatalf("unexpected page 1 bounds: %q .. %q", appName(body.Apps[0]), appName(body.Apps[24]))
	}

	recorder = env.do(t, http.MethodGet, "/api/steamusers/by-name?name=pageuser&page=2", "")
	requireStatus(t, recorder, http.StatusOK)
	pageTwo := decodeJSON[steamUserViewResponse](t, recorder)
	if pageTwo.Page != 2 || len(pageTwo.Apps) != 5 {
		t.Fatalf("page 2 = page %d, %d apps; want page 2, 5 apps", pageTwo.Page, len(pageTwo.Apps))
	}
	if appName(pageTwo.Apps[0]) != "Game 26" || appName(pageTwo.Apps[4]) != "Game 30" {
		t.Fatalf("unexpected page 2 bounds: %q .. %q", appName(pageTwo.Apps[0]), appName(pageTwo.Apps[4]))
	}
	pageOneIDs := appIDs(body.Apps)
	pageTwoIDs := appIDs(pageTwo.Apps)
	seen := make(map[int32]bool, len(pageOneIDs)+len(pageTwoIDs))
	for _, id := range append(pageOneIDs, pageTwoIDs...) {
		if seen[id] {
			t.Fatalf("app ID %d appears on more than one page", id)
		}
		seen[id] = true
	}

	for _, pageSize := range []int{50, 100} {
		t.Run(fmt.Sprintf("pageSize %d", pageSize), func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, fmt.Sprintf("/api/steamusers/by-name?name=pageuser&pageSize=%d", pageSize), "")
			requireStatus(t, recorder, http.StatusOK)
			body := decodeJSON[steamUserViewResponse](t, recorder)
			if body.PageSize != pageSize || body.Page != 1 || body.TotalPages != 1 || len(body.Apps) != 30 {
				t.Fatalf("pageSize %d response = %+v", pageSize, body)
			}
		})
	}
}

func TestGetSteamUserViewClampsPage(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	const steamID = int64(1601)
	seedSteamUser(t, env, steamID, "ClampUser")
	for index := 0; index < 30; index++ {
		appID := int32(4000 + index)
		seedApp(t, env, appID, fmt.Sprintf("Clamp %02d", index+1))
		seedAlias(t, env, appID, appID)
		seedOwnedApp(t, env, steamID, appID, false)
	}

	recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=clampuser&page=99&pageSize=25", "")
	requireStatus(t, recorder, http.StatusOK)
	body := decodeJSON[steamUserViewResponse](t, recorder)
	if body.Page != 2 || len(body.Apps) != 5 || body.TotalPages != 2 {
		t.Fatalf("clamped response = page %d, %d apps, %d pages", body.Page, len(body.Apps), body.TotalPages)
	}
	if appName(body.Apps[0]) != "Clamp 26" {
		t.Fatalf("first clamped app = %q, want Clamp 26", appName(body.Apps[0]))
	}
}

func TestGetSteamUserViewListSelection(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	const steamID = int64(1701)
	seedSteamUser(t, env, steamID, "ListUser")
	for index, name := range []string{"Owned A", "Owned B"} {
		appID := int32(5000 + index)
		seedApp(t, env, appID, name)
		seedAlias(t, env, appID, appID)
		seedOwnedApp(t, env, steamID, appID, false)
	}
	seedApp(t, env, 5100, "Wish A")
	seedAlias(t, env, 5100, 5100)
	seedWishlistApp(t, env, steamID, 5100, false)

	for _, test := range []struct {
		name      string
		query     string
		wantList  string
		wantTotal int64
		wantIDs   []int32
	}{
		{name: "default owned", query: "", wantList: appListOwned, wantTotal: 2, wantIDs: []int32{5000, 5001}},
		{name: "explicit owned", query: "&list=owned", wantList: appListOwned, wantTotal: 2, wantIDs: []int32{5000, 5001}},
		{name: "wishlist", query: "&list=wishlist", wantList: appListWishlist, wantTotal: 1, wantIDs: []int32{5100}},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=listuser"+test.query, "")
			requireStatus(t, recorder, http.StatusOK)
			body := decodeJSON[steamUserViewResponse](t, recorder)
			if body.List != test.wantList || body.Total != test.wantTotal {
				t.Fatalf("list = %q, total = %d; want %q, %d", body.List, body.Total, test.wantList, test.wantTotal)
			}
			if body.OwnedAppCount != 2 || body.WishlistAppCount != 1 {
				t.Fatalf("counts = %d owned, %d wishlist; want 2 and 1", body.OwnedAppCount, body.WishlistAppCount)
			}
			assertAppIDs(t, body.Apps, test.wantIDs...)
		})
	}
}

func TestGetSteamUserViewEmptyList(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	const steamID = int64(1801)
	seedSteamUser(t, env, steamID, "EmptyUser")

	recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=emptyuser", "")
	requireStatus(t, recorder, http.StatusOK)
	body := decodeJSON[steamUserViewResponse](t, recorder)
	if body.Total != 0 || body.TotalPages != 0 || body.Page != 1 || body.PageSize != 25 {
		t.Fatalf("empty response = total %d, pages %d, page %d, size %d", body.Total, body.TotalPages, body.Page, body.PageSize)
	}
	if body.Apps == nil || len(body.Apps) != 0 {
		t.Fatalf("apps = %#v, want empty non-nil slice", body.Apps)
	}
	if body.LatestImport != nil || body.HasActiveJob {
		t.Fatalf("latestImport = %+v, hasActiveJob = %v; want nil/false", body.LatestImport, body.HasActiveJob)
	}
}

func TestGetSteamUserViewOrdering(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	const steamID = int64(1901)
	seedSteamUser(t, env, steamID, "OrderUser")

	apps := []struct {
		id   int32
		name string
	}{
		{id: 300, name: "Beta"},
		{id: 200, name: "Alpha"},
		{id: 100, name: "Alpha"},
		{id: 400, name: ""},
		{id: 500, name: "Gamma"},
	}
	for _, app := range apps {
		seedApp(t, env, app.id, app.name)
		seedAlias(t, env, app.id, app.id)
		seedOwnedApp(t, env, steamID, app.id, false)
	}

	recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=orderuser", "")
	requireStatus(t, recorder, http.StatusOK)
	body := decodeJSON[steamUserViewResponse](t, recorder)
	assertAppIDs(t, body.Apps, 100, 200, 300, 500, 400)

	wantNames := []string{"Alpha", "Alpha", "Beta", "Gamma", "<nil>"}
	for index, want := range wantNames {
		if got := appName(body.Apps[index]); got != want {
			t.Fatalf("app %d name = %q, want %q", index, got, want)
		}
	}
}

func TestGetSteamUserViewCountsDeduplicateAliasesAndSkipRemoved(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	const steamID = int64(2001)
	seedSteamUser(t, env, steamID, "DedupeUser")

	seedApp(t, env, 80, "Canonical One")
	seedAlias(t, env, 80, 80)
	seedAlias(t, env, 80, 100)
	seedAlias(t, env, 80, 101)
	seedApp(t, env, 90, "Canonical Two")
	seedAlias(t, env, 90, 90)
	seedAlias(t, env, 90, 91)
	seedApp(t, env, 95, "Removed Only")
	seedAlias(t, env, 95, 95)

	seedOwnedApp(t, env, steamID, 80, false)
	seedOwnedApp(t, env, steamID, 100, false)
	seedOwnedApp(t, env, steamID, 101, true)
	seedOwnedApp(t, env, steamID, 90, false)
	seedOwnedApp(t, env, steamID, 95, true)

	seedWishlistApp(t, env, steamID, 80, false)
	seedWishlistApp(t, env, steamID, 100, false)
	seedWishlistApp(t, env, steamID, 90, false)
	seedWishlistApp(t, env, steamID, 91, true)

	for _, test := range []struct {
		list    string
		wantIDs []int32
	}{
		{list: appListOwned, wantIDs: []int32{80, 90}},
		{list: appListWishlist, wantIDs: []int32{80, 90}},
	} {
		t.Run(test.list, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name=dedupeuser&list="+test.list, "")
			requireStatus(t, recorder, http.StatusOK)
			body := decodeJSON[steamUserViewResponse](t, recorder)
			if body.OwnedAppCount != 2 || body.WishlistAppCount != 2 {
				t.Fatalf("counts = %d owned, %d wishlist; want 2 and 2", body.OwnedAppCount, body.WishlistAppCount)
			}
			assertAppIDs(t, body.Apps, test.wantIDs...)
		})
	}
}

func TestGetSteamUserViewLatestImportAndActiveJob(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	now := time.Now()

	seedSteamUser(t, env, 2101, "ActiveView")
	seedImportJob(t, env, 2101, importer.StatusSucceeded, now.Add(-2*time.Hour))
	seedImportJob(t, env, 2101, importer.StatusRunning, now.Add(-time.Hour))

	seedSteamUser(t, env, 2102, "IdleView")
	seedImportJob(t, env, 2102, importer.StatusSucceeded, now.Add(-time.Hour))

	tests := []struct {
		name       string
		query      string
		wantStatus string
		wantActive bool
	}{
		{name: "active", query: "activeview", wantStatus: importer.StatusRunning, wantActive: true},
		{name: "idle", query: "idleview", wantStatus: importer.StatusSucceeded, wantActive: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, "/api/steamusers/by-name?name="+test.query, "")
			requireStatus(t, recorder, http.StatusOK)
			body := decodeJSON[steamUserViewResponse](t, recorder)
			if body.LatestImport == nil {
				t.Fatal("latestImport = nil")
			}
			if body.LatestImport.Status != test.wantStatus {
				t.Fatalf("latest status = %q, want %q", body.LatestImport.Status, test.wantStatus)
			}
			if body.HasActiveJob != test.wantActive {
				t.Fatalf("hasActiveJob = %v, want %v", body.HasActiveJob, test.wantActive)
			}
		})
	}
}

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
)

const (
	commonGamesAlice = int64(76561198000000101)
	commonGamesBob   = int64(76561198000000102)
	commonGamesCarol = int64(76561198000000103)
)

type commonGameSeed struct {
	AppID                int32
	Name                 string
	HeaderImage          string
	ShortDescription     string
	Website              string
	PriceCurrency        string
	PriceInitial         sql.NullInt32
	PriceFinal           sql.NullInt32
	PriceDiscountPercent sql.NullInt32
	PriceFinalFormatted  string
	PlatformWindows      sql.NullBool
	PlatformMac          sql.NullBool
	PlatformLinux        sql.NullBool
	ReleaseDate          string
	ReleaseDateParsed    sql.NullTime
	ComingSoon           bool
	Categories           []int32
}

func seedCommonGame(t *testing.T, env *handlerEnv, seed commonGameSeed) {
	t.Helper()
	ctx := context.Background()
	seedApp(t, env, seed.AppID, seed.Name)
	seedAlias(t, env, seed.AppID, seed.AppID)

	if _, err := env.db.ExecContext(ctx, `
		UPDATE apps SET
			header_image = $1,
			short_description = $2,
			website = $3,
			price_currency = $4,
			price_initial = $5,
			price_final = $6,
			price_discount_percent = $7,
			price_final_formatted = $8,
			platform_windows = $9,
			platform_mac = $10,
			platform_linux = $11,
			release_date = $12,
			release_date_parsed = $13,
			release_coming_soon = $14
		WHERE steam_appid = $15`,
		sql.NullString{String: seed.HeaderImage, Valid: seed.HeaderImage != ""},
		sql.NullString{String: seed.ShortDescription, Valid: seed.ShortDescription != ""},
		sql.NullString{String: seed.Website, Valid: seed.Website != ""},
		sql.NullString{String: seed.PriceCurrency, Valid: seed.PriceCurrency != ""},
		seed.PriceInitial,
		seed.PriceFinal,
		seed.PriceDiscountPercent,
		sql.NullString{String: seed.PriceFinalFormatted, Valid: seed.PriceFinalFormatted != ""},
		seed.PlatformWindows,
		seed.PlatformMac,
		seed.PlatformLinux,
		sql.NullString{String: seed.ReleaseDate, Valid: seed.ReleaseDate != ""},
		seed.ReleaseDateParsed,
		seed.ComingSoon,
		seed.AppID,
	); err != nil {
		t.Fatalf("set release data for app %d: %v", seed.AppID, err)
	}

	for _, categoryID := range seed.Categories {
		if _, err := env.queries.UpsertCategory(ctx, dbgen.UpsertCategoryParams{
			ID:          categoryID,
			Description: sql.NullString{String: fmt.Sprintf("Category %d", categoryID), Valid: true},
		}); err != nil {
			t.Fatalf("upsert category %d: %v", categoryID, err)
		}
		if err := env.queries.AddAppCategory(ctx, dbgen.AddAppCategoryParams{
			AppID:      seed.AppID,
			CategoryID: categoryID,
		}); err != nil {
			t.Fatalf("add category %d to app %d: %v", categoryID, seed.AppID, err)
		}
	}
}

func seedReleasedCommonGame(t *testing.T, env *handlerEnv, appID int32, name string, release time.Time, categories ...int32) {
	t.Helper()
	seedCommonGame(t, env, commonGameSeed{
		AppID:             appID,
		Name:              name,
		ReleaseDate:       release.Format("2 Jan, 2006"),
		ReleaseDateParsed: sql.NullTime{Time: release, Valid: true},
		Categories:        categories,
	})
}

func seedImportedCommonUser(t *testing.T, env *handlerEnv, steamID int64) {
	t.Helper()
	seedSteamUser(t, env, steamID, fmt.Sprintf("user-%d", steamID))
	seedImportJob(t, env, steamID, importer.StatusSucceeded, time.Now())
}

func seedOwnedFor(t *testing.T, env *handlerEnv, appID int32, steamIDs ...int64) {
	t.Helper()
	for _, steamID := range steamIDs {
		seedOwnedApp(t, env, steamID, appID, false)
	}
}

func seedWishlistedFor(t *testing.T, env *handlerEnv, appID int32, steamIDs ...int64) {
	t.Helper()
	for _, steamID := range steamIDs {
		seedWishlistApp(t, env, steamID, appID, false)
	}
}

func commonGamesLimit(value int) *int {
	return &value
}

func postCommonGames(t *testing.T, env *handlerEnv, limit *int, steamIDs ...int64) *httptest.ResponseRecorder {
	t.Helper()
	ids := make([]string, len(steamIDs))
	for index, steamID := range steamIDs {
		ids[index] = strconv.FormatInt(steamID, 10)
	}
	body, err := json.Marshal(commonGamesRequest{SteamIDs: ids, Limit: limit})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return env.do(t, http.MethodPost, "/api/common-games", string(body))
}

func requireCommonGames(t *testing.T, recorder *httptest.ResponseRecorder) commonGamesResponse {
	t.Helper()
	requireStatus(t, recorder, http.StatusOK)
	return decodeJSON[commonGamesResponse](t, recorder)
}

func assertCommonGameIDs(t *testing.T, games []commonGameView, want ...int32) {
	t.Helper()
	got := make([]int32, len(games))
	for index, game := range games {
		got[index] = game.AppID
	}
	if len(got) != len(want) {
		t.Fatalf("game IDs = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("game IDs = %v, want %v", got, want)
		}
	}
}

func assertCommonGameCounts(t *testing.T, game commonGameView, owned, wishlist, total int64) {
	t.Helper()
	if game.OwnedCount != owned || game.WishlistCount != wishlist || game.TotalCommonUsers != total {
		t.Fatalf(
			"app %d counts = owned %d, wishlist %d, total %d; want %d, %d, %d",
			game.AppID, game.OwnedCount, game.WishlistCount, game.TotalCommonUsers,
			owned, wishlist, total,
		)
	}
}

func TestCommonGamesHappyPath(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedImportedCommonUser(t, env, commonGamesBob)

	seedCommonGame(t, env, commonGameSeed{
		AppID:                730,
		Name:                 "Counter-Strike 2",
		HeaderImage:          "https://cdn.example.test/steam/apps/730/header.jpg",
		ShortDescription:     "A tactical multiplayer shooter.",
		Website:              "https://www.counter-strike.net/",
		PriceCurrency:        "USD",
		PriceInitial:         sql.NullInt32{Int32: 1999, Valid: true},
		PriceFinal:           sql.NullInt32{Int32: 999, Valid: true},
		PriceDiscountPercent: sql.NullInt32{Int32: 50, Valid: true},
		PriceFinalFormatted:  "$9.99",
		PlatformWindows:      sql.NullBool{Bool: true, Valid: true},
		PlatformMac:          sql.NullBool{Bool: false, Valid: true},
		PlatformLinux:        sql.NullBool{Bool: true, Valid: true},
		ReleaseDate:          "21 Nov, 2024",
		ReleaseDateParsed:    sql.NullTime{Time: time.Date(2024, time.November, 21, 0, 0, 0, 0, time.UTC), Valid: true},
		Categories:           []int32{1},
	})
	seedCommonGame(t, env, commonGameSeed{
		AppID:             570,
		Name:              "Dota 2",
		ReleaseDate:       "9 Jul, 2013",
		ReleaseDateParsed: sql.NullTime{Time: time.Date(2013, time.July, 9, 0, 0, 0, 0, time.UTC), Valid: true},
		Categories:        []int32{1},
	})
	seedOwnedFor(t, env, 730, commonGamesAlice, commonGamesBob)
	seedWishlistedFor(t, env, 570, commonGamesAlice, commonGamesBob)

	games := requireCommonGames(t, postCommonGames(t, env, nil, commonGamesAlice, commonGamesBob))
	assertCommonGameIDs(t, games.Games, 730, 570)

	shared := games.Games[0]
	if shared.Name == nil || *shared.Name != "Counter-Strike 2" {
		t.Fatalf("name = %v, want Counter-Strike 2", shared.Name)
	}
	if shared.HeaderImage == nil || *shared.HeaderImage != "https://cdn.example.test/steam/apps/730/header.jpg" {
		t.Fatalf("headerImage = %v", shared.HeaderImage)
	}
	if shared.ReleaseDate == nil || *shared.ReleaseDate != "21 Nov, 2024" {
		t.Fatalf("releaseDate = %v, want 21 Nov, 2024", shared.ReleaseDate)
	}
	if shared.ShortDescription == nil || *shared.ShortDescription != "A tactical multiplayer shooter." {
		t.Fatalf("shortDescription = %v", shared.ShortDescription)
	}
	if shared.Website == nil || *shared.Website != "https://www.counter-strike.net/" {
		t.Fatalf("website = %v", shared.Website)
	}
	if shared.PriceCurrency == nil || *shared.PriceCurrency != "USD" {
		t.Fatalf("priceCurrency = %v", shared.PriceCurrency)
	}
	if shared.PriceInitial == nil || *shared.PriceInitial != 1999 {
		t.Fatalf("priceInitial = %v", shared.PriceInitial)
	}
	if shared.PriceFinal == nil || *shared.PriceFinal != 999 {
		t.Fatalf("priceFinal = %v", shared.PriceFinal)
	}
	if shared.PriceDiscountPercent == nil || *shared.PriceDiscountPercent != 50 {
		t.Fatalf("priceDiscountPercent = %v", shared.PriceDiscountPercent)
	}
	if shared.PriceFinalFormatted == nil || *shared.PriceFinalFormatted != "$9.99" {
		t.Fatalf("priceFinalFormatted = %v", shared.PriceFinalFormatted)
	}
	if shared.PlatformWindows == nil || !*shared.PlatformWindows {
		t.Fatalf("platformWindows = %v", shared.PlatformWindows)
	}
	if shared.PlatformMac == nil || *shared.PlatformMac {
		t.Fatalf("platformMac = %v", shared.PlatformMac)
	}
	if shared.PlatformLinux == nil || !*shared.PlatformLinux {
		t.Fatalf("platformLinux = %v", shared.PlatformLinux)
	}
	assertCommonGameCounts(t, shared, 2, 0, 2)
	assertCommonGameCounts(t, games.Games[1], 0, 2, 2)
}

func TestCommonGamesCategoryFilter(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	release := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)

	seedReleasedCommonGame(t, env, 10, "Multiplayer", release, 1)
	seedReleasedCommonGame(t, env, 20, "Single-player", release, 2)
	seedReleasedCommonGame(t, env, 30, "Uncategorized", release)
	seedReleasedCommonGame(t, env, 40, "Multiplayer and single-player", release, 1, 2)
	for _, appID := range []int32{10, 20, 30, 40} {
		seedOwnedFor(t, env, appID, commonGamesAlice)
	}

	games := requireCommonGames(t, postCommonGames(t, env, nil, commonGamesAlice))
	assertCommonGameIDs(t, games.Games, 10, 40)
}

func TestCommonGamesRanking(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedImportedCommonUser(t, env, commonGamesBob)
	seedImportedCommonUser(t, env, commonGamesCarol)

	type rankingGame struct {
		appID int32
		name  string
		date  time.Time
	}
	rankingGames := []rankingGame{
		{100, "A", time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{110, "D", time.Date(2014, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{120, "B", time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{130, "E", time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{140, "C", time.Date(2017, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{150, "G", time.Date(2016, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{160, "F", time.Date(2015, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{940, "Tie low ID", time.Date(2012, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{950, "Tie high ID", time.Date(2012, time.January, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, game := range rankingGames {
		seedReleasedCommonGame(t, env, game.appID, game.name, game.date, 1)
	}

	seedOwnedFor(t, env, 100, commonGamesAlice, commonGamesBob, commonGamesCarol)

	seedOwnedFor(t, env, 110, commonGamesAlice, commonGamesBob)
	seedWishlistedFor(t, env, 110, commonGamesAlice, commonGamesCarol)

	for _, appID := range []int32{120, 130, 140} {
		seedOwnedFor(t, env, appID, commonGamesAlice, commonGamesBob)
		seedWishlistedFor(t, env, appID, commonGamesCarol)
	}

	seedOwnedFor(t, env, 150, commonGamesAlice)
	seedWishlistedFor(t, env, 150, commonGamesBob, commonGamesCarol)

	seedOwnedFor(t, env, 160, commonGamesAlice, commonGamesBob)
	seedOwnedFor(t, env, 940, commonGamesAlice)
	seedOwnedFor(t, env, 950, commonGamesAlice)

	response := requireCommonGames(t, postCommonGames(t, env, commonGamesLimit(100), commonGamesAlice, commonGamesBob, commonGamesCarol))
	assertCommonGameIDs(t, response.Games, 100, 110, 120, 130, 140, 150, 160, 940, 950)

	byID := make(map[int32]commonGameView, len(response.Games))
	for _, game := range response.Games {
		byID[game.AppID] = game
	}
	assertCommonGameCounts(t, byID[100], 3, 0, 3)
	assertCommonGameCounts(t, byID[110], 2, 2, 3)
	assertCommonGameCounts(t, byID[120], 2, 1, 3)
	assertCommonGameCounts(t, byID[150], 1, 2, 3)
	assertCommonGameCounts(t, byID[160], 2, 0, 2)
}

func TestCommonGamesReleaseEligibility(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedImportedCommonUser(t, env, commonGamesBob)
	release := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)

	seedReleasedCommonGame(t, env, 100, "Released", release, 1)
	seedCommonGame(t, env, commonGameSeed{
		AppID:             200,
		Name:              "Coming soon",
		ReleaseDate:       "1 Jan, 2010",
		ReleaseDateParsed: sql.NullTime{Time: release, Valid: true},
		ComingSoon:        true,
		Categories:        []int32{1},
	})
	seedCommonGame(t, env, commonGameSeed{
		AppID:             300,
		Name:              "Future",
		ReleaseDate:       "1 Jan, 2999",
		ReleaseDateParsed: sql.NullTime{Time: time.Now().AddDate(1, 0, 0), Valid: true},
		Categories:        []int32{1},
	})
	seedCommonGame(t, env, commonGameSeed{
		AppID:       400,
		Name:        "Unknown release date",
		ReleaseDate: "Coming Soon",
		Categories:  []int32{1},
	})
	for _, appID := range []int32{100, 200, 300, 400} {
		seedOwnedFor(t, env, appID, commonGamesAlice, commonGamesBob)
	}

	games := requireCommonGames(t, postCommonGames(t, env, nil, commonGamesAlice, commonGamesBob))
	assertCommonGameIDs(t, games.Games, 100)
}

func TestCommonGamesPartialCommonality(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedImportedCommonUser(t, env, commonGamesBob)
	release := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)

	seedReleasedCommonGame(t, env, 100, "Shared", release, 1)
	seedReleasedCommonGame(t, env, 200, "Alice only", release, 1)
	seedOwnedFor(t, env, 100, commonGamesAlice, commonGamesBob)
	seedOwnedFor(t, env, 200, commonGamesAlice)

	games := requireCommonGames(t, postCommonGames(t, env, commonGamesLimit(10), commonGamesAlice, commonGamesBob))
	assertCommonGameIDs(t, games.Games, 100, 200)
	assertCommonGameCounts(t, games.Games[0], 2, 0, 2)
	assertCommonGameCounts(t, games.Games[1], 1, 0, 1)
}

func TestCommonGamesOwnedAndWishlistOverlap(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedReleasedCommonGame(t, env, 100, "Overlap", time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC), 1)

	seedOwnedFor(t, env, 100, commonGamesAlice)
	seedWishlistedFor(t, env, 100, commonGamesAlice)

	games := requireCommonGames(t, postCommonGames(t, env, nil, commonGamesAlice))
	assertCommonGameIDs(t, games.Games, 100)
	assertCommonGameCounts(t, games.Games[0], 1, 1, 1)
}

func TestCommonGamesAliasDedupe(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedReleasedCommonGame(t, env, 80, "Canonical", time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC), 1)
	seedAlias(t, env, 80, 100)

	seedOwnedFor(t, env, 80, commonGamesAlice)
	seedOwnedFor(t, env, 100, commonGamesAlice)
	seedWishlistedFor(t, env, 80, commonGamesAlice)
	seedWishlistedFor(t, env, 100, commonGamesAlice)

	games := requireCommonGames(t, postCommonGames(t, env, nil, commonGamesAlice))
	assertCommonGameIDs(t, games.Games, 80)
	assertCommonGameCounts(t, games.Games[0], 1, 1, 1)
}

func TestCommonGamesRemovedRows(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	release := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, appID := range []int32{100, 200, 300, 400} {
		seedReleasedCommonGame(t, env, appID, fmt.Sprintf("App %d", appID), release, 1)
	}

	seedOwnedApp(t, env, commonGamesAlice, 100, true)
	seedWishlistApp(t, env, commonGamesAlice, 200, true)
	seedOwnedApp(t, env, commonGamesAlice, 300, false)
	seedWishlistApp(t, env, commonGamesAlice, 400, false)

	games := requireCommonGames(t, postCommonGames(t, env, nil, commonGamesAlice))
	assertCommonGameIDs(t, games.Games, 300, 400)
	assertCommonGameCounts(t, games.Games[0], 1, 0, 1)
	assertCommonGameCounts(t, games.Games[1], 0, 1, 1)
}

func TestCommonGamesLimit(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	release := time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)
	for index := range 12 {
		appID := int32(1000 + index)
		seedReleasedCommonGame(t, env, appID, fmt.Sprintf("Game %d", appID), release, 1)
		seedOwnedFor(t, env, appID, commonGamesAlice)
	}

	defaultGames := requireCommonGames(t, postCommonGames(t, env, nil, commonGamesAlice))
	assertCommonGameIDs(t, defaultGames.Games, 1000, 1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009)

	omittedBody := fmt.Sprintf(`{"steamIds":[%q]}`, strconv.FormatInt(commonGamesAlice, 10))
	omitted := requireCommonGames(t, env.do(t, http.MethodPost, "/api/common-games", omittedBody))
	assertCommonGameIDs(t, omitted.Games, 1000, 1001, 1002, 1003, 1004, 1005, 1006, 1007, 1008, 1009)

	limited := requireCommonGames(t, postCommonGames(t, env, commonGamesLimit(3), commonGamesAlice))
	assertCommonGameIDs(t, limited.Games, 1000, 1001, 1002)

	steamID := strconv.FormatInt(commonGamesAlice, 10)
	for _, limit := range []string{"0", "101", "1.5", `"nope"`} {
		t.Run(limit, func(t *testing.T) {
			body := fmt.Sprintf(`{"steamIds":[%q],"limit":%s}`, steamID, limit)
			requireStatus(t, env.do(t, http.MethodPost, "/api/common-games", body), http.StatusBadRequest)
		})
	}
}

func TestCommonGamesInputValidation(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())

	tooManyIDs := make([]string, maxCommonGameUsers+1)
	for index := range tooManyIDs {
		tooManyIDs[index] = strconv.FormatInt(commonGamesAlice+int64(index), 10)
	}
	tooManyBody, err := json.Marshal(commonGamesRequest{SteamIDs: tooManyIDs})
	if err != nil {
		t.Fatalf("marshal too-many request: %v", err)
	}

	tests := []struct {
		name string
		body string
	}{
		{"invalid JSON", `{`},
		{"missing steamIds", `{}`},
		{"null steamIds", `{"steamIds":null}`},
		{"empty steamIds", `{"steamIds":[]}`},
		{"blank ID", `{"steamIds":["   "]}`},
		{"non-numeric ID", `{"steamIds":["not-a-number"]}`},
		{"decimal ID", `{"steamIds":["1.5"]}`},
		{"zero ID", `{"steamIds":["0"]}`},
		{"negative ID", `{"steamIds":["-1"]}`},
		{"too many unique IDs", string(tooManyBody)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireStatus(t, env.do(t, http.MethodPost, "/api/common-games", test.body), http.StatusBadRequest)
		})
	}
}

func TestCommonGamesDeduplicatesInputIDs(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedImportedCommonUser(t, env, commonGamesBob)
	seedReleasedCommonGame(t, env, 100, "Shared", time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC), 1)
	seedOwnedFor(t, env, 100, commonGamesAlice, commonGamesBob)

	duplicates := requireCommonGames(t, postCommonGames(t, env, nil,
		commonGamesAlice, commonGamesBob, commonGamesAlice, commonGamesBob,
	))
	unique := requireCommonGames(t, postCommonGames(t, env, nil, commonGamesAlice, commonGamesBob))

	assertCommonGameIDs(t, duplicates.Games, 100)
	assertCommonGameIDs(t, unique.Games, 100)
	assertCommonGameCounts(t, duplicates.Games[0], 2, 0, 2)
	assertCommonGameCounts(t, unique.Games[0], 2, 0, 2)
}

func TestCommonGamesUnknownUser(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	unknown := commonGamesAlice + 1000

	recorder := postCommonGames(t, env, nil, unknown, commonGamesAlice)
	requireStatus(t, recorder, http.StatusNotFound)
	if want := fmt.Sprintf("steam user %d not found", unknown); !strings.Contains(recorder.Body.String(), want) {
		t.Fatalf("body = %s, want %q", recorder.Body.String(), want)
	}
}

func TestCommonGamesUserWithoutCompletedImport(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, commonGamesAlice, "No completed import")
	now := time.Now()
	seedImportJob(t, env, commonGamesAlice, importer.StatusQueued, now.Add(-3*time.Hour))
	seedImportJob(t, env, commonGamesAlice, importer.StatusRunning, now.Add(-2*time.Hour))
	seedImportJob(t, env, commonGamesAlice, importer.StatusFailed, now.Add(-time.Hour))

	recorder := postCommonGames(t, env, nil, commonGamesAlice)
	requireStatus(t, recorder, http.StatusNotFound)
	if want := fmt.Sprintf("steam user %d has no completed import", commonGamesAlice); !strings.Contains(recorder.Body.String(), want) {
		t.Fatalf("body = %s, want %q", recorder.Body.String(), want)
	}
}

func TestCommonGamesEmptyLibrary(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)

	recorder := postCommonGames(t, env, nil, commonGamesAlice)
	games := requireCommonGames(t, recorder)
	if len(games.Games) != 0 {
		t.Fatalf("games = %+v, want none", games.Games)
	}
	if !strings.Contains(recorder.Body.String(), `"games":[]`) {
		t.Fatalf("empty games should be an array, got %s", recorder.Body.String())
	}
}

func TestCommonGamesNoMatches(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedImportedCommonUser(t, env, commonGamesBob)
	seedReleasedCommonGame(t, env, 100, "Not multiplayer", time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC), 2)
	seedOwnedFor(t, env, 100, commonGamesAlice, commonGamesBob)

	recorder := postCommonGames(t, env, nil, commonGamesAlice, commonGamesBob)
	games := requireCommonGames(t, recorder)
	if len(games.Games) != 0 {
		t.Fatalf("games = %+v, want none", games.Games)
	}
	if !strings.Contains(recorder.Body.String(), `"games":[]`) {
		t.Fatalf("empty games should be an array, got %s", recorder.Body.String())
	}
}

func TestCommonGamesNullableFields(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedImportedCommonUser(t, env, commonGamesAlice)
	seedCommonGame(t, env, commonGameSeed{
		AppID:             100,
		ReleaseDate:       "21 Nov, 2024",
		ReleaseDateParsed: sql.NullTime{Time: time.Date(2024, time.November, 21, 0, 0, 0, 0, time.UTC), Valid: true},
		Categories:        []int32{1},
	})
	seedOwnedFor(t, env, 100, commonGamesAlice)

	recorder := postCommonGames(t, env, nil, commonGamesAlice)
	requireStatus(t, recorder, http.StatusOK)

	var body struct {
		Games []map[string]json.RawMessage `json:"games"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	if len(body.Games) != 1 {
		t.Fatalf("games = %+v, want one", body.Games)
	}
	game := body.Games[0]
	if got := string(game["name"]); got != "null" {
		t.Fatalf("name = %s, want null", got)
	}
	if got := string(game["headerImage"]); got != "null" {
		t.Fatalf("headerImage = %s, want null", got)
	}
	var releaseDate string
	if err := json.Unmarshal(game["releaseDate"], &releaseDate); err != nil {
		t.Fatalf("releaseDate = %s: %v", game["releaseDate"], err)
	}
	if releaseDate != "21 Nov, 2024" {
		t.Fatalf("releaseDate = %q, want 21 Nov, 2024", releaseDate)
	}
}

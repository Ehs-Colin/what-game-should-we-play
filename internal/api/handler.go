package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
)

type insertSteamUserRequest struct {
	VanityUrl string `json:"vanityUrl"`
}

type Handler struct {
	apiStore *Store
	//steamClient *steamapi.Client
	//logger  *logging.Logger
}

func NewHandler(apiStore *Store) *Handler {
	return &Handler{
		apiStore: apiStore,
	}
}

func (handler *Handler) InsertSteamUser(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO::Require user login with write permissions
	// TODO::Check if user already exists
	var body insertSteamUserRequest
	request.Body = http.MaxBytesReader(responseWriter, request.Body, 1<<10)
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "invalid JSON body")
		return
	}
	vanityUrl := strings.TrimSpace(body.VanityUrl)
	if vanityUrl == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "vanityUrl is required")
		return
	}

	//fmt.Printf("Resolving Vanity URL: %v\n", vanityUrl)
	resolved, err := handler.apiStore.steamClient.ResolveVanityURL(request.Context(), vanityUrl)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	// Steam returns success == 1 when the vanity URL resolves to a user.
	if resolved.Response.Success != 1 {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
		return
	}

	//fmt.Printf("Resolved Steam ID: %v\n", resolved.Response.SteamID)
	steamID, err := strconv.ParseInt(resolved.Response.SteamID, 10, 64)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	//fmt.Printf("Creating Steam User %s with Steam ID: %v\n", vanityUrl, steamID)
	steamUser, err := handler.apiStore.queries.UpsertSteamUser(request.Context(), dbgen.UpsertSteamUserParams{
		SteamID: steamID,
		Name:    vanityUrl,
	})
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	//fmt.Printf("Getting owned games for Steam ID: %v\n", steamID)
	userApps, err := handler.apiStore.steamClient.GetOwnedGames(steamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	err = handler.addApps(request, *userApps, steamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	httpx.RespondWithJSON(responseWriter, http.StatusCreated, steamUser)
}

func (handler *Handler) addApps(request *http.Request, apps steamapi.OwnedGamesResponse, steamID int64) error {
	// Upsert every app before adding categories and genres, which reference the app rows.
	upsertedApps := make(map[int]*steamapi.AppDetail, len(apps.Response.Games))
	for _, app := range apps.Response.Games {
		//TODO::Check database for existing app and last time it was updated.  Update only if necessary.
		appID := app.AppID
		steamApp, err := handler.apiStore.steamClient.GetAppDetail(request.Context(), appID)
		if errors.Is(err, steamapi.ErrAppNotFound) {
			log.Printf("Skipping app %d: missing or removed from Steam", appID)
			continue
		}
		if err != nil {
			return fmt.Errorf("Failed to get App Detail From Steam: %v", err)
		}

		// Free apps have no price_overview; store them as a zero price.
		priceCurrency := "USD"
		priceInitial, priceFinal, priceDiscountPercent := 0, 0, 0
		priceFinalFormatted := "$0.00"
		if price := steamApp.Data.PriceOverview; price != nil {
			priceCurrency = price.Currency
			priceInitial = price.Initial
			priceFinal = price.Final
			priceDiscountPercent = price.DiscountPercent
			priceFinalFormatted = price.FinalFormatted
		}
		_, err = handler.apiStore.queries.UpsertApp(request.Context(), dbgen.UpsertAppParams{
			SteamAppid:           int32(steamApp.Data.AppID),
			Name:                 nullString(steamApp.Data.Name),
			DetailedDescription:  nullString(steamApp.Data.DetailedDescription),
			AboutTheGame:         nullString(steamApp.Data.AboutTheGame),
			ShortDescription:     nullString(steamApp.Data.ShortDescription),
			HeaderImage:          nullString(steamApp.Data.HeaderImage),
			Website:              nullString(steamApp.Data.Website),
			PriceCurrency:        sql.NullString{String: priceCurrency, Valid: true},
			PriceInitial:         sql.NullInt32{Int32: int32(priceInitial), Valid: true},
			PriceFinal:           sql.NullInt32{Int32: int32(priceFinal), Valid: true},
			PriceDiscountPercent: sql.NullInt32{Int32: int32(priceDiscountPercent), Valid: true},
			PriceFinalFormatted:  sql.NullString{String: priceFinalFormatted, Valid: true},
			PlatformWindows:      sql.NullBool{Bool: steamApp.Data.Platforms.Windows, Valid: true},
			PlatformMac:          sql.NullBool{Bool: steamApp.Data.Platforms.Mac, Valid: true},
			PlatformLinux:        sql.NullBool{Bool: steamApp.Data.Platforms.Linux, Valid: true},
			ReleaseDate:          nullString(steamApp.Data.ReleaseDate.Date),
			ReleaseComingSoon:    sql.NullBool{Bool: steamApp.Data.ReleaseDate.ComingSoon, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("Failed to Insert/Update AppDetail(%d): %v", appID, err)
		}
		canonicalID := steamApp.Data.AppID
		if canonicalID != appID {
			log.Printf("App %d resolves to app %d on Steam", appID, canonicalID)
			_, err = handler.apiStore.queries.UpsertAppAlias(request.Context(), dbgen.UpsertAppAliasParams{
				AliasID: int32(appID),
				AppID:   int32(canonicalID),
			})
			if err != nil {
				return fmt.Errorf("Failed to Add App Alias(%d -> %d): %v", appID, canonicalID, err)
			}
		}
		upsertedApps[canonicalID] = steamApp
	}

	//Add the user's ownership, categories, and genres to the database
	for appID, steamApp := range upsertedApps {
		err := handler.apiStore.queries.AddSteamUserApp(request.Context(), dbgen.AddSteamUserAppParams{
			SteamID: steamID,
			AppID:   int32(appID),
		})
		if err != nil {
			return fmt.Errorf("Failed to Add Steam User(%d) App(%d): %v", steamID, appID, err)
		}
		err = handler.addAppCategories(request, steamApp.Data.Categories, appID)
		if err != nil {
			return fmt.Errorf("Failed to Add App Categories: %v", err)
		}
		err = handler.addAppGenres(request, steamApp.Data.Genres, appID)
		if err != nil {
			return fmt.Errorf("Failed to Add App Genres: %v", err)
		}
	}
	return nil
}

func (handler *Handler) addAppCategories(request *http.Request, categories []steamapi.Category, appId int) error {
	for _, category := range categories {
		//Insert category into the database
		_, err := handler.apiStore.queries.UpsertCategory(request.Context(), dbgen.UpsertCategoryParams{
			ID: int32(category.ID),
			Description: sql.NullString{
				String: category.Description,
				Valid:  true,
			},
		})
		if err != nil {
			return fmt.Errorf("Failed to Upsert Category: %v", err)
		}
		//Add category to the app
		err = handler.apiStore.queries.AddAppCategory(request.Context(), dbgen.AddAppCategoryParams{
			CategoryID: int32(category.ID),
			AppID:      int32(appId),
		})
		if err != nil {
			return fmt.Errorf("Failed to Add App(%d) Category(%d): %v", appId, category.ID, err)
		}
	}
	return nil
}

func (handler *Handler) addAppGenres(request *http.Request, genres []steamapi.Genre, appId int) error {
	for _, genre := range genres {
		//Insert genre into the database
		genreId, err := strconv.Atoi(genre.ID)
		if err != nil {
			return fmt.Errorf("Failed to convert genre ID to int: %v", err)
		}
		_, err = handler.apiStore.queries.UpsertGenre(request.Context(), dbgen.UpsertGenreParams{
			ID: int32(genreId),
			Description: sql.NullString{
				String: genre.Description,
				Valid:  true,
			},
		})
		if err != nil {
			return fmt.Errorf("Failed to Upsert Genre: %v", err)
		}
		//Add genre to the app

		err = handler.apiStore.queries.AddAppGenre(request.Context(), dbgen.AddAppGenreParams{
			GenreID: int32(genreId),
			AppID:   int32(appId),
		})
		if err != nil {
			return fmt.Errorf("Failed to Add App Genre: %v", err)
		}
	}
	return nil
}

func (handler *Handler) GetSteamUserById(responseWriter http.ResponseWriter, request *http.Request) {

}

func (handler *Handler) GetSteamUserByName(responseWriter http.ResponseWriter, request *http.Request) {

}

func (handler *Handler) GetSteamUsers(responseWriter http.ResponseWriter, request *http.Request) {

}

func (handler *Handler) internalError(responseWriter http.ResponseWriter, _ *http.Request, err error) {
	//TODO::LOGGER
	//_ = handler.logger.Event("unhandled_error", map[string]any{"method": request.Method, "path": request.URL.Path, "message": err.Error()})
	httpx.RespondWithError(responseWriter, http.StatusInternalServerError, err.Error())
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

package api

import (
	"database/sql"
	"encoding/json"
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
	apiStore    *Store
	steamClient *steamapi.Client
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

	steamID, err := strconv.ParseInt(resolved.Response.SteamID, 10, 64)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	//TODO::Parse apps and things
	userApps, err := handler.steamClient.GetOwnedGames(steamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	err = handler.addApps(userApps)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	steamUser, err := handler.apiStore.queries.CreateSteamUser(request.Context(), dbgen.CreateSteamUserParams{
		SteamID: steamID,
		Name:    vanityUrl,
	})
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	httpx.RespondWithJSON(responseWriter, http.StatusCreated, steamUser)
}

func (handler *Handler) addApps(request *http.Request, apps steamapi.OwnedGamesResponse) error {
	for _, app := range apps.Response.Games {
		//TODO::Check database for existing app and last time it was updated.  Update only if necessary.

		// Add App to the database
		// TODO::ADD APP TO DATABASE
		steamApp, err := handler.steamClient.GetAppDetail(app.AppID)
		if err != nil {
			return err
		}
		//Add categories and genres to the database
		err = handler.addAppCategories(request, steamApp.Data.Categories, app.AppID)
		if err != nil {
			return err
		}
		err = handler.addAppGenres(request, steamApp.Data.Genres)
		if err != nil {
			return err
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
			return err
		}
		//Add category to the app
		err = handler.apiStore.queries.AddAppCategory(request.Context(), dbgen.AddAppCategoryParams{
			CategoryID: int32(category.ID),
			AppID:      int32(appId),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (handler *Handler) addAppGenres(request *http.Request, genres []steamapi.Genre) error {
	for _, genre := range genres {
		//Insert genre into the database
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

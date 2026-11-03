package api

import (
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
	//TODO::Require user login with write permissions
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

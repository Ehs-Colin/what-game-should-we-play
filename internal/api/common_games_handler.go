package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
)

const (
	defaultCommonGamesLimit = 10
	maxCommonGamesLimit     = 100
	maxCommonGameUsers      = 25
	maxCommonGamesBodyBytes = 4 << 10
)

type commonGamesRequest struct {
	SteamIDs []string `json:"steamIds"`
	Limit    *int     `json:"limit"`
}

type commonGamesResponse struct {
	Games []commonGameView `json:"games"`
}

// GetCommonGames returns multiplayer games owned or wishlisted by the
// requested users, ranked by how many of those users have each game.
func (handler *Handler) GetCommonGames(responseWriter http.ResponseWriter, request *http.Request) {
	var body commonGamesRequest
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxCommonGamesBodyBytes)
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "invalid JSON body")
		return
	}

	limit := defaultCommonGamesLimit
	if body.Limit != nil {
		limit = *body.Limit
	}
	if limit < 1 || limit > maxCommonGamesLimit {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "limit must be between 1 and 100")
		return
	}

	steamIDs, err := parseSteamIDs(body.SteamIDs)
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		return
	}

	importedUsers, err := handler.apiStore.queries.ListImportedSteamUsers(request.Context(), steamIDs)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	importStatus := make(map[int64]bool, len(importedUsers))
	for _, user := range importedUsers {
		importStatus[user.SteamID] = user.HasSucceededImport
	}
	for _, steamID := range steamIDs {
		hasSucceededImport, found := importStatus[steamID]
		if !found {
			httpx.RespondWithError(responseWriter, http.StatusNotFound, fmt.Sprintf("steam user %d not found", steamID))
			return
		}
		if !hasSucceededImport {
			httpx.RespondWithError(responseWriter, http.StatusNotFound, fmt.Sprintf("steam user %d has no completed import", steamID))
			return
		}
	}

	rows, err := handler.apiStore.queries.ListCommonGames(request.Context(), dbgen.ListCommonGamesParams{
		RowLimit: int32(limit),
		SteamIds: steamIDs,
	})
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	response := commonGamesResponse{Games: make([]commonGameView, 0, len(rows))}
	for _, row := range rows {
		response.Games = append(response.Games, newCommonGameView(row))
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, response)
}

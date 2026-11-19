package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
)

type insertSteamUserRequest struct {
	VanityUrl string `json:"vanityUrl"`
}

// ImportRequester creates or returns a user's import job. It's satisfied by
// *importer.Importer.
type ImportRequester interface {
	RequestImport(ctx context.Context, steamID int64) (dbgen.ImportJob, bool, error)
}

type insertSteamUserResponse struct {
	SteamUser steamUserView `json:"steamUser"`
	ImportJob importJobView `json:"importJob"`
}

func importJobLocation(id int64) string {
	return fmt.Sprintf("/api/import-jobs/%d", id)
}

// InsertSteamUser saves the user and queues a library import. It responds 202
// when a new job was created, or 200 with the existing active or still-fresh
// job.
func (handler *Handler) InsertSteamUser(responseWriter http.ResponseWriter, request *http.Request) {
	// TODO::Require user login with write permissions
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

	steamUser, err := handler.apiStore.queries.UpsertSteamUser(request.Context(), dbgen.UpsertSteamUserParams{
		SteamID: steamID,
		Name:    vanityUrl,
	})
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	job, created, err := handler.importer.RequestImport(request.Context(), steamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusAccepted
		responseWriter.Header().Set("Location", importJobLocation(job.ID))
	}
	httpx.RespondWithJSON(responseWriter, status, insertSteamUserResponse{
		SteamUser: newSteamUserView(steamUser),
		ImportJob: newImportJobView(job),
	})
}

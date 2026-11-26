package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
)

type insertSteamUserRequest struct {
	VanityUrl string `json:"vanityUrl"`
	// SteamID is the ID the Import page is showing. It's optional for
	// backward compatibility; when set, the name must still resolve to it.
	SteamID string `json:"steamId"`
	// Force skips the refresh-interval shortcut and always starts a new job
	// when none is active.
	Force bool `json:"force"`
}

// ImportRequester creates or returns a user's import job and reports their
// import state. It's satisfied by *importer.Importer.
type ImportRequester interface {
	RequestImport(ctx context.Context, steamID int64) (dbgen.ImportJob, bool, error)
	RequestForcedImport(ctx context.Context, steamID int64) (dbgen.ImportJob, bool, error)
	Snapshot(ctx context.Context, steamID int64) (importer.ImportSnapshot, error)
}

type insertSteamUserResponse struct {
	SteamUser steamUserView `json:"steamUser"`
	ImportJob importJobView `json:"importJob"`
}

func importJobLocation(id int64) string {
	return fmt.Sprintf("/api/import-jobs/%d", id)
}

// InsertSteamUser saves the user and queues a library import. It responds
// 202 when a new job was created, or 200 with the existing active or
// still-fresh job. A steamId that no longer matches the resolved name is
// rejected with 409 before anything is written, and force skips the
// refresh-interval shortcut while still reusing an active job.
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
	var previewedID int64
	if rawSteamID := strings.TrimSpace(body.SteamID); rawSteamID != "" {
		parsed, err := parseSteamID(rawSteamID)
		if err != nil {
			httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
			return
		}
		previewedID = parsed
	}

	steamID, player, err := handler.resolveSteamPlayer(request.Context(), vanityUrl)
	if err != nil {
		if errors.Is(err, errSteamUserNotFound) {
			httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
			return
		}
		handler.internalError(responseWriter, request, err)
		return
	}

	// The preview may be stale: the name could have been reassigned between
	// the lookup and the click. Refuse to import the wrong account.
	if previewedID != 0 && previewedID != steamID {
		httpx.RespondWithError(responseWriter, http.StatusConflict,
			"The Steam name now resolves to a different user. Look up the user again.")
		return
	}

	// The profile is stored alongside the vanity name so Search Users can show
	// the Steam display name, profile URL, and avatars. Nothing is written and
	// no import is queued when the resolution above fails.
	steamUser, err := handler.apiStore.queries.UpsertSteamUser(request.Context(), dbgen.UpsertSteamUserParams{
		SteamID:      steamID,
		Name:         vanityUrl,
		Personaname:  nullString(player.PersonaName),
		Profileurl:   nullString(player.ProfileURL),
		Avatar:       nullString(player.Avatar),
		AvatarMedium: nullString(player.AvatarMedium),
		AvatarFull:   nullString(player.AvatarFull),
	})
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	requestImport := handler.importer.RequestImport
	if body.Force {
		requestImport = handler.importer.RequestForcedImport
	}
	job, created, err := requestImport(request.Context(), steamID)
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

package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/logging"
)

type insertSteamUserRequest struct {
	VanityUrl string `json:"vanityUrl"`
}

// ImportRequester creates or returns a user's import job. It's satisfied by
// *importer.Importer.
type ImportRequester interface {
	RequestImport(ctx context.Context, steamID int64) (dbgen.ImportJob, bool, error)
}

type Handler struct {
	apiStore *Store
	importer ImportRequester
	logger   *logging.Logger
}

func NewHandler(apiStore *Store, importer ImportRequester, logger *logging.Logger) *Handler {
	return &Handler{
		apiStore: apiStore,
		importer: importer,
		logger:   logger,
	}
}

type insertSteamUserResponse struct {
	SteamUser dbgen.SteamUser `json:"steamUser"`
	ImportJob importJobView   `json:"importJob"`
}

type importJobView struct {
	ID            int64      `json:"id"`
	SteamID       int64      `json:"steamId"`
	Status        string     `json:"status"`
	TotalApps     int32      `json:"totalApps"`
	ProcessedApps int32      `json:"processedApps"`
	SkippedApps   int32      `json:"skippedApps"`
	FailedApps    int32      `json:"failedApps"`
	Error         *string    `json:"error"`
	CreatedAt     time.Time  `json:"createdAt"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

func newImportJobView(job dbgen.ImportJob) importJobView {
	view := importJobView{
		ID:            job.ID,
		SteamID:       job.SteamID,
		Status:        job.Status,
		TotalApps:     job.TotalApps,
		ProcessedApps: job.ProcessedApps,
		SkippedApps:   job.SkippedApps,
		FailedApps:    job.FailedApps,
		CreatedAt:     job.CreatedAt,
	}
	if job.Error.Valid {
		view.Error = &job.Error.String
	}
	if job.StartedAt.Valid {
		view.StartedAt = &job.StartedAt.Time
	}
	if job.FinishedAt.Valid {
		view.FinishedAt = &job.FinishedAt.Time
	}
	return view
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
		SteamUser: steamUser,
		ImportJob: newImportJobView(job),
	})
}

func (handler *Handler) GetImportJob(responseWriter http.ResponseWriter, request *http.Request) {
	id, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "import job id must be a number")
		return
	}
	job, err := handler.apiStore.queries.GetImportJob(request.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "import job not found")
		return
	}
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, newImportJobView(job))
}

func (handler *Handler) GetSteamUserById(responseWriter http.ResponseWriter, request *http.Request) {

}

func (handler *Handler) GetSteamUserByName(responseWriter http.ResponseWriter, request *http.Request) {
	name := strings.TrimSpace(request.URL.Query().Get("name"))
	if name == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "name is required")
		return
	}

	steamUser, err := handler.apiStore.queries.GetSteamUserByName(request.Context(), name)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
		return
	}
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	httpx.RespondWithJSON(responseWriter, http.StatusOK, map[string]any{
		"steamUser": steamUser,
	})
}

func (handler *Handler) GetSteamUsers(responseWriter http.ResponseWriter, request *http.Request) {

}

func (handler *Handler) internalError(responseWriter http.ResponseWriter, request *http.Request, err error) {
	if handler.logger != nil {
		_ = handler.logger.Event("unhandled_error", map[string]any{
			"method":  request.Method,
			"path":    request.URL.Path,
			"message": err.Error(),
		})
	}
	httpx.RespondWithError(responseWriter, http.StatusInternalServerError, err.Error())
}

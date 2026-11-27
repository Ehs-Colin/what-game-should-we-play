package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
)

type dashboardResponse struct {
	TotalSteamUsers int64 `json:"totalSteamUsers"`
	TotalApps       int64 `json:"totalApps"`
	ImportJobs      struct {
		Running int64 `json:"running"`
		Queued  int64 `json:"queued"`
	} `json:"importJobs"`
}

// GetDashboard returns the database-wide counts shown on the dashboard.
func (handler *Handler) GetDashboard(responseWriter http.ResponseWriter, request *http.Request) {
	stats, err := handler.apiStore.queries.GetDashboardStats(request.Context())
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	var response dashboardResponse
	response.TotalSteamUsers = stats.TotalSteamUsers
	response.TotalApps = stats.TotalApps
	response.ImportJobs.Running = stats.RunningJobs
	response.ImportJobs.Queued = stats.QueuedJobs
	httpx.RespondWithJSON(responseWriter, http.StatusOK, response)
}

type importJobsResponse struct {
	ImportJobs []importJobListView `json:"importJobs"`
}

// ListImportJobs returns every import job, newest first. The browser handles
// sorting ties and pagination.
func (handler *Handler) ListImportJobs(responseWriter http.ResponseWriter, request *http.Request) {
	rows, err := handler.apiStore.queries.ListImportJobs(request.Context())
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	// make(..., 0, ...) serializes an empty list as [], not null.
	jobs := make([]importJobListView, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, newImportJobListView(row))
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, importJobsResponse{ImportJobs: jobs})
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

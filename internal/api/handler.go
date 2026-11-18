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

// func (handler *Handler) GetSteamUserById(responseWriter http.ResponseWriter, request *http.Request) {

// }

func (handler *Handler) GetSteamUsersByIDs(responseWriter http.ResponseWriter, request *http.Request) {
	rawIDs := strings.TrimSpace(request.URL.Query().Get("steamIds"))
	if rawIDs == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "steamIds is required")
		return
	}

	steamIDs, err := parseSteamIDs(strings.Split(rawIDs, ","))
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		return
	}

	rows, err := handler.apiStore.queries.ListImportedSteamUsers(request.Context(), steamIDs)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	users := make([]steamUserLookupView, 0, len(rows))
	for _, row := range rows {
		users = append(users, newSteamUserLookupView(row))
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, map[string]any{
		"steamUsers": users,
	})
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
		"steamUser": newSteamUserView(steamUser),
	})
}

type searchSteamUsersResponse struct {
	SteamUsers []steamUserSummaryView `json:"steamUsers"`
}

// SearchSteamUsers returns users whose names contain the query
// case-insensitively. Sorting and filtering happen in the browser.
func (handler *Handler) SearchSteamUsers(responseWriter http.ResponseWriter, request *http.Request) {
	name := strings.TrimSpace(request.URL.Query().Get("name"))
	if name == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "name is required")
		return
	}

	rows, err := handler.apiStore.queries.SearchSteamUsers(request.Context(), name)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	response := searchSteamUsersResponse{SteamUsers: make([]steamUserSummaryView, 0, len(rows))}
	for _, row := range rows {
		response.SteamUsers = append(response.SteamUsers, newSteamUserSummaryView(row))
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, response)
}

type commonGamesRequest struct {
	SteamIDs []string `json:"steamIds"`
	Limit    *int     `json:"limit"`
}

type commonGamesResponse struct {
	Games []commonGameView `json:"games"`
}

const (
	defaultCommonGamesLimit = 10
	maxCommonGamesLimit     = 100
	maxCommonGameUsers      = 25
	maxCommonGamesBodyBytes = 4 << 10
)

func parseSteamIDs(rawIDs []string) ([]int64, error) {
	seen := make(map[int64]struct{}, len(rawIDs))
	steamIDs := make([]int64, 0, len(rawIDs))
	for _, rawSteamID := range rawIDs {
		value := strings.TrimSpace(rawSteamID)
		if value == "" {
			return nil, errors.New("steamIds must not contain blank values")
		}
		steamID, err := strconv.ParseInt(value, 10, 64)
		if err != nil || steamID <= 0 {
			return nil, errors.New("steamIds must contain positive SteamID64 values")
		}
		if _, duplicate := seen[steamID]; duplicate {
			continue
		}
		seen[steamID] = struct{}{}
		steamIDs = append(steamIDs, steamID)
	}
	if len(steamIDs) == 0 {
		return nil, errors.New("steamIds is required")
	}
	if len(steamIDs) > maxCommonGameUsers {
		return nil, fmt.Errorf("at most %d unique steamIds are allowed", maxCommonGameUsers)
	}
	return steamIDs, nil
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

type steamUserViewResponse struct {
	SteamUser        steamUserView    `json:"steamUser"`
	OwnedAppCount    int64            `json:"ownedAppCount"`
	WishlistAppCount int64            `json:"wishlistAppCount"`
	LatestImport     *importJobView   `json:"latestImport"`
	HasActiveJob     bool             `json:"hasActiveJob"`
	List             string           `json:"list"`
	Page             int              `json:"page"`
	PageSize         int              `json:"pageSize"`
	Total            int64            `json:"total"`
	TotalPages       int64            `json:"totalPages"`
	Apps             []appSummaryView `json:"apps"`
}

const (
	appListOwned    = "owned"
	appListWishlist = "wishlist"

	defaultAppPageSize = 25
)

// GetSteamUserView resolves an exact Steam name and returns one page of that
// user's owned or wishlisted apps.
func (handler *Handler) GetSteamUserView(responseWriter http.ResponseWriter, request *http.Request) {
	name := strings.TrimSpace(request.URL.Query().Get("name"))
	if name == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "name is required")
		return
	}

	list, ok := parseAppList(request.URL.Query().Get("list"))
	if !ok {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "list must be owned or wishlist")
		return
	}

	page, ok := parsePositivePage(request.URL.Query().Get("page"))
	if !ok {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "page must be a positive number")
		return
	}

	pageSize, ok := parsePageSize(request.URL.Query().Get("pageSize"))
	if !ok {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "pageSize must be 25, 50, or 100")
		return
	}

	ctx := request.Context()
	steamUser, err := handler.apiStore.queries.GetSteamUserByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
		return
	}
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	ownedAppCount, err := handler.apiStore.queries.CountActiveSteamUserApps(ctx, steamUser.SteamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	wishlistAppCount, err := handler.apiStore.queries.CountActiveSteamUserWishlist(ctx, steamUser.SteamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	total := ownedAppCount
	if list == appListWishlist {
		total = wishlistAppCount
	}
	totalPages := int64(0)
	if total > 0 {
		totalPages = (total + int64(pageSize) - 1) / int64(pageSize)
	}
	if totalPages == 0 {
		page = 1
	} else if int64(page) > totalPages {
		page = int(totalPages)
	}

	params := dbgen.ListActiveSteamUserAppsPageParams{
		SteamID:   steamUser.SteamID,
		RowOffset: int32((page - 1) * pageSize),
		PageSize:  int32(pageSize),
	}
	var apps []dbgen.App
	if list == appListWishlist {
		apps, err = handler.apiStore.queries.ListActiveSteamUserWishlistPage(ctx, dbgen.ListActiveSteamUserWishlistPageParams(params))
	} else {
		apps, err = handler.apiStore.queries.ListActiveSteamUserAppsPage(ctx, params)
	}
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	var latestImport *importJobView
	latestJob, err := handler.apiStore.queries.GetLatestImportJobForUser(ctx, steamUser.SteamID)
	if errors.Is(err, sql.ErrNoRows) {
		latestImport = nil
	} else if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	} else {
		view := newImportJobView(latestJob)
		latestImport = &view
	}

	hasActiveJob := true
	if _, err := handler.apiStore.queries.GetActiveImportJobForUser(ctx, steamUser.SteamID); errors.Is(err, sql.ErrNoRows) {
		hasActiveJob = false
	} else if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	response := steamUserViewResponse{
		SteamUser:        newSteamUserView(steamUser),
		OwnedAppCount:    ownedAppCount,
		WishlistAppCount: wishlistAppCount,
		LatestImport:     latestImport,
		HasActiveJob:     hasActiveJob,
		List:             list,
		Page:             page,
		PageSize:         pageSize,
		Total:            total,
		TotalPages:       totalPages,
		Apps:             make([]appSummaryView, 0, len(apps)),
	}
	for _, app := range apps {
		response.Apps = append(response.Apps, newAppSummaryView(app))
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, response)
}

// parseAppList defaults an empty value to owned and rejects unknown lists.
func parseAppList(value string) (string, bool) {
	switch value {
	case "", appListOwned:
		return appListOwned, true
	case appListWishlist:
		return appListWishlist, true
	default:
		return "", false
	}
}

// parsePositivePage defaults an empty value to page 1.
func parsePositivePage(value string) (int, bool) {
	if value == "" {
		return 1, true
	}
	page, err := strconv.Atoi(value)
	if err != nil || page < 1 {
		return 0, false
	}
	return page, true
}

// parsePageSize defaults an empty value to 25 and accepts only the sizes the
// pagination controls offer.
func parsePageSize(value string) (int, bool) {
	if value == "" {
		return defaultAppPageSize, true
	}
	pageSize, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	switch pageSize {
	case 25, 50, 100:
		return pageSize, true
	default:
		return 0, false
	}
}

// func (handler *Handler) GetSteamUsers(responseWriter http.ResponseWriter, request *http.Request) {

// }

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

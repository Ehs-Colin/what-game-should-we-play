package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
)

const (
	appListOwned    = "owned"
	appListWishlist = "wishlist"

	defaultAppPageSize = 25
)

type searchSteamUsersResponse struct {
	SteamUsers []steamUserSummaryView `json:"steamUsers"`
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

package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/steamapi"
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
	Groups           []groupRefView   `json:"groups"`
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

	steamIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		steamIDs = append(steamIDs, row.SteamID)
	}
	groupsBySteamID, err := handler.groupsBySteamID(request.Context(), steamIDs)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	response := searchSteamUsersResponse{SteamUsers: make([]steamUserSummaryView, 0, len(rows))}
	for _, row := range rows {
		user := newSteamUserSummaryView(row)
		user.Groups = groupsBySteamID[row.SteamID]
		response.SteamUsers = append(response.SteamUsers, user)
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, response)
}

// groupsBySteamID maps Steam IDs to the group references shown on
// steamUserSummaryView rows. Every requested ID is present in the result,
// empty when the user joins no groups, so callers can assign it directly.
// The Search Users and View Group handlers share it so their member rows
// stay identical.
func (handler *Handler) groupsBySteamID(ctx context.Context, steamIDs []int64) (map[int64][]groupRefView, error) {
	groups := make(map[int64][]groupRefView, len(steamIDs))
	for _, steamID := range steamIDs {
		groups[steamID] = []groupRefView{}
	}
	if len(steamIDs) == 0 {
		return groups, nil
	}
	rows, err := handler.apiStore.queries.ListGroupsForSteamUsers(ctx, steamIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		groups[row.SteamID] = append(groups[row.SteamID], groupRefView{
			GroupID: row.ID,
			Name:    row.Name,
		})
	}
	return groups, nil
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

// Lookup sources. "database" means the stored row matched the typed name;
// "steam" means the name had to be resolved through Steam, including a
// renamed vanity that matched a stored Steam ID.
const (
	lookupSourceDatabase = "database"
	lookupSourceSteam    = "steam"
)

// steamUserLookupResponse is the GET /api/steamusers payload: the profile,
// the server-computed import state, the job that explains it, and the
// current owned/wishlist counts. Counts and the timestamps are nil for a
// Steam preview because the library has not been imported.
type steamUserLookupResponse struct {
	SteamUser        steamUserView        `json:"steamUser"`
	Source           string               `json:"source"`
	ImportState      importer.ImportState `json:"importState"`
	LatestImport     *importJobView       `json:"latestImport"`
	HasActiveJob     bool                 `json:"hasActiveJob"`
	OwnedAppCount    *int64               `json:"ownedAppCount"`
	WishlistAppCount *int64               `json:"wishlistAppCount"`
}

// GetSteamUserByName looks a user up without changing anything. A stored row
// returns its import state, latest job, and counts. An unknown name is
// resolved with read-only Steam calls: a renamed vanity reports the stored
// row's status, otherwise the profile comes back as an unimported preview.
// Nothing is written and no job is queued on the miss path; a 404 means
// neither the database nor Steam knows the name.
func (handler *Handler) GetSteamUserByName(responseWriter http.ResponseWriter, request *http.Request) {
	name := strings.TrimSpace(request.URL.Query().Get("name"))
	if name == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "name is required")
		return
	}

	ctx := request.Context()
	steamUser, err := handler.apiStore.queries.GetSteamUserByName(ctx, name)
	if err == nil {
		handler.respondStoredLookup(responseWriter, request, steamUser, lookupSourceDatabase)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handler.internalError(responseWriter, request, err)
		return
	}

	// The name is not stored. Numeric input resolves by Steam ID; anything
	// else is treated as a vanity name. Neither path writes anything.
	if parsedID, parseErr := parseSteamID(name); parseErr == nil {
		handler.respondIDLookup(responseWriter, request, parsedID)
		return
	}

	// Resolve the vanity with read-only Steam calls; this path must not
	// upsert the user or request an import.
	steamID, player, err := handler.resolveSteamPlayer(ctx, name)
	if err != nil {
		if errors.Is(err, errSteamUserNotFound) {
			httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
			return
		}
		handler.internalError(responseWriter, request, err)
		return
	}

	// The resolved ID may already be stored under an older name. That row
	// owns the import history the user should see.
	stored, err := handler.apiStore.queries.GetSteamUser(ctx, steamID)
	if err == nil {
		handler.respondStoredLookup(responseWriter, request, stored, lookupSourceSteam)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handler.internalError(responseWriter, request, err)
		return
	}

	httpx.RespondWithJSON(responseWriter, http.StatusOK, steamUserLookupResponse{
		SteamUser:   newSteamUserPreviewView(name, steamID, player),
		Source:      lookupSourceSteam,
		ImportState: importer.StateNotImported,
	})
}

// respondIDLookup answers a numeric lookup: a stored row by ID first, so a
// renamed or custom-labeled user keeps their import history, then a
// read-only profile preview for users the database does not know. A 404
// means Steam has no profile for the ID either.
func (handler *Handler) respondIDLookup(responseWriter http.ResponseWriter, request *http.Request, steamID int64) {
	ctx := request.Context()
	stored, err := handler.apiStore.queries.GetSteamUser(ctx, steamID)
	if err == nil {
		handler.respondStoredLookup(responseWriter, request, stored, lookupSourceDatabase)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		handler.internalError(responseWriter, request, err)
		return
	}

	player, err := handler.fetchSteamPlayerByID(ctx, steamID)
	if err != nil {
		if errors.Is(err, errSteamUserNotFound) {
			httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
			return
		}
		handler.internalError(responseWriter, request, err)
		return
	}

	httpx.RespondWithJSON(responseWriter, http.StatusOK, steamUserLookupResponse{
		SteamUser:   newSteamUserPreviewView(formatSteamID(steamID), steamID, player),
		Source:      lookupSourceSteam,
		ImportState: importer.StateNotImported,
	})
}

// respondStoredLookup completes a lookup for a stored row with its app
// counts and importer snapshot.
func (handler *Handler) respondStoredLookup(responseWriter http.ResponseWriter, request *http.Request, steamUser dbgen.SteamUser, source string) {
	ctx := request.Context()
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
	snapshot, err := handler.importer.Snapshot(ctx, steamUser.SteamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	response := steamUserLookupResponse{
		SteamUser:        newSteamUserView(steamUser),
		Source:           source,
		ImportState:      snapshot.State,
		HasActiveJob:     snapshot.HasActiveJob,
		OwnedAppCount:    &ownedAppCount,
		WishlistAppCount: &wishlistAppCount,
	}
	if snapshot.Latest != nil {
		view := newImportJobView(*snapshot.Latest)
		response.LatestImport = &view
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, response)
}

// newSteamUserPreviewView builds the view for a Steam profile that is not in
// the database. Timestamps stay nil so the JSON omits them, and empty
// profile fields become null the way nullString stores them.
func newSteamUserPreviewView(name string, steamID int64, player steamapi.PlayerSummary) steamUserView {
	return steamUserView{
		SteamID:      formatSteamID(steamID),
		Name:         name,
		PersonaName:  previewString(player.PersonaName),
		ProfileURL:   previewString(player.ProfileURL),
		Avatar:       previewString(player.Avatar),
		AvatarMedium: previewString(player.AvatarMedium),
		AvatarFull:   previewString(player.AvatarFull),
	}
}

func previewString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
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

	groupRows, err := handler.apiStore.queries.ListGroupsForSteamUser(ctx, steamUser.SteamID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	groups := make([]groupRefView, 0, len(groupRows))
	for _, row := range groupRows {
		groups = append(groups, groupRefView{GroupID: row.ID, Name: row.Name})
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
		Groups:           groups,
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

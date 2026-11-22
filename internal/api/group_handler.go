package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/httpx"
)

const (
	maxGroupNameLength = 64
	maxGroupBodyBytes  = 1 << 10
	maxGroupIDs        = 50

	groupNameUniqueConstraint = "steam_user_groups_name_lower"
)

var (
	errInvalidGroupName = errors.New("name must be between 1 and 64 characters")
	errInvalidGroupID   = errors.New("group id must be a positive number")
)

type groupNameRequest struct {
	Name string `json:"name"`
}

type groupResponse struct {
	Group groupView `json:"group"`
}

type groupViewResponse struct {
	Group   groupView              `json:"group"`
	Members []steamUserSummaryView `json:"members"`
}

type groupSummariesResponse struct {
	Groups []groupSummaryView `json:"groups"`
}

// SearchGroups returns groups whose names contain the query
// case-insensitively, each with its members. Sorting happens in the browser.
func (handler *Handler) SearchGroups(responseWriter http.ResponseWriter, request *http.Request) {
	name := strings.TrimSpace(request.URL.Query().Get("name"))
	if name == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "name is required")
		return
	}

	ctx := request.Context()
	rows, err := handler.apiStore.queries.SearchGroups(ctx, name)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	groups, err := handler.groupSummaries(request, rows)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, groupSummariesResponse{Groups: groups})
}

// GetGroupsByIDs returns the requested groups in request order, each with its
// members. Any unknown id fails the whole request.
func (handler *Handler) GetGroupsByIDs(responseWriter http.ResponseWriter, request *http.Request) {
	rawIDs := strings.TrimSpace(request.URL.Query().Get("groupIds"))
	if rawIDs == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "groupIds is required")
		return
	}
	groupIDs, err := parseGroupIDs(strings.Split(rawIDs, ","))
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		return
	}

	ctx := request.Context()
	listed, err := handler.apiStore.queries.ListGroupsByIDs(ctx, groupIDs)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	byID := make(map[int64]dbgen.SearchGroupsRow, len(listed))
	for _, row := range listed {
		byID[row.ID] = dbgen.SearchGroupsRow(row)
	}
	rows := make([]dbgen.SearchGroupsRow, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		row, found := byID[groupID]
		if !found {
			httpx.RespondWithError(responseWriter, http.StatusNotFound, fmt.Sprintf("group %d not found", groupID))
			return
		}
		rows = append(rows, row)
	}

	groups, err := handler.groupSummaries(request, rows)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, groupSummariesResponse{Groups: groups})
}

// groupSummaries attaches member lookups to group rows, keeping row order.
func (handler *Handler) groupSummaries(request *http.Request, rows []dbgen.SearchGroupsRow) ([]groupSummaryView, error) {
	groups := make([]groupSummaryView, 0, len(rows))
	if len(rows) == 0 {
		return groups, nil
	}

	groupIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		groupIDs = append(groupIDs, row.ID)
	}
	memberRows, err := handler.apiStore.queries.ListGroupMemberLookups(request.Context(), groupIDs)
	if err != nil {
		return nil, err
	}
	members := make(map[int64][]steamUserLookupView, len(rows))
	for _, member := range memberRows {
		members[member.GroupID] = append(members[member.GroupID], steamUserLookupView{
			SteamID:            formatSteamID(member.SteamID),
			Name:               member.Name,
			HasSucceededImport: member.HasSucceededImport,
		})
	}

	for _, row := range rows {
		groups = append(groups, newGroupSummaryView(row, members[row.ID]))
	}
	return groups, nil
}

// CreateGroup creates an empty group with a unique name.
func (handler *Handler) CreateGroup(responseWriter http.ResponseWriter, request *http.Request) {
	name, ok := decodeGroupName(responseWriter, request)
	if !ok {
		return
	}

	group, err := handler.apiStore.queries.CreateGroup(request.Context(), name)
	if isGroupNameTaken(err) {
		httpx.RespondWithError(responseWriter, http.StatusConflict, "group name is already taken")
		return
	}
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	httpx.RespondWithJSON(responseWriter, http.StatusCreated, groupResponse{
		Group: newGroupView(group, dbgen.GetGroupCountsRow{}),
	})
}

// RenameGroup changes a group's name. Renaming to a different case of its own
// name is allowed.
func (handler *Handler) RenameGroup(responseWriter http.ResponseWriter, request *http.Request) {
	groupID, err := parseGroupID(request.PathValue("id"))
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		return
	}
	name, ok := decodeGroupName(responseWriter, request)
	if !ok {
		return
	}

	ctx := request.Context()
	group, err := handler.apiStore.queries.RenameGroup(ctx, dbgen.RenameGroupParams{
		ID:   groupID,
		Name: name,
	})
	if errors.Is(err, sql.ErrNoRows) {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "group not found")
		return
	}
	if isGroupNameTaken(err) {
		httpx.RespondWithError(responseWriter, http.StatusConflict, "group name is already taken")
		return
	}
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	counts, err := handler.apiStore.queries.GetGroupCounts(ctx, group.ID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, groupResponse{
		Group: newGroupView(group, counts),
	})
}

// DeleteGroup deletes a group. Its memberships are removed by cascade.
func (handler *Handler) DeleteGroup(responseWriter http.ResponseWriter, request *http.Request) {
	groupID, err := parseGroupID(request.PathValue("id"))
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		return
	}

	deleted, err := handler.apiStore.queries.DeleteGroup(request.Context(), groupID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	if deleted == 0 {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "group not found")
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

// GetGroupView resolves an exact group name, ignoring case, for View Group.
func (handler *Handler) GetGroupView(responseWriter http.ResponseWriter, request *http.Request) {
	name := strings.TrimSpace(request.URL.Query().Get("name"))
	if name == "" {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "name is required")
		return
	}

	ctx := request.Context()
	group, err := handler.apiStore.queries.GetGroupByName(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "group not found")
		return
	}
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	counts, err := handler.apiStore.queries.GetGroupCounts(ctx, group.ID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	rows, err := handler.apiStore.queries.ListGroupMembers(ctx, group.ID)
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	response := groupViewResponse{
		Group:   newGroupView(group, counts),
		Members: make([]steamUserSummaryView, 0, len(rows)),
	}
	for _, row := range rows {
		response.Members = append(response.Members, newSteamUserSummaryView(dbgen.SearchSteamUsersRow(row)))
	}
	httpx.RespondWithJSON(responseWriter, http.StatusOK, response)
}

// AddGroupMember adds an existing Steam user to a group. Adding a current
// member succeeds without changes.
func (handler *Handler) AddGroupMember(responseWriter http.ResponseWriter, request *http.Request) {
	groupID, steamID, ok := parseGroupMemberPath(responseWriter, request)
	if !ok {
		return
	}

	ctx := request.Context()
	if !handler.requireGroup(responseWriter, request, groupID) {
		return
	}
	if _, err := handler.apiStore.queries.GetSteamUser(ctx, steamID); errors.Is(err, sql.ErrNoRows) {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
		return
	} else if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}

	if err := handler.apiStore.queries.AddGroupMember(ctx, dbgen.AddGroupMemberParams{
		GroupID: groupID,
		SteamID: steamID,
	}); err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	if err := handler.apiStore.queries.TouchGroup(ctx, groupID); err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

// RemoveGroupMember removes a Steam user from a group. Removing a non-member
// succeeds without changes.
func (handler *Handler) RemoveGroupMember(responseWriter http.ResponseWriter, request *http.Request) {
	groupID, steamID, ok := parseGroupMemberPath(responseWriter, request)
	if !ok {
		return
	}

	ctx := request.Context()
	if !handler.requireGroup(responseWriter, request, groupID) {
		return
	}

	removed, err := handler.apiStore.queries.RemoveGroupMember(ctx, dbgen.RemoveGroupMemberParams{
		GroupID: groupID,
		SteamID: steamID,
	})
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return
	}
	if removed > 0 {
		if err := handler.apiStore.queries.TouchGroup(ctx, groupID); err != nil {
			handler.internalError(responseWriter, request, err)
			return
		}
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

// parseGroupMemberPath reads {id} and {steamId} and writes a 400 response when
// either is invalid.
func parseGroupMemberPath(responseWriter http.ResponseWriter, request *http.Request) (int64, int64, bool) {
	groupID, err := parseGroupID(request.PathValue("id"))
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		return 0, 0, false
	}
	steamID, err := parseSteamID(request.PathValue("steamId"))
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		return 0, 0, false
	}
	return groupID, steamID, true
}

// requireGroup writes a 404 response when the group does not exist.
func (handler *Handler) requireGroup(responseWriter http.ResponseWriter, request *http.Request, groupID int64) bool {
	_, err := handler.apiStore.queries.GetGroup(request.Context(), groupID)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.RespondWithError(responseWriter, http.StatusNotFound, "group not found")
		return false
	}
	if err != nil {
		handler.internalError(responseWriter, request, err)
		return false
	}
	return true
}

// decodeGroupName reads a { "name" } body and writes a 400 response when the
// body or name is invalid.
func decodeGroupName(responseWriter http.ResponseWriter, request *http.Request) (string, bool) {
	var body groupNameRequest
	request.Body = http.MaxBytesReader(responseWriter, request.Body, maxGroupBodyBytes)
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, "invalid JSON body")
		return "", false
	}
	name, err := normalizeGroupName(body.Name)
	if err != nil {
		httpx.RespondWithError(responseWriter, http.StatusBadRequest, err.Error())
		return "", false
	}
	return name, true
}

// normalizeGroupName trims the name and checks its length in characters, which
// is how VARCHAR(64) counts.
func normalizeGroupName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	length := utf8.RuneCountInString(name)
	if length < 1 || length > maxGroupNameLength {
		return "", errInvalidGroupName
	}
	return name, nil
}

func parseGroupID(raw string) (int64, error) {
	groupID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || groupID < 1 {
		return 0, errInvalidGroupID
	}
	return groupID, nil
}

// parseGroupIDs trims and deduplicates ids, keeping the first occurrence's
// order.
func parseGroupIDs(rawIDs []string) ([]int64, error) {
	seen := make(map[int64]struct{}, len(rawIDs))
	groupIDs := make([]int64, 0, len(rawIDs))
	for _, rawGroupID := range rawIDs {
		value := strings.TrimSpace(rawGroupID)
		if value == "" {
			return nil, errors.New("groupIds must not contain blank values")
		}
		groupID, err := parseGroupID(value)
		if err != nil {
			return nil, errors.New("groupIds must contain positive numbers")
		}
		if _, duplicate := seen[groupID]; duplicate {
			continue
		}
		seen[groupID] = struct{}{}
		groupIDs = append(groupIDs, groupID)
	}
	if len(groupIDs) == 0 {
		return nil, errors.New("groupIds is required")
	}
	if len(groupIDs) > maxGroupIDs {
		return nil, fmt.Errorf("at most %d unique groupIds are allowed", maxGroupIDs)
	}
	return groupIDs, nil
}

func isGroupNameTaken(err error) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) &&
		postgresError.Code == "23505" &&
		postgresError.ConstraintName == groupNameUniqueConstraint
}

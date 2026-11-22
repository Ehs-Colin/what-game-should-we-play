package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Ehs-Colin/what-game-should-we-play/internal/database/dbgen"
	"github.com/Ehs-Colin/what-game-should-we-play/internal/importer"
)

const (
	groupAlice = int64(76561198000000201)
	groupBob   = int64(76561198000000202)
	groupCarol = int64(76561198000000203)
)

// ListGroupMembers must keep the same columns as SearchSteamUsers so group
// members can be rendered with newSteamUserSummaryView.
var _ = func(row dbgen.ListGroupMembersRow) dbgen.SearchSteamUsersRow {
	return dbgen.SearchSteamUsersRow(row)
}

func seedGroup(t *testing.T, env *handlerEnv, name string, steamIDs ...int64) dbgen.SteamUserGroup {
	t.Helper()
	ctx := context.Background()
	group, err := env.queries.CreateGroup(ctx, name)
	if err != nil {
		t.Fatalf("create group %q: %v", name, err)
	}
	for _, steamID := range steamIDs {
		if err := env.queries.AddGroupMember(ctx, dbgen.AddGroupMemberParams{
			GroupID: group.ID,
			SteamID: steamID,
		}); err != nil {
			t.Fatalf("add %d to group %q: %v", steamID, name, err)
		}
	}
	return group
}

func groupMembershipCount(t *testing.T, env *handlerEnv, query string, arg int64) int {
	t.Helper()
	var count int
	if err := env.db.QueryRowContext(context.Background(), query, arg).Scan(&count); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	return count
}

func TestGroupQueriesCountsAndCascades(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	ctx := context.Background()
	now := time.Now()

	seedSteamUser(t, env, groupAlice, "Alice")
	seedSteamUser(t, env, groupBob, "Bob")
	seedSteamUser(t, env, groupCarol, "Carol")
	seedImportJob(t, env, groupAlice, importer.StatusSucceeded, now)
	// Bob's latest import failed, but an earlier one succeeded.
	seedImportJob(t, env, groupBob, importer.StatusSucceeded, now.Add(-time.Hour))
	seedImportJob(t, env, groupBob, importer.StatusFailed, now)
	seedImportJob(t, env, groupCarol, importer.StatusQueued, now)

	friends := seedGroup(t, env, "Friends", groupAlice, groupBob, groupCarol)
	family := seedGroup(t, env, "Family", groupAlice)
	// Adding an existing member is a no-op.
	if err := env.queries.AddGroupMember(ctx, dbgen.AddGroupMemberParams{GroupID: family.ID, SteamID: groupAlice}); err != nil {
		t.Fatalf("re-add member: %v", err)
	}

	if _, err := env.queries.CreateGroup(ctx, "FRIENDS"); err == nil {
		t.Fatal("expected a case-insensitive duplicate group name to fail")
	}

	rows, err := env.queries.SearchGroups(ctx, "")
	if err != nil {
		t.Fatalf("search groups: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d groups, want 2", len(rows))
	}
	// Ordered by name.
	if rows[0].ID != family.ID || rows[1].ID != friends.ID {
		t.Fatalf("got group order %d, %d; want %d, %d", rows[0].ID, rows[1].ID, family.ID, friends.ID)
	}
	if rows[0].MemberCount != 1 || rows[0].UnavailableMemberCount != 0 {
		t.Errorf("Family counts = %d/%d, want 1/0", rows[0].MemberCount, rows[0].UnavailableMemberCount)
	}
	if rows[1].MemberCount != 3 || rows[1].UnavailableMemberCount != 1 {
		t.Errorf("Friends counts = %d/%d, want 3/1", rows[1].MemberCount, rows[1].UnavailableMemberCount)
	}

	partial, err := env.queries.SearchGroups(ctx, "RIEN")
	if err != nil {
		t.Fatalf("search groups partial: %v", err)
	}
	if len(partial) != 1 || partial[0].ID != friends.ID {
		t.Fatalf("partial search got %+v, want only Friends", partial)
	}

	byName, err := env.queries.GetGroupByName(ctx, "fRiEnDs")
	if err != nil || byName.ID != friends.ID {
		t.Fatalf("get group by name = %+v, %v; want Friends", byName, err)
	}

	counts, err := env.queries.GetGroupCounts(ctx, friends.ID)
	if err != nil {
		t.Fatalf("get group counts: %v", err)
	}
	if counts.MemberCount != 3 || counts.UnavailableMemberCount != 1 {
		t.Errorf("GetGroupCounts = %d/%d, want 3/1", counts.MemberCount, counts.UnavailableMemberCount)
	}

	members, err := env.queries.ListGroupMembers(ctx, friends.ID)
	if err != nil {
		t.Fatalf("list group members: %v", err)
	}
	if len(members) != 3 || members[0].Name != "Alice" || members[1].Name != "Bob" || members[2].Name != "Carol" {
		t.Fatalf("got members %+v, want Alice, Bob, Carol", members)
	}

	lookups, err := env.queries.ListGroupMemberLookups(ctx, []int64{friends.ID, family.ID})
	if err != nil {
		t.Fatalf("list group member lookups: %v", err)
	}
	if len(lookups) != 4 {
		t.Fatalf("got %d member lookups, want 4", len(lookups))
	}

	// Deleting a Steam user cascades to its memberships.
	if _, err := env.db.ExecContext(ctx, "DELETE FROM import_jobs WHERE steam_id = $1", groupCarol); err != nil {
		t.Fatalf("delete carol import jobs: %v", err)
	}
	if err := env.queries.DeleteSteamUser(ctx, groupCarol); err != nil {
		t.Fatalf("delete carol: %v", err)
	}
	if got := groupMembershipCount(t, env, "SELECT COUNT(*) FROM steam_user_group_members WHERE steam_id = $1", groupCarol); got != 0 {
		t.Errorf("Carol has %d memberships after delete, want 0", got)
	}
	byIDs, err := env.queries.ListGroupsByIDs(ctx, []int64{friends.ID})
	if err != nil {
		t.Fatalf("list groups by ids: %v", err)
	}
	if len(byIDs) != 1 || byIDs[0].MemberCount != 2 || byIDs[0].UnavailableMemberCount != 0 {
		t.Fatalf("Friends after delete = %+v, want 2/0", byIDs)
	}

	// Deleting a group cascades to its memberships.
	deleted, err := env.queries.DeleteGroup(ctx, friends.ID)
	if err != nil || deleted != 1 {
		t.Fatalf("delete group = %d, %v; want 1 row", deleted, err)
	}
	if got := groupMembershipCount(t, env, "SELECT COUNT(*) FROM steam_user_group_members WHERE group_id = $1", friends.ID); got != 0 {
		t.Errorf("deleted group has %d memberships, want 0", got)
	}
	if got := groupMembershipCount(t, env, "SELECT COUNT(*) FROM steam_user_group_members WHERE group_id = $1", family.ID); got != 1 {
		t.Errorf("Family has %d memberships, want 1", got)
	}
}

func groupMemberPath(groupID, steamID int64) string {
	return fmt.Sprintf("/api/groups/%d/members/%d", groupID, steamID)
}

func groupUpdatedAt(t *testing.T, env *handlerEnv, groupID int64) time.Time {
	t.Helper()
	group, err := env.queries.GetGroup(context.Background(), groupID)
	if err != nil {
		t.Fatalf("get group %d: %v", groupID, err)
	}
	return group.UpdatedAt
}

func groupMemberNames(t *testing.T, env *handlerEnv, groupName string) []string {
	t.Helper()
	recorder := getGroupView(t, env, groupName)
	requireStatus(t, recorder, http.StatusOK)
	members := decodeJSON[groupViewResponse](t, recorder).Members
	names := make([]string, 0, len(members))
	for _, member := range members {
		names = append(names, member.Name)
	}
	return names
}

func groupNameBody(name string) string {
	body, _ := json.Marshal(groupNameRequest{Name: name})
	return string(body)
}

func requireErrorMessage(t *testing.T, recorder *httptest.ResponseRecorder, status int, want string) {
	t.Helper()
	requireStatus(t, recorder, status)
	body := decodeJSON[map[string]string](t, recorder)
	if body["error"] != want {
		t.Fatalf("error = %q, want %q", body["error"], want)
	}
}

func createGroupViaAPI(t *testing.T, env *handlerEnv, name string) groupView {
	t.Helper()
	recorder := env.do(t, http.MethodPost, "/api/groups", groupNameBody(name))
	requireStatus(t, recorder, http.StatusCreated)
	return decodeJSON[groupResponse](t, recorder).Group
}

func getGroupView(t *testing.T, env *handlerEnv, name string) *httptest.ResponseRecorder {
	t.Helper()
	return env.do(t, http.MethodGet, "/api/groups/by-name?name="+url.QueryEscape(name), "")
}

func TestCreateGroup(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())

	recorder := env.do(t, http.MethodPost, "/api/groups", groupNameBody("  Friday Night  "))
	requireStatus(t, recorder, http.StatusCreated)
	raw := decodeJSON[map[string]json.RawMessage](t, recorder)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw["group"], &fields); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	assertJSONString(t, fields["name"], "Friday Night")
	assertJSONNumber(t, fields["memberCount"], 0)
	assertJSONNumber(t, fields["unavailableMemberCount"], 0)

	var groupID int64
	if err := json.Unmarshal(fields["groupId"], &groupID); err != nil || groupID < 1 {
		t.Fatalf("groupId = %s, want a positive JSON number", fields["groupId"])
	}
	stored, err := env.queries.GetGroup(context.Background(), groupID)
	if err != nil {
		t.Fatalf("get stored group: %v", err)
	}
	if stored.Name != "Friday Night" {
		t.Errorf("stored name = %q, want the trimmed name", stored.Name)
	}

	// 64 multi-byte characters fit because the limit counts characters.
	createGroupViaAPI(t, env, strings.Repeat("é", maxGroupNameLength))
}

func TestCreateGroupBadRequests(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	createGroupViaAPI(t, env, "Friends")

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string
	}{
		{name: "blank name", body: groupNameBody("   "), wantStatus: http.StatusBadRequest, wantError: errInvalidGroupName.Error()},
		{name: "missing name", body: "{}", wantStatus: http.StatusBadRequest, wantError: errInvalidGroupName.Error()},
		{name: "65 characters", body: groupNameBody(strings.Repeat("a", maxGroupNameLength+1)), wantStatus: http.StatusBadRequest, wantError: errInvalidGroupName.Error()},
		{name: "invalid JSON", body: "{\"name\":", wantStatus: http.StatusBadRequest, wantError: "invalid JSON body"},
		{name: "oversized body", body: groupNameBody(strings.Repeat("a", maxGroupBodyBytes)), wantStatus: http.StatusBadRequest, wantError: "invalid JSON body"},
		{name: "duplicate ignoring case", body: groupNameBody("fRIENDS"), wantStatus: http.StatusConflict, wantError: "group name is already taken"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := env.do(t, http.MethodPost, "/api/groups", test.body)
			requireErrorMessage(t, recorder, test.wantStatus, test.wantError)
		})
	}
}

func TestGetGroupView(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	created := createGroupViaAPI(t, env, "Friends")

	recorder := getGroupView(t, env, "  fRiEnDs ")
	requireStatus(t, recorder, http.StatusOK)
	raw := decodeRawJSON(t, recorder)
	if string(raw["members"]) != "[]" {
		t.Errorf("members = %s, want []", raw["members"])
	}
	var group groupView
	if err := json.Unmarshal(raw["group"], &group); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	if group.GroupID != created.GroupID || group.Name != "Friends" {
		t.Errorf("group = %+v, want Friends (%d)", group, created.GroupID)
	}

	// Lookup is exact, not partial.
	requireErrorMessage(t, getGroupView(t, env, "Friend"), http.StatusNotFound, "group not found")
	requireErrorMessage(t, getGroupView(t, env, "Strangers"), http.StatusNotFound, "group not found")
	requireErrorMessage(t, env.do(t, http.MethodGet, "/api/groups/by-name", ""), http.StatusBadRequest, "name is required")
	requireErrorMessage(t, getGroupView(t, env, "   "), http.StatusBadRequest, "name is required")
}

func TestGetGroupViewCounts(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	seedSteamUser(t, env, groupBob, "Bob")
	seedImportJob(t, env, groupAlice, importer.StatusSucceeded, time.Now())
	seedGroup(t, env, "Friends", groupAlice, groupBob)

	recorder := getGroupView(t, env, "Friends")
	requireStatus(t, recorder, http.StatusOK)
	group := decodeJSON[groupViewResponse](t, recorder).Group
	if group.MemberCount != 2 || group.UnavailableMemberCount != 1 {
		t.Errorf("counts = %d/%d, want 2/1", group.MemberCount, group.UnavailableMemberCount)
	}
}

func TestRenameGroup(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	friends := seedGroup(t, env, "Friends", groupAlice)
	createGroupViaAPI(t, env, "Family")
	path := fmt.Sprintf("/api/groups/%d", friends.ID)

	recorder := env.do(t, http.MethodPatch, path, groupNameBody(" Old Friends "))
	requireStatus(t, recorder, http.StatusOK)
	renamed := decodeJSON[groupResponse](t, recorder).Group
	if renamed.GroupID != friends.ID || renamed.Name != "Old Friends" {
		t.Errorf("renamed = %+v, want Old Friends (%d)", renamed, friends.ID)
	}
	if renamed.MemberCount != 1 || renamed.UnavailableMemberCount != 1 {
		t.Errorf("renamed counts = %d/%d, want 1/1", renamed.MemberCount, renamed.UnavailableMemberCount)
	}
	if renamed.UpdatedAt.Before(friends.UpdatedAt) {
		t.Errorf("updatedAt = %v, want at or after %v", renamed.UpdatedAt, friends.UpdatedAt)
	}
	requireStatus(t, getGroupView(t, env, "Old Friends"), http.StatusOK)
	requireStatus(t, getGroupView(t, env, "Friends"), http.StatusNotFound)

	// A different case of its own name is not a collision.
	recorder = env.do(t, http.MethodPatch, path, groupNameBody("OLD FRIENDS"))
	requireStatus(t, recorder, http.StatusOK)
	if got := decodeJSON[groupResponse](t, recorder).Group.Name; got != "OLD FRIENDS" {
		t.Errorf("name = %q, want OLD FRIENDS", got)
	}

	requireErrorMessage(t, env.do(t, http.MethodPatch, path, groupNameBody("family")), http.StatusConflict, "group name is already taken")
	requireErrorMessage(t, env.do(t, http.MethodPatch, "/api/groups/999999", groupNameBody("Nobody")), http.StatusNotFound, "group not found")
	requireErrorMessage(t, env.do(t, http.MethodPatch, "/api/groups/abc", groupNameBody("Nobody")), http.StatusBadRequest, errInvalidGroupID.Error())
	requireErrorMessage(t, env.do(t, http.MethodPatch, "/api/groups/0", groupNameBody("Nobody")), http.StatusBadRequest, errInvalidGroupID.Error())
	requireErrorMessage(t, env.do(t, http.MethodPatch, path, groupNameBody("")), http.StatusBadRequest, errInvalidGroupName.Error())
	requireErrorMessage(t, env.do(t, http.MethodPatch, path, "not json"), http.StatusBadRequest, "invalid JSON body")
}

func TestDeleteGroup(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	friends := seedGroup(t, env, "Friends", groupAlice)
	path := fmt.Sprintf("/api/groups/%d", friends.ID)

	recorder := env.do(t, http.MethodDelete, path, "")
	requireStatus(t, recorder, http.StatusNoContent)
	if recorder.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", recorder.Body.String())
	}
	requireErrorMessage(t, getGroupView(t, env, "Friends"), http.StatusNotFound, "group not found")
	if got := groupMembershipCount(t, env, "SELECT COUNT(*) FROM steam_user_group_members WHERE group_id = $1", friends.ID); got != 0 {
		t.Errorf("deleted group has %d memberships, want 0", got)
	}

	requireErrorMessage(t, env.do(t, http.MethodDelete, path, ""), http.StatusNotFound, "group not found")
	requireErrorMessage(t, env.do(t, http.MethodDelete, "/api/groups/-1", ""), http.StatusBadRequest, errInvalidGroupID.Error())
	requireErrorMessage(t, env.do(t, http.MethodDelete, "/api/groups/abc", ""), http.StatusBadRequest, errInvalidGroupID.Error())
}

func TestAddGroupMemberMatchesSearchUsers(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	now := time.Now()
	seedSteamUserProfile(t, env, groupAlice, "Alice", steamProfileFields{
		personaName:  "Alice Persona",
		profileURL:   "https://steamcommunity.com/profiles/76561198000000201/",
		avatar:       testAvatar,
		avatarMedium: testAvatarMedium,
		avatarFull:   testAvatarFull,
	})
	seedApp(t, env, 10, "Game")
	seedAlias(t, env, 10, 10)
	seedOwnedApp(t, env, groupAlice, 10, false)
	seedWishlistApp(t, env, groupAlice, 10, false)
	seedImportJob(t, env, groupAlice, importer.StatusSucceeded, now.Add(-time.Hour))
	seedImportJob(t, env, groupAlice, importer.StatusFailed, now)
	friends := seedGroup(t, env, "Friends")

	recorder := env.do(t, http.MethodPut, groupMemberPath(friends.ID, groupAlice), "")
	requireStatus(t, recorder, http.StatusNoContent)
	if recorder.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", recorder.Body.String())
	}
	if updatedAt := groupUpdatedAt(t, env, friends.ID); !updatedAt.After(friends.UpdatedAt) {
		t.Errorf("updatedAt = %v, want after %v", updatedAt, friends.UpdatedAt)
	}

	recorder = getGroupView(t, env, "Friends")
	requireStatus(t, recorder, http.StatusOK)
	view := decodeJSON[groupViewResponse](t, recorder)
	if view.Group.MemberCount != 1 || view.Group.UnavailableMemberCount != 0 {
		t.Errorf("counts = %d/%d, want 1/0", view.Group.MemberCount, view.Group.UnavailableMemberCount)
	}
	if len(view.Members) != 1 {
		t.Fatalf("got %d members, want 1", len(view.Members))
	}

	search := env.do(t, http.MethodGet, "/api/steamusers/search?name=Alice", "")
	requireStatus(t, search, http.StatusOK)
	searchUsers := decodeJSON[searchSteamUsersResponse](t, search).SteamUsers
	if len(searchUsers) != 1 {
		t.Fatalf("search returned %d users, want 1", len(searchUsers))
	}
	if !reflect.DeepEqual(view.Members[0], searchUsers[0]) {
		t.Errorf("member = %+v\nwant the Search Users row %+v", view.Members[0], searchUsers[0])
	}

	member := view.Members[0]
	if member.SteamID != "76561198000000201" || member.OwnedAppCount != 1 || member.WishlistAppCount != 1 ||
		!member.HasSucceededImport || latestImportStatus(member) != importer.StatusFailed ||
		member.PersonaName == nil || *member.PersonaName != "Alice Persona" {
		t.Errorf("member = %+v, want Alice's profile, counts and import state", member)
	}

	// Steam IDs stay JSON strings.
	var members []map[string]json.RawMessage
	if err := json.Unmarshal(decodeRawJSON(t, recorder)["members"], &members); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	assertJSONString(t, members[0]["steamId"], "76561198000000201")
}

func TestAddGroupMemberIsIdempotent(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	friends := seedGroup(t, env, "Friends")

	requireStatus(t, env.do(t, http.MethodPut, groupMemberPath(friends.ID, groupAlice), ""), http.StatusNoContent)
	requireStatus(t, env.do(t, http.MethodPut, groupMemberPath(friends.ID, groupAlice), ""), http.StatusNoContent)
	if got := groupMembershipCount(t, env, "SELECT COUNT(*) FROM steam_user_group_members WHERE group_id = $1", friends.ID); got != 1 {
		t.Errorf("got %d membership rows, want 1", got)
	}
}

func TestAddGroupMemberErrors(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	friends := seedGroup(t, env, "Friends")

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantError  string
	}{
		{name: "unknown group", path: groupMemberPath(999999, groupAlice), wantStatus: http.StatusNotFound, wantError: "group not found"},
		{name: "unknown user", path: groupMemberPath(friends.ID, groupBob), wantStatus: http.StatusNotFound, wantError: "steam user not found"},
		{name: "non-numeric group id", path: fmt.Sprintf("/api/groups/abc/members/%d", groupAlice), wantStatus: http.StatusBadRequest, wantError: errInvalidGroupID.Error()},
		{name: "zero group id", path: groupMemberPath(0, groupAlice), wantStatus: http.StatusBadRequest, wantError: errInvalidGroupID.Error()},
		{name: "non-numeric steam id", path: fmt.Sprintf("/api/groups/%d/members/alice", friends.ID), wantStatus: http.StatusBadRequest, wantError: errInvalidSteamID.Error()},
		{name: "negative steam id", path: groupMemberPath(friends.ID, -1), wantStatus: http.StatusBadRequest, wantError: errInvalidSteamID.Error()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireErrorMessage(t, env.do(t, http.MethodPut, test.path, ""), test.wantStatus, test.wantError)
		})
	}
	if got := groupMembershipCount(t, env, "SELECT COUNT(*) FROM steam_user_group_members WHERE group_id = $1", friends.ID); got != 0 {
		t.Errorf("got %d membership rows after failed adds, want 0", got)
	}
}

func TestRemoveGroupMember(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	seedSteamUser(t, env, groupBob, "Bob")
	friends := seedGroup(t, env, "Friends", groupAlice, groupBob)

	requireStatus(t, env.do(t, http.MethodDelete, groupMemberPath(friends.ID, groupAlice), ""), http.StatusNoContent)
	if names := groupMemberNames(t, env, "Friends"); !reflect.DeepEqual(names, []string{"Bob"}) {
		t.Errorf("members = %v, want [Bob]", names)
	}
	if updatedAt := groupUpdatedAt(t, env, friends.ID); !updatedAt.After(friends.UpdatedAt) {
		t.Errorf("updatedAt = %v, want after %v", updatedAt, friends.UpdatedAt)
	}

	// Removing a non-member, or an unknown Steam user, is still 204 and doesn't
	// touch the group.
	before := groupUpdatedAt(t, env, friends.ID)
	requireStatus(t, env.do(t, http.MethodDelete, groupMemberPath(friends.ID, groupAlice), ""), http.StatusNoContent)
	requireStatus(t, env.do(t, http.MethodDelete, groupMemberPath(friends.ID, groupCarol), ""), http.StatusNoContent)
	if after := groupUpdatedAt(t, env, friends.ID); !after.Equal(before) {
		t.Errorf("updatedAt changed from %v to %v on a no-op remove", before, after)
	}

	requireErrorMessage(t, env.do(t, http.MethodDelete, groupMemberPath(999999, groupBob), ""), http.StatusNotFound, "group not found")
	requireErrorMessage(t, env.do(t, http.MethodDelete, fmt.Sprintf("/api/groups/abc/members/%d", groupBob), ""), http.StatusBadRequest, errInvalidGroupID.Error())
	requireErrorMessage(t, env.do(t, http.MethodDelete, fmt.Sprintf("/api/groups/%d/members/bob", friends.ID), ""), http.StatusBadRequest, errInvalidSteamID.Error())
}

func TestGroupViewUnavailableMemberCount(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	now := time.Now()
	seedSteamUser(t, env, groupAlice, "Alice")
	seedSteamUser(t, env, groupBob, "Bob")
	seedSteamUser(t, env, groupCarol, "Carol")
	seedImportJob(t, env, groupAlice, importer.StatusSucceeded, now)
	seedImportJob(t, env, groupBob, importer.StatusRunning, now)
	friends := seedGroup(t, env, "Friends")
	for _, steamID := range []int64{groupAlice, groupBob, groupCarol} {
		requireStatus(t, env.do(t, http.MethodPut, groupMemberPath(friends.ID, steamID), ""), http.StatusNoContent)
	}

	recorder := getGroupView(t, env, "Friends")
	requireStatus(t, recorder, http.StatusOK)
	view := decodeJSON[groupViewResponse](t, recorder)
	if view.Group.MemberCount != 3 || view.Group.UnavailableMemberCount != 2 {
		t.Errorf("counts = %d/%d, want 3/2", view.Group.MemberCount, view.Group.UnavailableMemberCount)
	}
	available := map[string]bool{}
	for _, member := range view.Members {
		available[member.Name] = member.HasSucceededImport
	}
	if !reflect.DeepEqual(available, map[string]bool{"Alice": true, "Bob": false, "Carol": false}) {
		t.Errorf("hasSucceededImport by member = %v", available)
	}
}

func TestUserInTwoGroups(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	seedSteamUser(t, env, groupBob, "Bob")
	friends := seedGroup(t, env, "Friends")
	family := seedGroup(t, env, "Family")
	requireStatus(t, env.do(t, http.MethodPut, groupMemberPath(friends.ID, groupAlice), ""), http.StatusNoContent)
	requireStatus(t, env.do(t, http.MethodPut, groupMemberPath(friends.ID, groupBob), ""), http.StatusNoContent)
	requireStatus(t, env.do(t, http.MethodPut, groupMemberPath(family.ID, groupAlice), ""), http.StatusNoContent)

	if names := groupMemberNames(t, env, "Friends"); !reflect.DeepEqual(names, []string{"Alice", "Bob"}) {
		t.Errorf("Friends members = %v, want [Alice Bob]", names)
	}
	if names := groupMemberNames(t, env, "Family"); !reflect.DeepEqual(names, []string{"Alice"}) {
		t.Errorf("Family members = %v, want [Alice]", names)
	}

	// Removing Alice from one group leaves her other membership alone.
	requireStatus(t, env.do(t, http.MethodDelete, groupMemberPath(friends.ID, groupAlice), ""), http.StatusNoContent)
	if names := groupMemberNames(t, env, "Family"); !reflect.DeepEqual(names, []string{"Alice"}) {
		t.Errorf("Family members after removal from Friends = %v, want [Alice]", names)
	}
}

func searchGroups(t *testing.T, env *handlerEnv, name string) []groupSummaryView {
	t.Helper()
	recorder := env.do(t, http.MethodGet, "/api/groups/search?name="+url.QueryEscape(name), "")
	requireStatus(t, recorder, http.StatusOK)
	return decodeJSON[groupSummariesResponse](t, recorder).Groups
}

func groupSummaryNames(groups []groupSummaryView) []string {
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.Name)
	}
	return names
}

func TestSearchGroups(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	seedSteamUser(t, env, groupBob, "Bob")
	seedImportJob(t, env, groupAlice, importer.StatusSucceeded, time.Now())
	friends := seedGroup(t, env, "Friday Friends", groupBob, groupAlice)
	seedGroup(t, env, "Old friends")
	seedGroup(t, env, "Family", groupAlice)

	groups := searchGroups(t, env, "FRIEND")
	if names := groupSummaryNames(groups); !reflect.DeepEqual(names, []string{"Friday Friends", "Old friends"}) {
		t.Fatalf("groups = %v, want [Friday Friends, Old friends]", names)
	}

	friday := groups[0]
	if friday.GroupID != friends.ID || friday.MemberCount != 2 || friday.UnavailableMemberCount != 1 {
		t.Errorf("Friday Friends = %+v, want id %d with counts 2/1", friday.groupView, friends.ID)
	}
	wantMembers := []steamUserLookupView{
		{SteamID: strconv.FormatInt(groupAlice, 10), Name: "Alice", HasSucceededImport: true},
		{SteamID: strconv.FormatInt(groupBob, 10), Name: "Bob", HasSucceededImport: false},
	}
	if !reflect.DeepEqual(friday.Members, wantMembers) {
		t.Errorf("members = %+v, want %+v", friday.Members, wantMembers)
	}

	if groups[1].MemberCount != 0 || groups[1].UnavailableMemberCount != 0 {
		t.Errorf("Old friends counts = %d/%d, want 0/0", groups[1].MemberCount, groups[1].UnavailableMemberCount)
	}

	// The response is flat (groupView fields at the top level), and an empty
	// group has members: [] rather than null.
	recorder := env.do(t, http.MethodGet, "/api/groups/search?name=old", "")
	var raw struct {
		Groups []map[string]json.RawMessage `json:"groups"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(raw.Groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(raw.Groups))
	}
	assertJSONString(t, raw.Groups[0]["name"], "Old friends")
	if string(raw.Groups[0]["members"]) != "[]" {
		t.Errorf("members = %s, want []", raw.Groups[0]["members"])
	}
}

func TestSearchGroupsTreatsWildcardsLiterally(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedGroup(t, env, "100%Valid")
	seedGroup(t, env, "100XValid")
	seedGroup(t, env, "under_score")
	seedGroup(t, env, "underXscore")

	tests := []struct {
		query string
		want  string
	}{
		{query: "%", want: "100%Valid"},
		{query: "_", want: "under_score"},
	}
	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			if names := groupSummaryNames(searchGroups(t, env, test.query)); !reflect.DeepEqual(names, []string{test.want}) {
				t.Fatalf("groups = %v, want only %q", names, test.want)
			}
		})
	}
}

func TestSearchGroupsNoMatchesAndBadRequests(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedGroup(t, env, "Friends")

	recorder := env.do(t, http.MethodGet, "/api/groups/search?name=nobody", "")
	requireStatus(t, recorder, http.StatusOK)
	if got := string(decodeRawJSON(t, recorder)["groups"]); got != "[]" {
		t.Errorf("groups = %s, want []", got)
	}

	requireErrorMessage(t, env.do(t, http.MethodGet, "/api/groups/search", ""), http.StatusBadRequest, "name is required")
	requireErrorMessage(t, env.do(t, http.MethodGet, "/api/groups/search?name=%20%20", ""), http.StatusBadRequest, "name is required")
}

func TestGetGroupsByIDs(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	seedSteamUser(t, env, groupAlice, "Alice")
	seedSteamUser(t, env, groupBob, "Bob")
	seedImportJob(t, env, groupAlice, importer.StatusSucceeded, time.Now())
	friends := seedGroup(t, env, "Friends", groupAlice, groupBob)
	family := seedGroup(t, env, "Family", groupAlice)
	empty := seedGroup(t, env, "Empty")

	// Request order is kept, not name order, and duplicates collapse.
	recorder := env.do(t, http.MethodGet, fmt.Sprintf("/api/groups/by-ids?groupIds=%d,%d,+%d+,%d", friends.ID, empty.ID, family.ID, friends.ID), "")
	requireStatus(t, recorder, http.StatusOK)
	groups := decodeJSON[groupSummariesResponse](t, recorder).Groups
	if names := groupSummaryNames(groups); !reflect.DeepEqual(names, []string{"Friends", "Empty", "Family"}) {
		t.Fatalf("groups = %v, want [Friends Empty Family]", names)
	}
	if groups[0].MemberCount != 2 || groups[0].UnavailableMemberCount != 1 || len(groups[0].Members) != 2 {
		t.Errorf("Friends = %+v, want 2 members with 1 unavailable", groups[0])
	}
	if groups[1].Members == nil || len(groups[1].Members) != 0 {
		t.Errorf("Empty members = %#v, want an empty array", groups[1].Members)
	}
	if len(groups[2].Members) != 1 || groups[2].Members[0].Name != "Alice" || !groups[2].Members[0].HasSucceededImport {
		t.Errorf("Family members = %+v, want only Alice (imported)", groups[2].Members)
	}

	missing := friends.ID + 1000
	requireErrorMessage(t,
		env.do(t, http.MethodGet, fmt.Sprintf("/api/groups/by-ids?groupIds=%d,%d", friends.ID, missing), ""),
		http.StatusNotFound, fmt.Sprintf("group %d not found", missing))
}

func TestGetGroupsByIDsBadRequests(t *testing.T) {
	env := newHandlerEnv(t, defaultResolver())
	tooMany := make([]string, maxGroupIDs+1)
	for index := range tooMany {
		tooMany[index] = strconv.Itoa(index + 1)
	}
	// Duplicates don't count toward the limit.
	dedupedToLimit := append(append([]string{}, tooMany[:maxGroupIDs]...), "1")

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantError  string
	}{
		{name: "missing", query: "", wantStatus: http.StatusBadRequest, wantError: "groupIds is required"},
		{name: "blank", query: "groupIds=%20", wantStatus: http.StatusBadRequest, wantError: "groupIds is required"},
		{name: "blank entry", query: "groupIds=1,,2", wantStatus: http.StatusBadRequest, wantError: "groupIds must not contain blank values"},
		{name: "not a number", query: "groupIds=abc", wantStatus: http.StatusBadRequest, wantError: "groupIds must contain positive numbers"},
		{name: "zero", query: "groupIds=0", wantStatus: http.StatusBadRequest, wantError: "groupIds must contain positive numbers"},
		{name: "negative", query: "groupIds=-1", wantStatus: http.StatusBadRequest, wantError: "groupIds must contain positive numbers"},
		{name: "too many", query: "groupIds=" + strings.Join(tooMany, ","), wantStatus: http.StatusBadRequest, wantError: "at most 50 unique groupIds are allowed"},
		{name: "duplicates within limit", query: "groupIds=" + strings.Join(dedupedToLimit, ","), wantStatus: http.StatusNotFound, wantError: "group 1 not found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := env.do(t, http.MethodGet, "/api/groups/by-ids?"+test.query, "")
			requireErrorMessage(t, recorder, test.wantStatus, test.wantError)
		})
	}
}

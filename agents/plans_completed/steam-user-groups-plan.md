# Steam user groups plan

Status: completed

## Original Prompt

In workspace/steam-decision-engine-dev/ Make a plan to add groups that steam users can be assigned to. Groups should have an id and a name. Steam users can be assigned to any number of groups. The group can be selected in the Find Common Games page. The Find Common Games function should know how many users are in selected groups, ensuring the constraint of minimum and maximum users is kept. If a user is in multiple selected groups or is selected individually while in 1 or more selected groups the user should only count as one distinct user.

A View Group page should be added that matches the functionality of the View User page except with a group instead of a user. The User list of users assigned to the group should appear the same as the Search Users page User list.

Ask any questions about unresolved issues. After all questions are answered, write the plan to docs/plans, breaking it up into distinct and small phases that can be implemented and tested individually.

## Goal

1. Add groups: named collections of Steam users. A group has an `id` and a
   `name`, and a Steam user can belong to any number of groups.
2. Add a **Search Groups** page (partial name search) and a **View Group** page
   (exact name lookup) where groups are created, renamed and deleted, and where
   members are added and removed.
3. Let **Find Common Games** select groups as well as individual users through
   one combined search. The frontend expands the selected groups into distinct
   Steam IDs and enforces the 2–25 distinct-user limit before calling the
   unchanged `POST /api/common-games` endpoint.
4. Optionally, show and manage a user's groups on **View User**.

## Decisions

| Topic                              | Decision                                                                                                                                                                                                                                                     |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Group fields                       | `id` (`BIGINT` identity) and `name`, plus `created_at`/`updated_at`.                                                                                                                                                                                         |
| Group name rules                   | Required and trimmed, 1–64 characters, unique ignoring case. A duplicate returns 409.                                                                                                                                                                        |
| Group size                         | No cap. A group with more than 25 members can exist but can't be added in Common Games.                                                                                                                                                                      |
| Membership                         | Many-to-many. Deleting a group or a Steam user cascades to its memberships.                                                                                                                                                                                  |
| Group management UI                | On View Group: a Create form, Rename and Delete actions, a user search with Add buttons, and a per-row Remove action in the member table.                                                                                                                    |
| View Group lookup                  | `/group?name=...`, exact match ignoring case, like View User.                                                                                                                                                                                                |
| Finding groups                     | A separate **Search Groups** page at `/search-groups`. Partial, case-insensitive match. A name is required, like Search Users.                                                                                                                               |
| View Group content                 | Users only. No apps or apps tabs.                                                                                                                                                                                                                            |
| View Group summary                 | Group name, Group ID, member count, status badge.                                                                                                                                                                                                            |
| Group status badge                 | **Empty** (no members, `none` tone). **Unavailable** (any member lacks a completed import, `failed` tone). Otherwise **Available** (`succeeded` tone).                                                                                                       |
| View Group member table            | Same rows as the Search Users table (avatar, name link, Owned/Wishlist meta with privacy badge, import status badge, expandable details), plus a Remove column. Sort by Name / Last updated / Owned apps / Wishlist apps with Direction. No Wishlist filter. |
| Search Groups table                | Same table style. Group name links to View Group, meta line `Members: N`, status badge, expandable details (Group name, Group ID, Member count). Sort by Name / Member count with Direction. No filters.                                                     |
| Common Games search                | One input, two result sub-sections: **Users**, then **Groups**.                                                                                                                                                                                              |
| Group availability in Common Games | A group shows "Unavailable" (disabled) when any member lacks a completed import, the same as a single user without a completed import.                                                                                                                       |
| Selected groups                    | Groups stay as groups. A **Selected groups** list shows name, member count and Remove. **Selected users** lists only individually added users. The count badge shows the distinct total, e.g. `7 / 25`.                                                      |
| Distinct counting                  | The union of individually selected Steam IDs and all members of selected groups. A user in several selected groups, or also selected individually, counts once.                                                                                              |
| Limit enforcement                  | Frontend only. "Add" on a group is disabled if adding it would push the distinct count over 25. A group whose members are all already selected stays addable, since it adds 0.                                                                               |
| Common Games API                   | Unchanged. It still accepts only `{ steamIds, limit }`. The frontend sends the distinct expanded Steam IDs.                                                                                                                                                  |
| Empty groups                       | Can be selected and add 0 users.                                                                                                                                                                                                                             |
| Unknown groups                     | Group endpoints return 404. The frontend shows an error when a selected or shared group no longer exists.                                                                                                                                                    |
| Shared links                       | `groupIds=...` sits next to `steamIds=...`. Groups are live references: membership is re-fetched when the link loads.                                                                                                                                        |
| Results display                    | Distinct users only. No per-user group attribution.                                                                                                                                                                                                          |
| Mutating HTTP verbs                | Group endpoints use `PATCH`, `PUT` and `DELETE`. The origin check, which today only checks `POST`, is extended to every non-safe method. The CSRF check already covers them.                                                                                 |
| View User groups section           | Included as the last, optional phase.                                                                                                                                                                                                                        |

## Non-goals

- No changes to the `POST /api/common-games` request, response or query.
- No group-level app lists, counts or common-games-by-group endpoint.
- No permissions or per-account ownership of groups (the app has one real
  account).
- No bulk member import, group nesting, or group descriptions.

## API summary

All responses use the existing `httpx.RespondWithJSON` /
`httpx.RespondWithError` shapes. Steam IDs stay JSON strings. Group IDs are
JSON numbers, like `importJobView.ID`.

| Method and path                             | Body / query    | Success                  | Errors                           |
| ------------------------------------------- | --------------- | ------------------------ | -------------------------------- |
| `POST /api/groups`                          | `{ "name" }`    | 201 `{ group }`          | 400 invalid name, 409 name taken |
| `GET /api/groups/by-name`                   | `?name=`        | 200 `{ group, members }` | 400 missing name, 404 not found  |
| `PATCH /api/groups/{id}`                    | `{ "name" }`    | 200 `{ group }`          | 400, 404, 409                    |
| `DELETE /api/groups/{id}`                   | none            | 204                      | 400 bad id, 404                  |
| `PUT /api/groups/{id}/members/{steamId}`    | none            | 204 (idempotent)         | 400, 404 group or user not found |
| `DELETE /api/groups/{id}/members/{steamId}` | none            | 204 (idempotent)         | 400, 404 group not found         |
| `GET /api/groups/search`                    | `?name=`        | 200 `{ groups }`         | 400 missing name                 |
| `GET /api/groups/by-ids`                    | `?groupIds=1,2` | 200 `{ groups }`         | 400 bad ids, 404 any unknown id  |

View shapes (in `internal/api/views.go`):

```go
type groupView struct {
	GroupID                int64     `json:"groupId"`
	Name                   string    `json:"name"`
	MemberCount            int64     `json:"memberCount"`
	UnavailableMemberCount int64     `json:"unavailableMemberCount"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// Used by search and by-ids so Common Games can expand and count members
// without another request.
type groupSummaryView struct {
	groupView
	Members []steamUserLookupView `json:"members"`
}
```

`GET /api/groups/by-name` returns `members` as `[]steamUserSummaryView`, the
same shape Search Users renders. The frontend derives the status badge from
`memberCount` and `unavailableMemberCount`, so the rule lives in one place
(`groupStatus()` in `app.js`).

---

## Phase 1: schema and queries

**Goal:** tables and generated query code. No HTTP changes.

### 1.1 Migration

Add `internal/database/migrations/00003_steam_user_groups.sql`:

```sql
-- +goose Up
CREATE TABLE steam_user_groups (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Group names are unique without regard to case (exact View Group lookup).
CREATE UNIQUE INDEX steam_user_groups_name_lower ON steam_user_groups (lower(name));

CREATE TABLE steam_user_group_members (
    group_id BIGINT NOT NULL,
    steam_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (group_id, steam_id),
    FOREIGN KEY (group_id) REFERENCES steam_user_groups(id) ON DELETE CASCADE,
    FOREIGN KEY (steam_id) REFERENCES steam_users(steam_id) ON DELETE CASCADE
);

-- Reverse lookup for "groups this user belongs to".
CREATE INDEX steam_user_group_members_steam_id ON steam_user_group_members (steam_id);

-- +goose Down
DROP TABLE steam_user_group_members;
DROP TABLE steam_user_groups;
```

The table is called `steam_user_groups` so it isn't confused with `app_users`
(login accounts) or Steam Community groups.

### 1.2 Queries

Add `internal/database/queries/groups.sql`:

- `CreateGroup :one`: insert `name`, `NOW()` timestamps, `RETURNING *`.
- `GetGroup :one`: by `id`.
- `GetGroupByName :one`: `WHERE lower(name) = lower(sqlc.arg(name))`.
- `RenameGroup :one`: `UPDATE ... SET name, updated_at = NOW() WHERE id RETURNING *`.
- `DeleteGroup :execrows`: by `id`.
- `TouchGroup :exec`: `updated_at = NOW()`, called after membership changes.
- `AddGroupMember :exec`: `INSERT ... ON CONFLICT (group_id, steam_id) DO NOTHING`.
- `RemoveGroupMember :execrows`.
- `SearchGroups :many`: `id, name, created_at, updated_at`, plus
    - `member_count`: `COUNT(*)` of members,
    - `unavailable_member_count`: members with no `import_jobs` row where
      `status = 'succeeded'` (the same rule as `has_succeeded_import`),
    - `WHERE strpos(lower(g.name), lower(sqlc.arg(name))) > 0`,
    - `ORDER BY g.name, g.id`.
- `ListGroupsByIDs :many`: the same columns as `SearchGroups`, with
  `WHERE g.id = ANY(@group_ids::bigint[])`.
- `GetGroupCounts :one`: `member_count` and `unavailable_member_count` for one
  `id` (used by View Group).
- `ListGroupMemberLookups :many`: `gm.group_id, su.steam_id, su.name,
has_succeeded_import` for `group_id = ANY(@group_ids::bigint[])`, ordered by
  `group_id, su.name, su.steam_id`. Used to fill `groupSummaryView.Members`.
- `ListGroupMembers :many`: exactly the `SELECT` list and joins of
  `SearchSteamUsers`, but `JOIN steam_user_group_members gm ON gm.steam_id =
su.steam_id WHERE gm.group_id = @group_id`, ordered by `su.name, su.steam_id`.
  Keep the column list and order identical so that
  `dbgen.SearchSteamUsersRow(row)` is a valid struct conversion and
  `newSteamUserSummaryView` can be reused.
- `ListGroupsForSteamUser :many`: `id, name` for one `steam_id`, ordered by
  name. Only used in Phase 11. It can be added then.

Run `sqlc generate`.

### 1.3 Tests and verification

- `sqlc generate` succeeds. `go build ./...` and `go vet ./...` pass.
- `TEST_DATABASE_URL=... go test ./...` passes. `dbtest.Open` runs all
  migrations, so a broken migration fails every DB test.
- Add `internal/api/handler_groups_test.go` with a small direct query test.
  Insert two groups and members, then check `SearchGroups` counts
  (`member_count`, `unavailable_member_count`) and that deleting a Steam user
  or a group cascades its membership rows.

---

## Phase 2: origin check for all unsafe methods

**Goal:** `PATCH`/`PUT`/`DELETE` get the same Origin/Referer protection as
`POST`.

- `internal/httpserver/middleware.go`, `validateRequestOrigin`: replace
  `request.Method != http.MethodPost` with a safe-method check (`GET`, `HEAD`,
  `OPTIONS` pass through). Reuse `isSafeMethod` from `auth.go`, since it's in
  the same package.
- `internal/httpserver/middleware_test.go`: add cases showing that `PUT`,
  `PATCH` and `DELETE` with a foreign Origin get the existing rejection, that
  they pass with the app origin, and that `GET` is unaffected.
- Verify with `go test ./internal/httpserver/...`.

---

## Phase 3: group CRUD API

**Goal:** create, look up, rename and delete groups (no membership changes
yet).

Add `internal/api/group_handler.go`:

- `normalizeGroupName(raw string) (string, error)`: `strings.TrimSpace`, then
  `utf8.RuneCountInString` must be 1–64 (`VARCHAR(64)` counts characters).
  Error: `name must be between 1 and 64 characters`.
- `parseGroupID(raw string) (int64, error)`: positive `int64`. Error:
  `group id must be a positive number`.
- `mapGroupNameError(err)`: a `*pgconn.PgError` with code `23505` and
  constraint `steam_user_groups_name_lower` becomes 409
  `group name is already taken`. This follows `mapCreateAppUserError` in
  `internal/auth/manager.go`.
- Request bodies go through `http.MaxBytesReader` (1 KiB) and
  `json.NewDecoder`. Invalid JSON returns 400 `invalid JSON body`, matching
  `GetCommonGames`.
- Handlers:
    - `CreateGroup`: `POST /api/groups`, returns 201 `{ "group": groupView }` with
      zero counts.
    - `RenameGroup`: `PATCH /api/groups/{id}`. `sql.ErrNoRows` returns 404
      `group not found`. The response includes counts from `GetGroupCounts`.
    - `DeleteGroup`: `DELETE /api/groups/{id}`. 0 rows affected returns 404,
      otherwise 204.
    - `GetGroupView`: `GET /api/groups/by-name?name=`. Returns
      `{ "group": groupView, "members": [] }` for now. Phase 4 fills `members`.
- Register the routes in `internal/httpserver/app.go`. Go 1.22 mux patterns
  give `request.PathValue("id")`.

Tests (`handler_groups_test.go`):

- Create: success, trimmed name, blank name, a 65-character name, invalid
  JSON, and a duplicate that differs only in case (409).
- Lookup: exact match ignoring case, 404, missing name returns 400.
- Rename: success, rename to a different case of its own name succeeds,
  collision with another group returns 409, unknown id returns 404, a
  non-numeric id returns 400.
- Delete: 204, then lookup returns 404. Deleting again returns 404.

---

## Phase 4: membership API

**Goal:** add and remove members, and return members on View Group.

- `AddGroupMember`: `PUT /api/groups/{id}/members/{steamId}`.
    - Parse both path values. Steam IDs use the same rules as `parseSteamIDs`
      (positive `int64`). Factor a `parseSteamID(raw)` helper out of
      `parseSteamIDs` and reuse it.
    - `GetGroup` returns 404 `group not found` if missing. `GetSteamUser`
      returns 404 `steam user not found` if missing.
    - Call `AddGroupMember`, then `TouchGroup`, then return 204. Adding an
      existing member is still 204.
- `RemoveGroupMember`: `DELETE /api/groups/{id}/members/{steamId}`. Return 404
  if the group is missing. Otherwise call `RemoveGroupMember` and return 204,
  touching the group when a row was removed. Removing a non-member is 204.
- `GetGroupView` now calls `GetGroupCounts` and `ListGroupMembers`, and maps
  each row with `newSteamUserSummaryView(dbgen.SearchSteamUsersRow(row))`.

Tests:

- Add a member, then View Group lists them with the same fields as Search
  Users (profile fields, counts, `latestImportStatus`, `hasSucceededImport`).
- Adding the same member twice gives one row. An unknown group or user
  returns 404. Bad ids return 400.
- Remove a member, removing again is still 204, and an unknown group returns 404.
- `unavailableMemberCount` counts members without a succeeded import.
- One user in two groups appears in both.
- Mutating requests go through the CSRF check like other `POST` routes. One
  `httpserver` test should confirm `PUT` without `X-CSRF-Token` returns 403.

---

## Phase 5: group search and by-ids API

**Goal:** endpoints for Search Groups and for Common Games (search and shared
links).

- `SearchGroups`: `GET /api/groups/search?name=`. A blank name returns 400
  `name is required`. Runs `SearchGroups`, then `ListGroupMemberLookups` for
  the returned ids, and builds `[]groupSummaryView` with members grouped by
  `group_id`. Returns `{ "groups": [...] }`. Members are always an array,
  never `null`.
- `GetGroupsByIDs`: `GET /api/groups/by-ids?groupIds=1,2`.
    - `parseGroupIDs`: trim, positive, deduplicated, at least 1, at most 50
      (`at most 50 unique groupIds are allowed`).
    - `ListGroupsByIDs`. If any requested id is missing, return 404
      `group <id> not found`, the same message style as
      `steam user %d not found`.
    - Returns `{ "groups": [...] }` in request order, with members.

Tests:

- Search: partial and case-insensitive, `%`/`_` treated literally (as in
  `TestSearchSteamUsersTreatsWildcardsLiterally`), member lookups and counts,
  an empty group with `members: []`, no matches gives an empty array, and a
  missing name returns 400.
- By-ids: success and ordering, duplicates collapsed, an unknown id returns
  404, and blank, non-numeric or too many ids return 400.

---

## Phase 6: seed data

**Goal:** representative groups for manual testing of later phases.

Add to `cmd/test/seed.sql`, before `COMMIT`. Seeded users with a succeeded
import are SomePlayer, BobTheBuilder, CaseTwin, WishlistOnly and
PrivateWishlist. PlayerTwo (queued), Alice (running), casetwin (failed) and
EmptyUser (none) don't have one.

- 26 bulk users `RaidMember01`–`RaidMember26` (Steam IDs
  `76561198000001001`–`76561198000001026`, generated with `generate_series`). Each gets a
  succeeded `import_jobs` row and owns 2–3 already-seeded multiplayer apps.
- Groups:
    - **Friday Night**: SomePlayer, BobTheBuilder, WishlistOnly. Shows Available.
    - **Co-op Crew**: BobTheBuilder, WishlistOnly, PrivateWishlist, CaseTwin.
      Overlaps Friday Night, so selecting both gives 5 distinct users, not 7.
    - **Not Ready Yet**: SomePlayer, PlayerTwo. Shows Unavailable.
    - **Empty Group**: no members. Shows Empty.
    - **Almost Full**: RaidMember01–RaidMember23. Addable alone, but disabled
      after Friday Night is selected (23 + 3 = 26).
    - **Big Raid**: RaidMember01–RaidMember26. Never addable (26 > 25).
- Update `cmd/test/README.md` to list the seeded groups and the scenario each
  one covers.

Verify: `cmd/test/start.sh` loads without errors, and the Phase 5 search
endpoint returns the expected counts.

---

## Phase 7: frontend refactor of the Search Users table

**Goal:** make the user table builders reusable, with no visible change.

In `web/public/app.js`:

- Rename `buildSearchResultRows(user, index)` to
  `buildSteamUserRows(user, index, options = {})`, where:
    - `options.idPrefix` (default `"search-user-details"`) builds the details
      element id,
    - `options.actionCell`, if given, is a function `(user) => HTMLTableCellElement`
      inserted before the expand cell. The details `colSpan` becomes 4 when it's
      present, otherwise 3.
- Extract the sort from `visibleSearchUsers` into
  `sortSteamUsers(users, sort, direction)`. `visibleSearchUsers` becomes
  filter + `sortSteamUsers`.
- Add a generic `buildExpandToggle(label, detailsID, detailsRow)` used by
  `buildSteamUserRows` and, later, the group rows, so the chevron/aria logic
  isn't copied.

Verify manually: Search Users renders, sorts, filters and expands exactly as
before.

---

## Phase 8: Search Groups page

**Goal:** `/search-groups` lists groups that match a partial name.

- `index.html`:
    - Add a nav link "Search Groups" after "Search Users", and "View Group"
      after "View User".
    - Add a `view-search-groups` section mirroring `view-search`: a form with
      `#search-groups-name`, controls (`#search-groups-sort` with Name / Member
      count, `#search-groups-direction`, Reset), a status line, a count line, and
      a `table.app-table` with headings `Group`, `Status`, and expand.
- `app.js`:
    - Add `["/search-groups", "searchGroups"]` to `ROUTES` and `ROUTE_TITLES`,
      and a `pageLifecycles.searchGroups` entry with a `leave` that aborts the
      in-flight request, like `leaveSearch`.
    - Add `groupStatus(group)`, which returns
      `{ label: "Empty", tone: "none" }` when `memberCount === 0`,
      `{ label: "Unavailable", tone: "failed" }` when
      `unavailableMemberCount > 0`, and `{ label: "Available", tone: "succeeded" }`
      otherwise.
    - `performGroupSearch()`: token, `AbortController` and busy pattern copied
      from `performSearch`. Calls `/api/groups/search`.
    - `buildGroupRows(group, index)`:
        - Name cell with `a.app-cell__name` to `/group?name=<encoded>` with
          `data-route="/group"`, and meta `Members: N`.
        - Status cell with `badge(...)` from `groupStatus`.
        - Expand cell via `buildExpandToggle`.
        - Details: `Group name`, `Group ID`, `Member count`.
    - Sorting: by name (case-insensitive, then id) or `memberCount`, with
      Direction. Default directions are name ascending and member count
      descending, following `SEARCH_SORT_DEFAULT_DIRECTIONS`.
- No CSS changes expected. The existing `.app-table` and `.controls` styles
  apply.

Verify manually with the seed data: search "a" and "raid", check sorting,
expand a row, and follow a name link (View Group arrives in Phase 9; until then
the link lands on the dashboard fallback).

---

## Phase 9: View Group page (read-only)

**Goal:** `/group?name=...` shows the summary and member table.

- `index.html`: add a `view-group` section modeled on `view-user`:
    - A lookup form `#group-name` with a "View group" button and the hint "Exact
      name match, ignoring case. Use Search Groups for partial searches."
    - A status line.
    - `#group-summary` panel: heading "Group" with `#group-status-badge`, and
      `dl#group-summary-details`.
    - `#group-members` panel: heading "Members", sort controls
      (`#group-members-sort`: Name / Last updated / Owned apps / Wishlist apps,
      `#group-members-direction`), a count line, and a table with headings
      `User`, `Status`, `Actions`, and expand. Phase 10 adds the Actions content.
      Until then the column holds an empty cell.
- `app.js`:
    - Add the route `["/group", "group"]`, a title, and lifecycle
      `enterGroup`/`leaveGroup`, mirroring `enterUser`/`leaveUser`.
    - URL state: `name` only. `parseGroupQuery`, `buildGroupURL`, and a
      replace/push helper like `syncUserURL`/`pushUserURL`. Submitting the form
      pushes a new URL.
    - `loadGroup()` calls `/api/groups/by-name`. A 404 shows "No group named
      'X'." with a link to Search Groups, like `showUserNotFound`.
    - `renderGroupSummary(group)`: `Group name`, `Group ID`, `Members`, and the
      badge from `groupStatus`.
    - `renderGroupMembers()`: `sortSteamUsers` over the members, rendered with
      `buildSteamUserRows(user, index, { idPrefix: "group-member-details",
actionCell })`. Shows "No members yet." when empty. Sort changes re-render
      locally without refetching.

Verify manually: open each seeded group (including the 26-member Big Raid
and Empty Group), check that the badge matches the rule, that rows match
Search Users, that sorting works, and that 404 and name-case lookups behave.

---

## Phase 10: View Group management

**Goal:** create, rename and delete groups, and add and remove members, from
View Group.

- **Create:** a small "Create group" form (`#group-create-name`, Create
  button) shown under the lookup form, always visible. `POST /api/groups`.
  On 201, push `/group?name=<created name>`. Show 400/409 messages inline.
- **Rename:** a "Rename" button in the summary panel toggles an inline form
  (input prefilled, Save/Cancel). `PATCH /api/groups/{id}`. On success,
  replace the URL with the new name and re-render the summary. Show 409
  inline.
- **Delete:** a "Delete group" `button--secondary` with a `confirm()` prompt.
  `DELETE /api/groups/{id}`. On 204, push `/search-groups` and show the status
  "Deleted group 'X'." there. If that's awkward, show it on the cleared View
  Group page instead.
- **Add members:** an "Add members" sub-panel with a user search input that
  calls `/api/steamusers/search`. Results use the existing `result-list` /
  `result` card markup (the same as the Common Games search), showing name,
  Steam ID and an import badge. The button reads "Add", or "Added" (disabled)
  if the user is already a member. Any imported or non-imported user can be
  added, since groups aren't restricted. Clicking calls
  `PUT /api/groups/{id}/members/{steamId}` and then reloads the group.
- **Remove member:** the `actionCell` renders a `button--secondary` "Remove"
  with `aria-label="Remove <name> from <group>"`. Clicking calls
  `DELETE .../members/{steamId}` and reloads the group.
- Disable the relevant buttons while a mutation is in flight, using one
  `groupMutationBusy` flag. Abort the in-flight load before reloading.

Verify manually: create, a duplicate name (409), rename (including a case-only
rename), delete, adding a member flips the badge (add PlayerTwo to Friday
Night to get Unavailable), removing returns it, and the add-search shows
"Added" for existing members.

---

## Phase 11: Common Games combined search and group selection

**Goal:** groups can be searched, added and removed, and are counted by
distinct user.

- `index.html`:
    - Relabel the search: label "Search users and groups", hint "Partial,
      case-insensitive match on Steam names and group names."
    - Inside `#common-games-search-results-panel`, add two sub-sections with
      `h3` headings: "Users" (the existing `#common-games-search-results`) and
      "Groups" (`#common-games-group-results`, `ul.result-list`). Hide each
      sub-section when it has no matches.
    - In `#common-games-selected-panel`, retitle the heading to "Selection" (the
      count badge stays) and add a "Selected groups" `ul.selected-users` list
      (`#common-games-selected-groups`) above a "Selected users" sub-heading.
      The empty hint becomes "No users or groups selected yet. Search above and
      add at least two users."
- `app.js` state: add `commonGamesState.selectedGroups: Map<string groupId,
groupSummaryView>` and `commonGamesState.searchGroups: []`.
- Distinct counting:

    ```js
    function commonGamesSelectedSteamIDs(extra = []) {
        const ids = new Set(commonGamesState.selectedUsers.keys());
        for (const group of commonGamesState.selectedGroups.values()) {
            for (const member of group.members) {
                ids.add(String(member.steamId));
            }
        }
        for (const id of extra) {
            ids.add(String(id));
        }
        return ids;
    }
    ```

    Every place that reads `selectedUsers.size` for limits or the badge
    (`renderCommonGamesSelectedUsers`, `syncCommonGamesControls`,
    `buildCommonGamesSearchResult`, `addCommonGamesUser`, `findCommonGames`)
    switches to `commonGamesSelectedSteamIDs().size`. Individually adding a user
    who is already covered by a selected group stays allowed (adds 0) and is
    disabled only when the user is individually selected ("Added").

- Search: `performCommonGamesSearch` fires `/api/steamusers/search` and
  `/api/groups/search` in parallel with one controller and token
  (`Promise.all`). If either fails, show the error and clear both lists. The
  "No matches" message becomes "No users or groups match 'X'."
- `buildCommonGamesGroupResult(group)`, using `result` card markup:
    - Heading: the group name and a badge. For Common Games, "Ready" when
      available (matching `commonGamesUserStatus`), otherwise the
      `groupStatus` label.
    - Meta line: `Members: N`, and when it's addable, `· adds M new` where
      `M = commonGamesSelectedSteamIDs(memberIDs).size - currentSize`.
    - Button (`data-add-group-id`):
        - "Added" (disabled) when already selected.
        - "Unavailable" (disabled, title "A member has no completed import") when
          `unavailableMemberCount > 0`.
        - "Add" (disabled, title "Adding this group would exceed 25 users.") when
          the new distinct size would be over 25.
        - "Add" otherwise. Empty groups and groups that add 0 are enabled.
- `addCommonGamesGroup(group)` and `removeCommonGamesGroup(groupId)` mirror
  the user versions: re-check the rules, then `invalidateCommonGamesResults()`,
  re-render the selection and both search lists, and sync the URL.
- `buildCommonGamesSelectedGroup(group)`: name, `N members`, and a Remove
  button (`data-remove-group-id`, `aria-label="Remove group <name>"`). Wire
  it into the existing delegated click handler next to
  `data-add-steam-id`/`data-remove-steam-id`.
- `findCommonGames`:
    - Build a `Map<steamId, {steamId, name}>` from the individually selected
      users and then the group members (first one wins). Require a size of 2–25
      (show "Select at least 2 distinct users." otherwise).
    - Send `steamIds: [...map.keys()]`. The body shape is unchanged.
    - Store `users: [...map.values()]` in `commonGamesState.results`, so the
      query-user list and per-game owner lists show distinct users without
      group attribution.
- `syncCommonGamesControls`: the submit hint becomes "Select at least 2
  distinct users to continue."

Verify manually with the seed data:

- Searching "a" shows both sub-sections.
- Friday Night plus Co-op Crew gives `5 / 25`. Then adding BobTheBuilder
  individually still gives `5 / 25`.
- Not Ready Yet shows Unavailable. Big Raid is disabled. Almost Full becomes
  disabled after Friday Night is selected, and re-enables when it's removed.
- Empty Group can be added and the count is unchanged.
- Find Common Games sends each distinct Steam ID once (check the request
  body in devtools) and the results list each user once.
- Removing a group keeps an individually selected member.

---

## Phase 12: Common Games shared links with groups

**Goal:** `groupIds` round-trips through the URL as live references.

- `buildCommonGamesURL`: add `groupIds=<comma-separated ids>` when groups are
  selected. `steamIds` keeps only the individually selected users.
- `parseCommonGamesQuery`: parse `groupIds` like `steamIds` (digits only,
  strip leading zeros, deduplicate).
- `enterCommonGames`: drop the up-front 2–25 check on `steamIds.length`. When
  either list is non-empty, call the loader. Clear `selectedGroups` along with
  `selectedUsers`.
- Rename `loadCommonGamesUsersFromURL` to
  `loadCommonGamesSelectionFromURL(steamIDs, groupIDs, submitted)`:
    - Fetch `/api/steamusers/by-ids` (when there are users) and
      `/api/groups/by-ids` (when there are groups) in parallel, under the
      existing users token and controller.
    - A group 404 shows the error "A shared group no longer exists." and selects
      nothing from the link.
    - Groups with `unavailableMemberCount > 0` aren't selected. Count them in
      the "no longer available" message alongside missing users.
    - After restoring, if the distinct count is over 25, show "This shared link
      now includes more than 25 users." and don't auto-submit. If it's under 2,
      keep the existing "not enough imported users" message.
    - Auto-submit when `submitted` is true and everything restored cleanly, as
      today.

Verify manually: share a link with Friday Night plus one user and reload. Add
a member to Friday Night on View Group, then reload the link and see the new
member counted (live reference). Delete a group and reload to get the 404
message. Grow a group past 25 and reload to get the limit message.

---

## Phase 13 (optional): groups on View User

**Goal:** see and edit a user's groups from View User.

- Backend:
    - Add the `ListGroupsForSteamUser` query (if it wasn't added in Phase 1).
    - `steamUserViewResponse` gains `Groups []groupRefView`
      (`{ groupId, name }`), always an array.
    - Test: `GetSteamUserView` returns the user's groups sorted by name, and an
      empty array when there are none.
- Frontend:
    - A new `#user-groups` panel between the summary and Apps panels, with the
      heading "Groups".
    - The list (`ul.selected-users`) shows each group name linking to
      `/group?name=...` with a Remove button
      (`DELETE /api/groups/{id}/members/{steamId}`).
    - "Add to group": a search input that calls `/api/groups/search`. Results
      use the `result` card markup with Add or Added, and Add calls `PUT ...`.
    - Both actions reload the user view. The Apps tab, page and page size are
      preserved, since they live in the URL.

Verify manually: add Alice to Co-op Crew from View User, then open Co-op Crew
and see Alice. Remove her from View User and the group updates.

---

## File change summary

| File                                                       | Phases                                                                        |
| ---------------------------------------------------------- | ----------------------------------------------------------------------------- |
| `internal/database/migrations/00003_steam_user_groups.sql` | 1                                                                             |
| `internal/database/queries/groups.sql`                     | 1, 13                                                                         |
| `internal/database/dbgen/*` (generated)                    | 1, 13                                                                         |
| `internal/httpserver/middleware.go`, `middleware_test.go`  | 2                                                                             |
| `internal/api/group_handler.go`                            | 3, 4, 5                                                                       |
| `internal/api/handler.go` (`parseSteamID` helper)          | 4                                                                             |
| `internal/api/views.go`                                    | 3, 5, 13                                                                      |
| `internal/api/steam_user_handler.go`                       | 13                                                                            |
| `internal/api/handler_groups_test.go`                      | 1, 3, 4, 5                                                                    |
| `internal/api/handler_views_test.go`                       | 13                                                                            |
| `internal/httpserver/app.go` (routes)                      | 3, 4, 5                                                                       |
| `cmd/test/seed.sql`, `cmd/test/README.md`                  | 6                                                                             |
| `web/public/index.html`                                    | 8, 9, 10, 11, 13                                                              |
| `web/public/app.js`                                        | 7–13                                                                          |
| `web/public/styles.css`                                    | Only if a phase needs small tweaks (e.g. `h3` spacing in the combined search) |

## Testing approach

- Each backend phase adds Go tests in `internal/api` (DB-backed, run with
  `TEST_DATABASE_URL=... go test ./...`) and must leave the full suite green.
- There's no JavaScript test harness. Frontend phases are verified manually
  against `cmd/test/start.sh` (or `/start-test-server`) using the Phase 6 seed
  data, following the checklist in each phase.
- After every phase, Search Users, View User and Find Common Games must keep
  their current behaviour except where the phase changes it.

## Risks and notes

- **Live group links are not reproducible.** A shared Common Games link can
  give different results after membership changes. This is intended
  (Decisions: Shared links).
- **Large groups:** there's no cap, and `groupSummaryView.Members` is embedded
  in search results. That's fine at this app's scale. If groups grow very
  large, return only member IDs from search and fetch names lazily.
- **Unavailable is all-or-nothing:** one member with a pending import blocks
  the whole group in Common Games. The user's other members can still be added
  individually.
- **Struct conversion coupling:** `ListGroupMembers` must keep its `SELECT`
  list identical to `SearchSteamUsers`. If the columns ever change, the
  `dbgen.SearchSteamUsersRow(row)` conversion stops compiling, which flags the
  mismatch at build time.
- **Existing Steam user deletion:** `DeleteSteamUser` is still blocked by the
  non-cascading `import_jobs`/library foreign keys. Group memberships cascade
  correctly whenever a user row is actually deleted.
- When everything is complete, move this file to `docs/plans_completed/`.

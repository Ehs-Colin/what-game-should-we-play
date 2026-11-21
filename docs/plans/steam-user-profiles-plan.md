# Steam user profiles and Search Users table plan

Status: proposed

## Goal

1. Make `InsertSteamUser` call Steam's `ISteamUser/GetPlayerSummaries` after
   resolving the vanity URL, and persist the returned profile data.
2. Add `personaname`, `profileurl`, and avatar fields to `steam_users`.
3. Redesign the **Search Users** results as a table that mirrors the **Apps**
   table on **View User**:
   - user name where the app title appears,
   - `Owned: X / Wishlist: Y · Public wishlist` where the release date appears,
   - the import status badge where the price appears,
   - the same chevron expand control and expanded-detail styling.
4. Expanded rows show the stored Steam user name, Steam ID, personaname, and
   profile URL.

## Decisions

| Topic | Decision |
| --- | --- |
| Avatar storage | Store all three Steam avatar sizes: `avatar`, `avatar_medium`, `avatar_full`. |
| `steam_users.name` | Keep the existing meaning: the vanity slug from the import request. `personaname` is stored separately. Search and View User keep using `name`. |
| Summary failure | Fail the request. A transport/API error returns 500; an empty `players` array returns 404 `steam user not found`. No row is saved and no import job is queued. |
| Search table headings | `User`, `Status`, and the unlabeled expand column. |
| Row avatar | Show the user's avatar in the name cell, matching where the Apps table shows the header image. Omit the image when no avatar is stored. |
| Wishlist text | Append `· Public wishlist` or `· Private wishlist` to `Owned: X / Wishlist: Y`. |
| Expanded row | Exactly four fields: Steam user name, Steam ID, personaname, profile URL. Profile URL is a link when valid. |
| View User action | The user name is the link to `/user?name=...`. No separate "View user" button. |
| Profile refresh | Call `GetPlayerSummaries` on every `InsertSteamUser`, update the stored fields, and rely on the existing Steam API cache. No separate backfill command. |
| Status appearance | Reuse the existing status badge component, labels, and tones. |

## Non-goals

- No batch profile refresh command or startup backfill.
- No changes to the View User app table.
- No changes to the Common Games search UI, which keeps the existing
  `result-list` card layout.
- No profile lookup during library imports; only `InsertSteamUser` refreshes
  profile data.
- No login/permission changes.

## 1. Steam API client

### 1.1 Models

Add to `internal/steamapi/steam_user_models.go`:

```go
type GetPlayerSummariesResponse struct {
	Response GetPlayerSummariesResponseData `json:"response"`
}

type GetPlayerSummariesResponseData struct {
	Players []PlayerSummary `json:"players"`
}

type PlayerSummary struct {
	SteamID      string `json:"steamid"`
	PersonaName  string `json:"personaname"`
	ProfileURL   string `json:"profileurl"`
	Avatar       string `json:"avatar"`
	AvatarMedium string `json:"avatarmedium"`
	AvatarFull   string `json:"avatarfull"`
}
```

### 1.2 Handler

Add to `internal/steamapi/steam_user_handler.go`:

```go
func (client *Client) GetPlayerSummaries(
	ctx context.Context,
	steamIDs []string,
) (*GetPlayerSummariesResponse, error)
```

Behaviour:

- Build `https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/`.
- Trim, normalize, deduplicate, and sort the Steam IDs for a stable cache key.
- Reject an empty input with a descriptive error.
- Send `key` and `steamids` (comma-separated) as query parameters.
- Use `client.fetch` with:
  - cache key `v2:player_summaries:<comma-separated IDs>`,
  - `checkStatus(resp, "GetPlayerSummaries")`,
  - `json.Unmarshal` into `GetPlayerSummariesResponse`.
- Return an empty `players` array unchanged; the API handler decides that this
  means "not found".
- Follow the error-message style already used in `player_service.go`.

Although `InsertSteamUser` passes one ID, accepting a slice matches the Steam
endpoint and leaves room for future batch use.

## 2. Database

### 2.1 Migration

Add `internal/database/migrations/00002_steam_user_profiles.sql`:

```sql
-- +goose Up
ALTER TABLE steam_users
    ADD COLUMN personaname VARCHAR(255),
    ADD COLUMN profileurl VARCHAR(255),
    ADD COLUMN avatar VARCHAR(255),
    ADD COLUMN avatar_medium VARCHAR(255),
    ADD COLUMN avatar_full VARCHAR(255);

-- +goose Down
ALTER TABLE steam_users
    DROP COLUMN personaname,
    DROP COLUMN profileurl,
    DROP COLUMN avatar,
    DROP COLUMN avatar_medium,
    DROP COLUMN avatar_full;
```

The columns are nullable so existing rows remain valid. They are populated the
next time each user is submitted through `InsertSteamUser`.

### 2.2 Queries

Update `internal/database/queries/steam_users.sql`:

- `SearchSteamUsers`: also select `su.personaname`, `su.profileurl`,
  `su.avatar`, `su.avatar_medium`, and `su.avatar_full`.
- `UpsertSteamUser`: add the five profile columns to the insert list and to the
  `ON CONFLICT ... DO UPDATE SET` list. Parameters use `sql.NullString`, matching
  the nullable columns and the existing `apps` queries.

`GetSteamUser` and `GetSteamUserByName` already use `SELECT *`, so they pick up
the new columns without query changes. `CreateSteamUser` can remain name-only;
new columns default to `NULL`.

Regenerate with:

```bash
sqlc generate
```

This updates `internal/database/dbgen/models.go`,
`internal/database/dbgen/steam_users.sql.go`, and the `SteamUser`,
`SearchSteamUsersRow`, and `UpsertSteamUserParams` types.

### 2.3 Seed data

Update `cmd/test/seed.sql` so some seeded users have profile data:

- Populate `personaname`, `profileurl`, and all three avatar URLs for most
  users.
- Leave at least one user (`EmptyUser`) with `NULL` profile fields to verify the
  no-avatar and missing-profile fallbacks.
- Use a stable Steam default avatar URL for seeded users that do not need a
  custom image.

## 3. API layer

### 3.1 Store interface

In `internal/api/store.go`, replace `VanityResolver` with an interface that
covers both calls:

```go
type SteamUserClient interface {
	ResolveVanityURL(ctx context.Context, vanityUrl string) (*steamapi.VanityUrl, error)
	GetPlayerSummaries(ctx context.Context, steamIDs []string) (*steamapi.GetPlayerSummariesResponse, error)
}
```

Rename the `Store.steamClient` field type accordingly. `*steamapi.Client` already
satisfies the interface, so `internal/httpserver/app.go` needs no change.

### 3.2 Views

In `internal/api/views.go`, add these fields to both `steamUserView` and
`steamUserSummaryView`:

```go
PersonaName  *string `json:"personaName"`
ProfileURL   *string `json:"profileUrl"`
Avatar       *string `json:"avatar"`
AvatarMedium *string `json:"avatarMedium"`
AvatarFull   *string `json:"avatarFull"`
```

Populate them with the existing `nullableString` helper from
`sql.NullString` values.

Add a small inverse helper for writes:

```go
func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
```

### 3.3 `InsertSteamUser`

Update `internal/api/import_handler.go` after the vanity URL resolves and the
Steam ID is parsed:

```go
summaries, err := handler.apiStore.steamClient.GetPlayerSummaries(
	request.Context(),
	[]string{resolved.Response.SteamID},
)
if err != nil {
	handler.internalError(responseWriter, request, err)
	return
}
if len(summaries.Response.Players) == 0 {
	httpx.RespondWithError(responseWriter, http.StatusNotFound, "steam user not found")
	return
}
player := summaries.Response.Players[0]
```

Then extend the existing `UpsertSteamUser` call:

```go
steamUser, err := handler.apiStore.queries.UpsertSteamUser(
	request.Context(),
	dbgen.UpsertSteamUserParams{
		SteamID:      steamID,
		Name:         vanityUrl,
		PersonaName:  nullString(player.PersonaName),
		ProfileURL:   nullString(player.ProfileURL),
		Avatar:       nullString(player.Avatar),
		AvatarMedium: nullString(player.AvatarMedium),
		AvatarFull:   nullString(player.AvatarFull),
	},
)
```

Ordering stays the same: resolve, fetch profile, upsert, request import. If the
profile fetch fails, no row is written and no job is queued.

`insertSteamUserResponse.SteamUser` automatically carries the new fields because
it uses `steamUserView`.

## 4. Frontend

### 4.1 HTML

In `web/public/index.html`, replace the Search Users result list:

```html
<ul id="search-results" class="result-list" hidden></ul>
```

with a table wrapper that reuses the Apps table classes:

```html
<div id="search-results-panel" class="table-scroll" hidden>
    <table id="search-table" class="app-table">
        <thead>
            <tr>
                <th scope="col">User</th>
                <th scope="col">Status</th>
                <th scope="col" class="app-expand-heading" aria-label="Expand"></th>
            </tr>
        </thead>
        <tbody id="search-results"></tbody>
    </table>
</div>
```

The existing sort/filter controls, status line, and result count stay in place.

### 4.2 JavaScript

In `web/public/app.js`:

- Add a `searchResultsPanel` selector for `#search-results-panel`.
- `renderSearchPipeline`:
  - hide/show `searchResultsPanel` instead of the `<ul>`,
  - build rows with `visible.flatMap((user, index) => buildSearchResultRows(user, index))`.
- `clearSearchResults` clears the `<tbody>` and hides the panel.
- Replace `buildSearchResult` with `buildSearchResultRows(user, index)` returning
  `[row, detailsRow]`, following the structure of `buildUserAppRows`:
  - **Name cell** (`td.app-cell`):
    - avatar image when `user.avatarMedium || user.avatarFull || user.avatar`
      exists; use `img.app-cell__image.app-cell__image--avatar`,
      `loading="lazy"`, `alt=""`,
    - name as an anchor with `href="/user?name=<encoded name>"`,
      `data-route="/user"`, and the existing `app-cell__name` class,
    - meta line with
      `Owned: ${formatCount(user.ownedAppCount)} / Wishlist: ${formatCount(user.wishlistAppCount)} · ${user.wishlistPrivate ? "Private" : "Public"} wishlist`.
  - **Status cell** (`td`):
    - `badge(status.label, status.tone)` using the existing `importStatus`
      mapping and tones.
  - **Expand cell** (`td.app-expand-cell`):
    - same chevron button, `aria-expanded`, `aria-controls`, and show/hide
      labels as the Apps table.
  - **Details row** (`tr.app-details-row` + `td.app-details-cell` with
    `colSpan = 3`):
    - `dl.details.app-details__list` with:
      - `Steam user name` → `user.name`,
      - `Steam ID` → `String(user.steamId)`,
      - `Personaname` → `user.personaName` or `—`,
      - `Profile URL` → a link built with the existing `safeExternalURL`
        helper and opened with `target="_blank" rel="noopener noreferrer"`, or
        `—` when invalid/missing.
- Remove `searchStatNode`, which becomes unused. Keep `searchStat` because the
  Common Games search still uses it.
- Keep the existing sort/filter functions unchanged.

### 4.3 CSS

In `web/public/styles.css`:

- Add an avatar modifier so the square avatar is not stretched by the
  app-image aspect ratio:

```css
.app-cell__image--avatar {
    width: 2.75rem;
    aspect-ratio: 1;
    border-radius: 50%;
    object-fit: cover;
}
```

- Style the name anchor without changing the existing span appearance:

```css
a.app-cell__name {
    color: var(--text);
    text-decoration: none;
}

a.app-cell__name:hover {
    color: var(--accent);
    text-decoration: underline;
}
```

- Reuse `.app-table`, `.app-cell__meta`, `.app-expand*`, `.app-details*`, and
  `.badge` styles. The existing second-column width rule applies to the Status
  column.
- Leave the `.result*` styles in place; they are still used by Common Games.

## 5. Tests

### 5.1 Steam API client

In `internal/steamapi/client_cache_test.go`:

- Add a `player summaries` case to `TestClientCacheKeys`:
  - response with a `players` entry,
  - expected key `v2:player_summaries:76561198000000001`,
  - call `GetPlayerSummaries(context.Background(), []string{steamID})`.
- Add a normalization case for an ID with leading zeros if the cache-key test
  table is extended.
- Add a focused decode test that verifies `personaname`, `profileurl`,
  `avatar`, `avatarmedium`, and `avatarfull` map correctly.
- Add a failure test to `TestClientDoesNotRecordFailures` for a 500 response.

### 5.2 API handlers

In `internal/api/handler_test.go`:

- Extend `fakeResolver` with `GetPlayerSummaries`, configurable profile values,
  an `err`, and an optional empty-players response.
- Update `TestInsertSteamUserNewUser` to assert the returned `steamUser` contains
  the fake personaname, profile URL, and avatar fields.
- Add cases to `TestInsertSteamUserBadRequests`:
  - summary API error → 500,
  - empty `players` → 404,
  - in both cases, assert no import job was created.
- Add a test that a second `InsertSteamUser` refreshes changed profile fields.
- Update `seedSteamUser`/add a helper for seeded profile data, using
  `UpsertSteamUser` with `sql.NullString` values.

In `internal/api/handler_views_test.go`:

- Assert `SearchSteamUsers` returns the new profile fields.
- Assert existing search ordering, counts, and status behavior is unchanged.

### 5.3 Frontend

There is no JavaScript test harness. Verify manually with the project's seeded
test environment:

```bash
/start-test-server
```

## 6. Verification checklist

1. `sqlc generate` succeeds and the generated code compiles.
2. `TEST_DATABASE_URL=... go test ./...` passes.
3. Submit a new vanity URL through **Import Steam User**:
   - the request succeeds,
   - the `steam_users` row has `personaname`, `profileurl`, and all three avatar
     URLs,
   - the response `steamUser` includes the new fields,
   - the import job is still queued exactly as before.
4. Submit the same user again and confirm the profile fields refresh.
5. Confirm a profile-fetch failure leaves no new row/job and returns an error.
6. On **Search Users**:
   - the result is a three-column table matching the Apps table,
   - the name is a link to View User,
   - the avatar appears when present and is omitted when absent,
   - the meta line reads
     `Owned: N / Wishlist: N · Public wishlist` or `· Private wishlist`,
   - the status badge appears in the Status column with its existing colors,
   - expanding shows the four requested fields,
   - sorting and wishlist filtering still work.
7. Confirm **Common Games** search still uses the old card layout.
8. Check narrow widths: the table scrolls horizontally like the Apps table and
   the expanded details remain readable.

## 7. File change summary

| File | Change |
| --- | --- |
| `internal/steamapi/steam_user_models.go` | Add `GetPlayerSummariesResponse` and `PlayerSummary`. |
| `internal/steamapi/steam_user_handler.go` | Add `GetPlayerSummaries`. |
| `internal/steamapi/client_cache_test.go` | Cache-key and decode/failure coverage. |
| `internal/database/migrations/00002_steam_user_profiles.sql` | Add the five nullable profile columns. |
| `internal/database/queries/steam_users.sql` | Extend `SearchSteamUsers` and `UpsertSteamUser`. |
| `internal/database/dbgen/*` | Regenerate with `sqlc generate`. |
| `cmd/test/seed.sql` | Add sample profile data, leaving one user empty. |
| `internal/api/store.go` | Replace `VanityResolver` with `SteamUserClient`. |
| `internal/api/views.go` | Add profile fields and `nullString` helper. |
| `internal/api/import_handler.go` | Fetch summaries and persist profile fields. |
| `internal/api/handler_test.go` | Update fake client and `InsertSteamUser` tests. |
| `internal/api/handler_views_test.go` | Assert search profile fields. |
| `web/public/index.html` | Replace the Search Users list with a table. |
| `web/public/app.js` | Build table rows, status cell, and expanded details. |
| `web/public/styles.css` | Avatar and name-link styles. |

## 8. Risks and notes

- Existing users have `NULL` profile fields until they are submitted through
  **Import Steam User** again. This is intentional; no backfill is included.
- `steam_users.name` remains the vanity slug, so search and View User behavior
  do not change. `personaname` is the Steam display name.
- `GetPlayerSummaries` uses the existing cache and rate limiter, so repeated
  inserts do not add unbounded Steam traffic.
- The migration is additive and has a matching down migration.
- The API JSON keys are camelCase to match the existing view convention:
  `personaName`, `profileUrl`, `avatar`, `avatarMedium`, `avatarFull`.
- The Search Users table reuses the Apps table CSS. If the Apps table styling
  changes later, both tables change together, which is the intended visual
  match.

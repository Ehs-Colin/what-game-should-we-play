# Common games front-end page plan

## Goal

Add a `/common-games` page to the existing vanilla-JS SPA that runs the
`POST /api/common-games` endpoint.

The page lets a user:

1. Search the database for Steam users by partial name.
2. Add search results to a visible selection list, up to 25 users.
3. Choose the maximum number of games to return, from 1 to 100 (API default 10).
4. Press **Find Common Games** once at least two users are selected.
5. See the users included in the query and the ranked list of common games.
6. See each game's name and the percentage of selected users who own or
   wishlist it, with an expandable details panel.

No new framework, build step, database table, or API endpoint is introduced.
The only backend change is one additive field on the existing user-search
response so the page can block users who cannot be queried yet.

```mermaid
sequenceDiagram
    participant U as User
    participant P as Common Games page
    participant H as API
    participant DB as Postgres

    U->>P: Type a name and press Search
    P->>H: GET /api/steamusers/search?name=...
    H->>DB: SearchSteamUsers
    DB-->>H: Users + import metadata
    H-->>P: 200 { steamUsers }
    P-->>U: Results with Add buttons

    U->>P: Add users (max 25)
    P-->>U: Selected users list + updated count

    U->>P: Set limit (1–100), press Find Common Games
    P->>P: Require >= 2 users and a valid limit
    P->>H: POST /api/common-games { steamIds, limit }
    alt Invalid user or no completed import
        H-->>P: 404 { error }
        P-->>U: Error message; results stay hidden
    else Success
        H-->>P: 200 { games: [...] }
        P-->>U: Users in query + ranked game list
        U->>P: Expand a game
        P-->>U: Image, release date, counts, percentages, store link
    end
```

## Resolved decisions

| Topic | Decision |
| --- | --- |
| Page location | New SPA route `/common-games` with a nav entry labelled **Find Common Games**. |
| User search | Explicit submit button, matching the existing **Search Users** page. Reuses `GET /api/steamusers/search`. |
| Eligible users | Add `hasSucceededImport` to the search response. Users without any succeeded import cannot be added. This matches the endpoint's own validation. |
| User selection limit | 25, matching `maxCommonGameUsers`. The client enforces the cap and the API still rejects oversized requests. |
| App limit control | `<input type="number" min="1" max="100" step="1">`, default `10`, matching `defaultCommonGamesLimit` and `maxCommonGamesLimit`. |
| Find button eligibility | Disabled until at least two users are selected; the click handler validates again. |
| Results output | Show the users used in the query and a ranked list of games. Each game row shows name + combined ownership/wishlist percentage. |
| Percentage | `round(totalCommonUsers / selectedUserCount * 100)`. Combined percentage is shown in the collapsed row; exact counts and per-list percentages are in the expanded panel. |
| Expand content | Header image, release date, owned count/percentage, wishlisted count/percentage, total common users, app ID, and a Steam store link. |
| URL state | None. Selected users, limit, and results live in memory and reset on a full page reload. SPA navigation away and back keeps them. |
| Stale results | Any selection or limit change clears existing results. In-flight requests are aborted or ignored by token. |
| Search result filtering | None server-side. All matching users are shown; ineligible users remain visible with a disabled Add button and an explanation. |
| Data rendering | `textContent`/DOM nodes only, never `innerHTML`. Steam IDs stay strings. |

## API contract used

### User search

`GET /api/steamusers/search?name=<partial name>` already returns:

```json
{
  "steamUsers": [
    {
      "steamId": "76561198000000001",
      "name": "someplayer",
      "wishlistPrivate": false,
      "ownedAppCount": 120,
      "wishlistAppCount": 8,
      "latestImportStatus": "succeeded",
      "latestImportFinishedAt": "2026-11-17T12:00:00Z",
      "hasActiveJob": false,
      "updatedAt": "2026-11-17T12:00:00Z"
    }
  ]
}
```

Add one field:

```json
"hasSucceededImport": true
```

This is based on `EXISTS` over all import jobs, not just the latest one, because
`GetCommonGames` accepts a user when **any** import job succeeded.

### Common games

`POST /api/common-games` accepts:

```json
{
  "steamIds": ["76561198000000001", "76561198000000002"],
  "limit": 10
}
```

and returns:

```json
{
  "games": [
    {
      "appId": 730,
      "name": "Counter-Strike 2",
      "headerImage": "https://cdn.akamai.steamstatic.com/steam/apps/730/header.jpg",
      "releaseDate": "21 Nov, 2024",
      "ownedCount": 2,
      "wishlistCount": 1,
      "totalCommonUsers": 2
    }
  ]
}
```

The page does not need any new common-games API work. Existing status codes are
surfaced as-is:

| Status | Page behaviour |
| --- | --- |
| `200` | Render users and games, including an empty game list. |
| `400` | Show the API error message. This indicates a client bug or an invalid limit. |
| `404` | Show the API error message, which names the first unusable user. Keep the selection so the user can remove them. |
| `500` | Show the API error message or a generic failure message. |

## Page layout

Add a fifth section inside `<main class="page">`:

```html
<section id="view-common-games" data-view="/common-games" hidden>
    <div class="card">
        <header class="card__header">
            <p class="eyebrow">Find Common Games</p>
            <h1 tabindex="-1">Find games your group can play together</h1>
            <p class="lede">
                Pick at least two imported Steam users, then find multiplayer games
                they own or want.
            </p>
        </header>

        <!-- 1. User search -->
        <form id="common-games-search-form" class="lookup" novalidate>
            <label for="common-games-search-name">Search Steam users</label>
            <div class="lookup__row">
                <input id="common-games-search-name" name="name" type="text"
                       autocomplete="off" spellcheck="false"
                       placeholder="e.g. gaben" required />
                <button id="common-games-search-button" type="submit">Search</button>
            </div>
            <p class="hint">Partial, case-insensitive match on stored Steam names.</p>
        </form>

        <p id="common-games-search-status" class="status"
           role="status" aria-live="polite"></p>

        <section id="common-games-search-results-panel" hidden
                 aria-labelledby="common-games-search-results-title">
            <h2 id="common-games-search-results-title" class="section-title">Search results</h2>
            <ul id="common-games-search-results" class="result-list"></ul>
        </section>

        <!-- 2. Selected users -->
        <section id="common-games-selected-panel" class="panel"
                 aria-labelledby="common-games-selected-title">
            <div class="section-heading">
                <h2 id="common-games-selected-title" class="section-title">
                    Selected users
                </h2>
                <span id="common-games-selected-count" class="badge">0 / 25</span>
            </div>
            <p id="common-games-selected-empty" class="hint">
                No users selected yet. Search above and add at least two.
            </p>
            <ul id="common-games-selected-list" class="selected-users"></ul>
            <p id="common-games-selected-hint" class="hint" hidden></p>
        </section>

        <!-- 3. Controls -->
        <div class="common-games-controls">
            <label for="common-games-limit">
                Maximum games
                <input id="common-games-limit" type="number"
                       min="1" max="100" step="1" value="10"
                       inputmode="numeric" />
            </label>
            <button id="common-games-submit" type="button" disabled>
                Find Common Games
            </button>
        </div>
        <p class="hint">Choose between 1 and 100 games. The API default is 10.</p>
        <p id="common-games-submit-hint" class="hint">
            Select at least 2 users to continue.
        </p>

        <p id="common-games-status" class="status"
           role="status" aria-live="polite"></p>

        <!-- 4. Results -->
        <section id="common-games-results" hidden
                 aria-labelledby="common-games-results-title"
                 aria-busy="false">
            <div class="section-heading">
                <h2 id="common-games-results-title" class="section-title">Common games</h2>
                <span id="common-games-results-count" class="badge"></span>
            </div>
            <div id="common-games-query-users" class="query-users"></div>
            <p id="common-games-results-empty" class="hint" hidden>
                No common multiplayer games found for these users.
            </p>
            <ul id="common-games-game-list" class="game-list"></ul>
        </section>
    </div>
</section>
```

Add the nav link after **View User**:

```html
<a href="/common-games" data-route="/common-games">Find Common Games</a>
```

Register the route in `app.js`:

```js
const ROUTES = new Map([
    // ...
    ["/common-games", "commonGames"],
]);

const ROUTE_TITLES = {
    // ...
    commonGames: "Find Common Games",
};
```

No `internal/httpserver/app.go` change is needed: the existing SPA catch-all
serves `index.html` for extensionless client-side routes.

## Client state model

Add page-scoped constants and state near the other page state:

```js
const MAX_COMMON_GAME_USERS = 25;
const COMMON_GAMES_LIMIT_MIN = 1;
const COMMON_GAMES_LIMIT_MAX = 100;
const COMMON_GAMES_LIMIT_DEFAULT = 10;

const commonGamesState = {
    selectedUsers: new Map(), // steamId string -> search result object
    searchUsers: [],
    results: null, // { games, users, limit }
};

let commonGamesSearchToken = 0;
let commonGamesSearchController = null;
let commonGamesRequestToken = 0;
let commonGamesRequestController = null;
let commonGamesBusy = false;
```

`selectedUsers` is a `Map` keyed by the string Steam ID. Insertion order is the
display order. The search result object is kept so the selected list can show
name, ID, and import status without another request.

A string key is required because SteamID64 values exceed JavaScript's safe
integer range. Never convert them to `Number`.

## Phase 1: backend search metadata

### Query

Add an `EXISTS` expression to `SearchSteamUsers` in
`internal/database/queries/steam_users.sql`, next to `has_active_job`:

```sql
    EXISTS (
        SELECT 1
        FROM import_jobs succeeded
        WHERE succeeded.steam_id = su.steam_id
          AND succeeded.status = 'succeeded'
    ) AS has_succeeded_import
```

Run `sqlc generate` to add `HasSucceededImport bool` to
`dbgen.SearchSteamUsersRow` and scan it in the generated code.

### View

Add `HasSucceededImport bool` to the JSON view in `internal/api/views.go`:

```go
type steamUserSummaryView struct {
    // ...
    HasSucceededImport bool `json:"hasSucceededImport"`
}
```

Map it in `newSteamUserSummaryView`. No handler change is required because the
handler builds the view from the generated row.

This field is additive and does not change the existing **Search Users** page,
which ignores unknown fields.

## Phase 2: page markup and navigation

Files:

- `web/public/index.html`
- `web/public/app.js`

Changes:

1. Add the nav link and the `view-common-games` section shown above.
2. Add the route and title entries to `ROUTES` and `ROUTE_TITLES`.
3. Add DOM element lookups near the existing page lookups.
4. Add a `commonGames` lifecycle entry to `pageLifecycles`.

Lifecycle rules:

- `enterCommonGames`: re-sync controls and focus is handled by `renderRoute`.
  Nothing is fetched automatically.
- `leaveCommonGames`: abort any in-flight search or common-games request and
  increment both tokens so late responses are ignored. Do **not** clear the
  selection or results; the in-memory state survives SPA navigation.
- A full reload naturally starts with an empty selection and a limit of `10`.

Accessibility requirements:

- Every input has a visible `<label>`.
- Status messages use `role="status"` and `aria-live="polite"`.
- Expand buttons use `aria-expanded` and `aria-controls`, with the detail panel
  having a unique `id`.
- Add/remove/expand buttons use `type="button"` so they never submit a form.
- Remove buttons have an accessible name such as `Remove someplayer`.
- Search result count and selection count are announced through their visible
  text; no duplicate live regions.

## Phase 3: styles

Add page-specific classes to `web/public/styles.css`, reusing existing tokens
and components where possible:

| Class | Purpose |
| --- | --- |
| `.common-games-controls` | Flex/grid row containing the limit input and Find button. |
| `.selected-users` | List of selected user rows. |
| `.selected-user` | Name, Steam ID, badge, and remove button. |
| `.query-users` | "Users in this query" chips above the results. |
| `.query-users__chip` | Small name + ID chip. |
| `.game-list` | Vertical list of game cards. |
| `.game-card` | Header image, name, percentage, expand button. |
| `.game-card__image` | Thumbnail with `aspect-ratio: 460 / 215`, matching app rows. |
| `.game-card__percent` | Prominent percentage badge. |
| `.game-card__details` | Expanded detail grid. |
| `.game-card__detail` | Label/value pair inside the detail grid. |

Implementation notes:

- Reuse existing `.result`, `.badge`, `.button--secondary`, and `.details`
  styling where it fits to keep the page visually consistent.
- The game detail panel is a normal block with the `hidden` attribute; no
  animation is required. Toggling `hidden` plus `aria-expanded` is enough.
- Add responsive rules so the game card stacks the image above the text on
  narrow screens, following the existing `@media (max-width: 30rem)` block.
- Keep visible focus styling through the existing global
  `button:focus-visible`/`input:focus-visible` rules.

## Phase 4: client behaviour

### Search

Reuse the existing search/abort/token pattern from `performSearch`.

```text
on common-games search submit:
  token = ++commonGamesSearchToken
  abort previous search controller
  name = trim(input.value)
  if empty: show "Enter a Steam name."; focus input; return

  set search busy (disable input + Search button)
  show "Searching for <name>..."
  fetch GET /api/steamusers/search?name=<encodeURIComponent(name)>
  on success:
      commonGamesState.searchUsers = body.steamUsers ?? []
      render search results
      if empty: show "No users match <name>."
      else: clear status and show the results panel
  on error:
      clear search results
      show readError(error)
  finally:
      if token current: re-enable controls
```

Search results rendering:

- One `.result` row per user.
- Show name, `Steam ID <id>` as text, owned/wishlist counts, and an import-status
  badge.
- Badge logic:
  - `hasSucceededImport === true` → **Ready** (`succeeded` tone).
  - Otherwise, if `latestImportStatus` exists → show that status with the
    existing `importStatus()` mapping.
  - Otherwise → **Never imported** (`out-of-date` tone).
- Add button states:
  - Already in `selectedUsers` → text **Added**, disabled.
  - `hasSucceededImport !== true` → text **Unavailable**, disabled, with a
    visible hint such as "No completed import". Treating a missing field as
    ineligible keeps the page safe if it is ever served by an older backend.
  - Selection is at 25 → all Add buttons disabled.
  - Otherwise → **Add**, enabled.
- Adding a user keeps the search results visible and re-renders them so the
  new button state is reflected.

### Selection list

```text
addCommonGamesUser(user):
  if selectedUsers.size >= 25: return
  if !user.hasSucceededImport: return
  selectedUsers.set(String(user.steamId), user)
  invalidateCommonGamesResults()
  renderSelectedUsers()
  renderCommonGamesSearchResults()

removeCommonGamesUser(steamId):
  selectedUsers.delete(String(steamId))
  invalidateCommonGamesResults()
  renderSelectedUsers()
  renderCommonGamesSearchResults()
```

`renderSelectedUsers` must:

- Update the count badge to `n / 25`.
- Show the empty message when the map is empty.
- Render one row per selected user in insertion order, with name, Steam ID,
  and a remove button.
- When `n === 25`, show "Selection limit reached (25 users)." and ensure all
  Add buttons are disabled.
- Call `syncCommonGamesControls()`.

`invalidateCommonGamesResults`:

```text
invalidateCommonGamesResults():
  commonGamesRequestToken += 1
  commonGamesRequestController?.abort()
  commonGamesRequestController = null
  commonGamesBusy = false
  commonGamesState.results = null
  clear the results section, its user chips, and its status
  set results aria-busy=false
  syncCommonGamesControls()
```

The token increment prevents an in-flight response from rendering results for
an old selection. Resetting `commonGamesBusy` and calling
`syncCommonGamesControls()` re-enables the Find button immediately; the aborted
request's `finally` block will see a stale token and do nothing.

### Limit control and Find button

- The limit input starts at `10`.
- On `input`, clear any displayed results. Do not silently clamp while typing.
- `parseCommonGamesLimit()` returns `null` unless the trimmed value is an
  integer from 1 to 100.
- On run, an invalid limit shows "Enter a whole number from 1 to 100." and
  focuses the input. Do not call the API.
- `syncCommonGamesControls()` sets:
  - Find button `disabled` when `selectedUsers.size < 2` or a request is in
    flight.
  - Submit hint text:
    - fewer than 2 users → "Select at least 2 users to continue."
    - otherwise → "" (hidden).
  - Limit input disabled only while a request is in flight.

### Find Common Games

```text
findCommonGames():
  userList = [...selectedUsers.values()]
  if userList.length < 2:
      show "Select at least 2 users."; return
  limit = parseCommonGamesLimit()
  if limit is null:
      show "Enter a whole number from 1 to 100."; focus limit; return

  token = ++commonGamesRequestToken
  abort previous request controller
  controller = new AbortController()

  commonGamesBusy = true
  syncCommonGamesControls()
  hide results; set results aria-busy=true
  show "Finding common games..."
  fetch POST /api/common-games with
      { steamIds: userList.map(user => String(user.steamId)), limit }
      headers Accept + Content-Type application/json
      signal: controller.signal

  on success:
      if token stale: return
      body = await response.json()
      commonGamesState.results = {
          games: Array.isArray(body.games) ? body.games : [],
          users: userList,
          limit,
      }
      renderCommonGamesResults()
      show "" (or "Found N games." for zero)
  on non-OK:
      if token stale: return
      show readError(response)
      keep results hidden
  finally:
      if token current:
          commonGamesBusy = false
          syncCommonGamesControls()
          set results aria-busy=false
```

Notes:

- The request body must use the string IDs, not parsed numbers.
- The API deduplicates, but the UI already prevents duplicates, so the selected
  count and `totalCommonUsers` always have the same denominator.
- A `404` most likely means the stored data changed after the search (for
  example, the user lost eligibility). Show the message and keep the selection
  so the user can remove the named user.
- Network failures use the existing `errorMessage()` helper.

### Results rendering

`renderCommonGamesResults` reads `commonGamesState.results` and:

1. Shows the results section.
2. Renders the **users included in this query** as chips, using the `users`
   snapshot taken when the request started. This satisfies the requirement to
   display the list of users alongside the common games.
3. Sets the results count badge to `N games`.
4. If `games.length === 0`, shows the empty-state message and an empty list.
5. Otherwise renders one card per game.

Percentage helpers:

```js
function commonGamesPercent(count, total) {
    if (!total) return 0;
    return Math.round((Number(count) / total) * 100);
}
```

For each game card:

```text
buildCommonGameCard(game, userCount, index):
  <li class="game-card">
    header:
      optional <img class="game-card__image" src=headerImage alt="" loading=lazy>
      <div>
        <p class="game-card__name">{game.name ?? "Unknown app"}</p>
        <p class="game-card__meta">
          {percent(totalCommonUsers)}% · {totalCommonUsers} of {userCount} users
          own or wishlist this game
        </p>
      </div>
      <button type="button"
              aria-expanded="false"
              aria-controls="game-details-{appId}-{index}">
        Show details
      </button>
    </header>
    <div id="game-details-{appId}-{index}" class="game-card__details" hidden>
      optional larger image
      Release date: {releaseDate ?? "—"}
      Owned: {ownedCount} ({percent(ownedCount)}%)
      Wishlisted: {wishlistCount} ({percent(wishlistCount)}%)
      Owned or wishlisted: {totalCommonUsers} ({percent(totalCommonUsers)}%)
      App ID: {appId}
      <a href="https://store.steampowered.com/app/{encodeURIComponent(appId)}/"
         target="_blank" rel="noopener noreferrer">Open on Steam</a>
      <p class="hint">
        A user who owns and wishlists the same game is counted once in the
        combined total, so owned and wishlist percentages can add up to more
        than 100%.
      </p>
    </div>
  </li>
```

Details:

- Use the `index` in the detail panel id because a request can only contain one
  row per app, but an index keeps ids unique even if a future API change returns
  duplicates.
- Toggling the expand button flips `aria-expanded` and the `hidden` attribute of
  the controlled panel, and changes the label between **Show details** and
  **Hide details**.
- Use `textContent` for every API-derived string. Never interpolate into HTML.
- `headerImage` is optional. If it is `null`, omit the image rather than showing
  a broken image.
- `name` is optional. Show **Unknown app** when it is `null`.
- `releaseDate` is expected for every returned game, but fall back to `—`
  defensively.
- The collapsed row intentionally shows only the combined percentage and counts.
  The expanded panel supplies the owned/wishlist split.

### Stale-result and control rules summary

| Action | Effect |
| --- | --- |
| Add or remove a selected user | Hide and discard results; abort in-flight request; re-render lists and controls. |
| Change the limit input | Hide and discard results; abort in-flight request. |
| Start a new user search | Does not affect the selection or common-games results. |
| Leave the route | Abort in-flight search/request; keep state. |
| Reload the browser | State resets to empty selection and limit 10. |
| Add a 26th user | Impossible in the UI; Add buttons disable at 25 and the add function guards the size. |

## Phase 5: tests and verification

### Go tests

Extend `TestSearchSteamUsersLatestImportAndActiveJob` in
`internal/api/handler_views_test.go` with a `wantSucceededImport` column:

| User | Jobs | Expected `hasSucceededImport` |
| --- | --- | --- |
| `QueuedUser` | succeeded then queued | `true` |
| `RunningUser` | running only | `false` |
| `SucceededUser` | succeeded | `true` |
| `FailedUser` | failed only | `false` |
| `MixedUser` | running then failed | `false` |

`QueuedUser` is the important regression case: the latest job is not succeeded,
but an older succeeded job makes the user valid for `GetCommonGames`.

Optionally add a focused test asserting the JSON key is `hasSucceededImport` and
is always present as a boolean.

Run:

```sh
sqlc generate
go test ./...
go vet ./...
```

### Manual verification

```sh
docker compose up -d
go run ./cmd/server
```

Open `http://localhost:3030/common-games` and check:

1. The nav link appears and is marked current; a hard refresh of
   `/common-games` serves the SPA shell.
2. A partial, case-insensitive search returns matching users.
3. A user with no succeeded import shows a disabled **Unavailable** button and a
   reason; a queued user with an older succeeded import remains addable.
4. Adding users updates the selection list and count.
5. Adding 25 users hits the cap, disables remaining Add buttons, and shows the
   limit hint.
6. Removing a user frees a slot and re-enables Add buttons.
7. **Find Common Games** stays disabled with fewer than two users.
8. An empty, `0`, `101`, or fractional limit shows a validation message and does
   not make a request.
9. A valid request returns games ranked by the API, with the selected users
   displayed above them.
10. Each collapsed game row shows its name and combined percentage.
11. Expanding shows the image, release date, owned/wishlist counts and
    percentages, total, app ID, and a working Steam store link; collapsing hides
    it again.
12. Changing the selection or limit immediately clears the results.
13. No matches returns `200` with an empty list and the empty-state message.
14. An API error (for example, remove a user's succeeded import in the database
    after adding them, then run) shows a readable error and no stale games.
15. Double-clicking or re-running does not allow an older response to overwrite
    a newer one.

### API checks

```sh
curl -sS 'http://localhost:3030/api/steamusers/search?name=player'

curl -sS -X POST http://localhost:3030/api/common-games \
  -H 'Content-Type: application/json' \
  -d '{"steamIds":["76561198000000001","76561198000000002"],"limit":10}'
```

## Files touched

| File | Change |
| --- | --- |
| `internal/database/queries/steam_users.sql` | Add `has_succeeded_import` to `SearchSteamUsers`. |
| `internal/database/dbgen/steam_users.sql.go` | Regenerated by `sqlc generate`. |
| `internal/api/views.go` | Add `hasSucceededImport` to `steamUserSummaryView` and map it. |
| `internal/api/handler_views_test.go` | Assert `hasSucceededImport` for the existing import-status fixture. |
| `web/public/index.html` | Add nav link and the `view-common-games` section. |
| `web/public/app.js` | Add route, state, search, selection, request, and rendering logic. |
| `web/public/styles.css` | Add selection, control, query-user, and game-card styles. |

## Edge cases and notes

- **Steam ID precision:** keep IDs as strings end to end. `String(user.steamId)`
  is used as the map key and in the request body.
- **Eligibility is stricter than "latest import succeeded":** use
  `hasSucceededImport`, because the endpoint checks for any succeeded import.
- **Duplicates:** the UI prevents adding the same Steam ID twice; the API also
  deduplicates.
- **Percentage denominator:** use the number of users in the request snapshot,
  not the current selection. The two cannot diverge in practice because results
  are cleared on selection changes, but the snapshot keeps rendering correct if
  that rule changes later.
- **Owned + wishlist overlap:** a user can contribute to both counts while
  counting once in `totalCommonUsers`. The expanded panel explains why the
  per-list percentages can exceed 100% in total.
- **Null fields:** game name and header image are nullable; release date is
  always present for eligible games but still handled defensively.
- **No result pagination:** the API returns at most 100 rows and the page renders
  all of them.
- **No auto-search:** the page makes no request until the user submits a search
  or presses Find Common Games.
- **No URL state:** sharing or refreshing the URL does not reproduce a selection
  or result set.

## Out of scope

- New API endpoints or changes to `GetCommonGames`.
- Changing the common-games ranking, limit validation, or 25-user cap.
- Server-side pagination, sorting, or filtering of search results.
- Selecting users by Steam persona/display name or resolving new users by vanity
  URL; only stored names are searched.
- Adding, importing, or refreshing users from this page.
- Persisting selections in the URL, local storage, or the database.
- Showing which specific users own or wishlist a game.
- Real-time updates when imports finish; the user can search again.
- A JavaScript framework, bundler, or automated browser tests.

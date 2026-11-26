# Import Steam User page: lookup, status, and actions

Status: proposed

Date: 2026-11-25

## Goal

Split the Import Steam User page into three distinct steps:

1. **Look up** a Steam user without changing the database or starting an
   import.
2. **Show the user's import status** when the user is already stored or has an
   import in progress.
3. **Offer the right action**: `Import User` for a user that has never been
   imported, `Update user` for a stale or failed import, and no action while an
   import is running or after a fresh successful import.

The existing import worker, job table, refresh interval, and polling endpoint
stay in place. This change adds a read-only lookup response, a forced-import
path, and the matching UI states.

## Decisions

| Topic                  | Decision                                                                                                                                                              |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Lookup miss            | Search the database first. If the name is unknown, resolve it with Steam **read-only** and show a preview. Nothing is written to `steam_users` and no job is queued.     |
| Renamed vanity         | If the name misses but the resolved Steam ID is already stored under another name, return that row's status. Importing or updating reconciles the stored name.         |
| Staleness              | A user is out of date when the latest successful import is older than `STEAM_REFRESH_INTERVAL`, or when the latest import attempt failed.                              |
| Never imported         | A stored row with no import jobs (or a Steam preview) shows `Import User`. A row whose latest attempt failed shows `Update user`, per the failed-attempt decision below. |
| Failed import          | The action is labelled `Update user` and forces a new import.                                                                                                         |
| Update semantics       | `Update user` always forces a new job when no job is active, bypassing the refresh-interval shortcut.                                                                  |
| Active job             | If a job is already queued or running, `Update user` reuses it instead of creating a duplicate.                                                                       |
| Awaiting import        | A lookup that finds a queued or running job shows its status/progress, polls every 2 seconds, and hides both action buttons until the job finishes.                    |
| Result detail          | Full summary: avatar, Steam name, Steam ID, persona name, source, owned/wishlist counts, last import time/status, and error when present.                             |
| Preview safety         | The UI sends the previewed `steamId` with Import/Update. The server rejects the request with `409` if the name now resolves to a different Steam ID.                  |
| State ownership        | The server computes the import state so the refresh interval is authoritative. The browser does not re-derive staleness from its own clock.                           |

## Current behaviour

- `GET /api/steamusers?name=X` only reads `steam_users` by name and returns
  `404` when the row is absent. It does not return import history.
- `POST /api/steamuser` resolves the vanity name, fetches the Steam profile,
  upserts the user, and calls `Importer.RequestImport`. It returns `202` for a
  new job or `200` for an active or still-fresh succeeded job.
- The page's `search()` calls `GET` first. On `404` it immediately calls `POST`,
  so a lookup miss saves the user and starts an import.
- Polling only starts from a `POST` response. The UI has one status line, a
  basic user panel, and a job panel. There are no Import/Update buttons.

## Desired user flow

### A. Lookup finds a fresh import

1. User enters a name and clicks **Look up**.
2. The page shows the full user summary with an **Up to date** badge.
3. The last import row shows when it succeeded.
4. No action button is shown. The user can look up another name.

### B. Lookup finds a stale or failed import

1. The page shows the summary with an **Out of date** badge.
2. The last import row shows the succeeded date, or the failure and its error.
3. The **Update user** button is shown.
4. Clicking it posts a forced import, shows the job panel, and starts polling.
5. When the job succeeds, the status becomes **Up to date**, the counts refresh,
   and the button disappears.
6. If the job fails, the status becomes **Out of date** and **Update user**
   returns for a retry.

### C. Lookup finds a queued or running import

1. The page shows the summary with **Queued** or **Importing** and the current
   job's progress.
2. Neither action button is shown.
3. The page polls every 2 seconds until the job succeeds or fails.
4. On success it refreshes the summary counts and shows **Up to date**; on
   failure it shows **Out of date** and **Update user**.

### D. Lookup finds a user that has never been imported

1. The page resolves the name with Steam and shows the previewed profile,
   persona name, and avatar.
2. Counts show `—` because they are unknown before the first import.
3. The badge says **Not imported** and the **Import User** button is shown.
4. Clicking it saves the profile, queues the first import, and starts polling.
5. Nothing is written if the user only looks up and never clicks.

### E. Lookup finds nothing

The page shows `Steam user not found. Check the name and try again.` and keeps
the input available for another attempt.

## Import state model

The server returns one of five states. The state controls the badge, status
text, and action.

| State          | Condition                                                                                       | Badge       | Action        |
| -------------- | ----------------------------------------------------------------------------------------------- | ----------- | ------------- |
| `not_imported` | No `steam_users` row, or a row with no import jobs                                              | Not imported | `Import User` |
| `queued`       | An active job with status `queued`                                                              | Queued      | None          |
| `running`      | An active job with status `running`                                                             | Importing   | None          |
| `up_to_date`   | Latest job `succeeded`, `finished_at` is set, and `now - finished_at < refreshInterval`          | Up to date  | None          |
| `out_of_date`  | Latest job `failed`, or latest job `succeeded` but stale or missing `finished_at`                | Out of date | `Update user` |

A successful job that has only just finished is treated as `up_to_date`. A
`failed` latest attempt always wins over an earlier success.

`STEAM_REFRESH_INTERVAL` keeps its current meaning: a non-negative number of
seconds, defaulting to 7 days. A value of `0` means every successful import is
immediately out of date, so `Update user` creates a new job whenever the
worker is idle.

## API changes

### `GET /api/steamusers?name=<vanity>`

Keep the route and the handler name (`GetSteamUserByName`). Extend the response
from `{steamUser}` to a lookup payload.

Database hit:

```json
{
  "steamUser": {
    "steamId": "76561197960287930",
    "name": "gaben",
    "wishlistPrivate": false,
    "personaName": "Gabe Newell",
    "profileUrl": "https://steamcommunity.com/id/gaben/",
    "avatar": "https://cdn.example/avatar.jpg",
    "avatarMedium": "https://cdn.example/avatar_medium.jpg",
    "avatarFull": "https://cdn.example/avatar_full.jpg",
    "createdAt": "2026-11-01T12:00:00Z",
    "updatedAt": "2026-11-01T12:00:00Z"
  },
  "source": "database",
  "importState": "out_of_date",
  "latestImport": {
    "id": 42,
    "steamId": "76561197960287930",
    "status": "succeeded",
    "totalApps": 100,
    "processedApps": 100,
    "skippedApps": 2,
    "failedApps": 0,
    "wishlistTotalApps": 12,
    "wishlistProcessedApps": 12,
    "wishlistSkippedApps": 0,
    "wishlistFailedApps": 0,
    "error": null,
    "createdAt": "2026-11-15T12:00:00Z",
    "startedAt": "2026-11-15T12:00:01Z",
    "finishedAt": "2026-11-15T12:03:00Z"
  },
  "hasActiveJob": false,
  "ownedAppCount": 321,
  "wishlistAppCount": 12
}
```

Steam preview (nothing stored):

```json
{
  "steamUser": {
    "steamId": "76561197960287930",
    "name": "gaben",
    "wishlistPrivate": false,
    "personaName": "Gabe Newell",
    "profileUrl": "https://steamcommunity.com/id/gaben/",
    "avatar": "https://cdn.example/avatar.jpg",
    "avatarMedium": "https://cdn.example/avatar_medium.jpg",
    "avatarFull": "https://cdn.example/avatar_full.jpg"
  },
  "source": "steam",
  "importState": "not_imported",
  "latestImport": null,
  "hasActiveJob": false,
  "ownedAppCount": null,
  "wishlistAppCount": null
}
```

Field notes:

- `source` is `database` when the user was found by name and `steam` when the
  name had to be resolved through Steam (including a rename match by ID).
- `importState` is always present.
- `latestImport` is the active job when the state is `queued` or `running`;
  otherwise it is the latest job, or `null`.
- `hasActiveJob` mirrors whether the returned job is active.
- `ownedAppCount` / `wishlistAppCount` are `null` for a preview because the
  library has not been imported.
- `createdAt` / `updatedAt` are omitted for a preview. The shared
  `steamUserView` should use `*time.Time` with `omitempty` so one view serves
  both cases.
- A `404` still means Steam does not know the name (and the database does not
  have it). A Steam or database error remains `500`.

Lookup algorithm:

1. Trim and validate `name`; empty returns `400`.
2. `GetSteamUserByName`. If found, set `source = "database"` and continue to
   step 5.
3. Resolve the name with Steam read-only (`ResolveVanityURL` then
   `GetPlayerSummaries`). Steam's "no match" or an empty player list is `404`.
4. If the resolved Steam ID exists in `steam_users`, use that row so a renamed
   vanity still reports the correct status. Set `source = "steam"`. Otherwise
   return the preview payload from the player summary with
   `importState = "not_imported"`.
5. Count active owned and wishlist apps and call the importer snapshot.
6. Return the lookup payload.

The preview path must not call `UpsertSteamUser`, `CreateImportJob`, or
`RequestImport`. The Steam client's normal response cache may still be used.

### `POST /api/steamuser`

Extend the request body:

```json
{
  "vanityUrl": "gaben",
  "steamId": "76561197960287930",
  "force": true
}
```

- `vanityUrl` is required, as today.
- `steamId` is optional for backward compatibility, but the Import page always
  sends the ID it is showing.
- `force` defaults to `false`.

Validation and behaviour:

1. Parse an optional `steamId`; an invalid value returns `400`.
2. Resolve the vanity name with the shared Steam helper. If the name does not
   resolve, return `404`.
3. If `steamId` was supplied and differs from the resolved ID, return `409`
   with `The Steam name now resolves to a different user. Look up the user
   again.` Do not write anything.
4. Fetch the player summary and upsert the profile as today.
5. `force == true` calls `RequestForcedImport`; otherwise `RequestImport`.
6. Return `202` with `Location` when a job was created, or `200` when an active
   job (or, for a non-forced request, a still-fresh succeeded job) was reused.
7. Keep the response shape `{steamUser, importJob}` so the page can render the
   returned profile and job directly.

Forced imports must still reuse an existing `queued` or `running` job. Only
the fresh-success shortcut is skipped.

### Importer contract

Add to `internal/importer`:

```go
type ImportState string

const (
    StateNotImported ImportState = "not_imported"
    StateQueued      ImportState = "queued"
    StateRunning     ImportState = "running"
    StateUpToDate    ImportState = "up_to_date"
    StateOutOfDate   ImportState = "out_of_date"
)

type ImportSnapshot struct {
    State        ImportState
    Latest       *dbgen.ImportJob
    HasActiveJob bool
}

func (importer *Importer) Snapshot(ctx context.Context, steamID int64) (ImportSnapshot, error)
func (importer *Importer) RequestForcedImport(ctx context.Context, steamID int64) (dbgen.ImportJob, bool, error)
```

Refactor the existing `RequestImport` into a shared
`requestImport(ctx, steamID, force bool)`:

1. Lock `requestMu`.
2. Return the active job if one exists.
3. If `!force`, return the latest succeeded job when it is still inside the
   refresh interval.
4. Create and enqueue a new job.

`Snapshot` uses `GetActiveImportJobForUser` first, then
`GetLatestImportJobForUser`:

- Active job found → state is its status (`queued` or `running`), latest is that
  job.
- No active job and no history → `not_imported`.
- Latest `succeeded` and fresh → `up_to_date`.
- Latest `succeeded` and stale or without `finished_at` → `out_of_date`.
- Latest `failed` → `out_of_date`.
- A queued/running latest job that races past the active query → report its
  status rather than downgrading it.

The API package imports `internal/importer` for these types. There is no import
cycle: `importer` does not import `api`.

Update `ImportRequester` in `internal/api/import_handler.go` to include
`RequestForcedImport` and `Snapshot`.

## Backend implementation

1. **`internal/importer/importer.go`**
   - Add `ImportState`, `ImportSnapshot`, `Snapshot`, and
     `RequestForcedImport`.
   - Refactor `RequestImport` to use a private `requestImport` with a `force`
     flag.
2. **`internal/api/steam_resolution.go`** (new)
   - Add `var errSteamUserNotFound = errors.New("steam user not found")`.
   - Add `resolveSteamPlayer(ctx, vanityURL) (int64, steamapi.PlayerSummary, error)`.
   - Map Steam's `Success != 1` and an empty player list to
     `errSteamUserNotFound`; wrap transport/API errors for `500`.
   - Reuse this helper from both lookup and insert so the resolution rules stay
     in one place.
3. **`internal/api/steam_user_handler.go`**
   - Replace the body of `GetSteamUserByName` with the lookup algorithm above.
   - Add `steamUserLookupResponse` with `source`, `importState`,
     `latestImport`, `hasActiveJob`, and pointer counts.
   - Add a preview constructor that builds `steamUserView` from the typed name
     and `steamapi.PlayerSummary` with nil timestamps.
   - Count active owned and wishlist apps with the existing
     `CountActiveSteamUserApps` and `CountActiveSteamUserWishlist` queries.
4. **`internal/api/import_handler.go`**
   - Add `SteamID string` and `Force bool` to `insertSteamUserRequest`.
   - Validate the optional ID and enforce the `409` identity check.
   - Call `RequestForcedImport` when `force` is true.
   - Use the shared resolver helper so the insert path cannot drift from the
     lookup path.
5. **`internal/api/views.go`**
   - Change `steamUserView.CreatedAt` / `UpdatedAt` to `*time.Time` with
     `omitempty`.
   - Keep `newSteamUserView` returning pointers to the timestamps.
6. **`internal/httpserver/app.go`**
   - No route changes. The existing `GET /api/steamusers` and
     `POST /api/steamuser` routes now return the extended payloads.

## Frontend implementation

### HTML (`web/public/index.html`, `#view-import`)

Keep the lookup form. Replace the current result panel contents with:

- A heading row with the section title and
  `<span id="result-status-badge" class="badge" data-status="none"></span>`.
- A details list containing:
  - avatar row (`#result-avatar-row`, `#result-avatar`), hidden when the
    profile has no avatar;
  - Steam name (`#result-name`);
  - Steam ID (`#steam-id`);
  - persona name (`#result-persona`);
  - source (`#result-source`);
  - owned apps (`#result-owned`);
  - wishlist apps (`#result-wishlist`);
  - last import (`#result-last-import`).
- A conditional `<p id="result-import-error" class="error" hidden>`.
- An action row:
  ```html
  <div id="result-actions" class="panel__actions" hidden>
      <button id="import-button" type="button">Import User</button>
      <button id="update-button" type="button" hidden>Update user</button>
  </div>
  ```

Keep the existing job panel and its progress/skipped/failed/error fields. It is
shown for a newly started job and for a job found by lookup; after completion it
stays visible until the next lookup so the user sees the result.

### CSS (`web/public/styles.css`)

- Add a small `.panel__actions` flex row with wrapping and a top margin.
- Reuse `.badge` tones: `queued`, `running`, `succeeded`, `out-of-date`, and
  `none`.
- Reuse the existing avatar styling (`.user-summary__avatar`) or add a
  result-specific class if the size needs to differ.
- Confirm the action row wraps cleanly on the existing narrow breakpoint.

### JavaScript (`web/public/app.js`)

Add DOM references for the new elements and a
`let currentLookup = null` variable that stores the last lookup payload plus
the normalized typed name.

**`search()`**

- Abort the previous lookup and polling, bump the token, and reset the view.
- `GET /api/steamusers?name=...`.
- `200` → `applyLookup(body, token)`.
- `404` → `Steam user not found. Check the name and try again.`
- Other statuses → existing `readError` handling.
- Do **not** fall back to `POST` on `404`.

**`applyLookup(body, token, options = {})`**

- Store `currentLookup` and the normalized name.
- Render the summary: avatar, names, ID, source text, counts (`—` for `null`),
  last import, and the status badge.
- Render the state:
  - `not_imported` → show `Import User`, hide `Update user`, hide the job panel.
  - `out_of_date` → show `Update user`, hide `Import User`, hide the job panel
    unless `options.keepJob` is set.
  - `queued` / `running` → hide both buttons, render `latestImport`, and call
    `startPolling(latestImport, token)`.
  - `up_to_date` → hide both buttons and the job panel unless
    `options.keepJob` is set.
- Set the status text from the state and the latest job (see below).

**`startImport(force)`**

- Guard on `currentLookup`.
- Use the normalized typed name and `currentLookup.steamUser.steamId`.
- `setBusy(true)` and disable the action buttons.
- `POST /api/steamuser` with `{vanityUrl, steamId, force}`.
- On success:
  - update `currentLookup.steamUser` from the response;
  - set `source = "database"`;
  - set `latestImport = body.importJob`;
  - set `importState` from the job status;
  - render the summary and state (which hides both buttons);
  - render the job panel;
  - if the job is terminal, run the terminal handler; otherwise call
    `startPolling(body.importJob, token)`.
- `409` → show the server message and ask the user to look up again.
- `404` → show the server message.
- Other errors → existing `readError` handling.
- Always re-enable the controls in `finally`.

**`stateFromJob(job)`**

- `queued` → `queued`
- `running` → `running`
- `succeeded` → `up_to_date`
- `failed` → `out_of_date`

**Polling (`startPolling`, `pollOnce`)**

- Keep the 2-second interval and 5-minute deadline.
- On each successful poll, update `latestImport`, the job panel, and the
  summary's last-import row.
- On `succeeded`:
  - set the state to `up_to_date`, hide the action buttons, and show
    `Import complete.`;
  - silently re-fetch the lookup payload and merge the refreshed counts and
    state while keeping the completed job panel visible (`options.keepJob`).
- On `failed`:
  - set the state to `out_of_date`, show `Update user`, and show
    `Import failed. Choose Update user to retry.`;
  - show `job.error` in the existing job error element.
- On timeout, keep the current status and show
  `The import is taking longer than expected. Look up the user again to see its status.`
  Do not show the action buttons while the state is still active.
- On route exit, abort polling as today. `enterImport()` continues to resume
  polling when `lastJob` is active.

**Status text**

- `not_imported`: `Not in the database yet. Check the Steam profile, then choose Import User.`
- `queued`: `Import queued.`
- `running`: `Import in progress…`
- `up_to_date`: `Up to date.`
- `out_of_date` with a failed latest job:
  `The last import failed. Choose Update user to try again.`
- `out_of_date` with a stale success:
  `The last successful import is out of date. Choose Update user to refresh it.`

**`resetView()`**

- Clear `currentLookup`, the badge, all detail fields, the error row, the
  avatar `src`, both action buttons, and the job panel.
- Keep the existing status reset.

**`enterImport()` / `leaveImport()`**

- No behaviour change beyond clearing the new state in `resetView()`.
- `enterImport()` still resumes polling when returning to the page with an
  active `lastJob`.

## Edge cases

- **Rapid clicks / two tabs**: `RequestImport` and `RequestForcedImport` share
  `requestMu`; a second request returns the active job instead of creating a
  duplicate.
- **Force while a job is running**: the active job is reused and the page polls
  it. No second job is queued.
- **Name changes between preview and click**: the `steamId` check returns
  `409`, so the wrong account is never imported. The user looks up again.
- **Renamed vanity**: a lookup by the new name resolves to a stored Steam ID
  and reports that row's status. Importing or updating upserts the typed name,
  so the stored name is reconciled without changing the import history.
- **Stored name no longer resolves**: a DB hit still shows its status, but
  `Update user` returns `404`. This is an existing Steam-side limitation; the
  user can look up the current name (which resolves by ID) and update from
  there.
- **Latest success without `finished_at`**: classified as `out_of_date`.
- **`STEAM_REFRESH_INTERVAL=0`**: every success is immediately stale; the page
  offers `Update user` whenever no job is active.
- **Private profile**: imports can succeed with no games. The summary shows the
  job's counts and status; wishlist privacy is already handled by the importer.
- **Lookup while the previous lookup is in flight**: the token/abort guard
  prevents stale responses from rendering.
- **Polling timeout**: the job may still be running server-side; a new lookup
  picks it up again.

## Testing plan

### Automated

Importer (`internal/importer/importer_test.go`):

- `Snapshot` returns `not_imported` with no jobs.
- `Snapshot` returns `queued` and `running` for active jobs.
- `Snapshot` returns `up_to_date` for a success inside the interval.
- `Snapshot` returns `out_of_date` for a stale success and for a failed latest
  job.
- `Snapshot` returns `out_of_date` when a success has no `finished_at`.
- `RequestForcedImport` creates a job when a fresh success exists.
- `RequestForcedImport` reuses an active job.
- `RequestImport` keeps its existing fresh-success shortcut.

API (`internal/api/handler_test.go` and helpers):

- Database hit returns `source: "database"`, counts, `latestImport`, and the
  expected state.
- Steam preview returns `source: "steam"`, `not_imported`, nil counts, and
  creates no `steam_users` row and no import job.
- Unknown name returns `404`.
- A DB hit still does not call Steam (preserve
  `TestGetSteamUserByNameDoesNotCallSteam`).
- Renamed vanity: DB miss, resolver returns a stored ID, response uses that
  row's state.
- `POST` with `force: true` creates a new job when a fresh success exists.
- `POST` with `force: true` and an active job reuses it.
- `POST` with a mismatched `steamId` returns `409` and writes nothing.
- Existing insert, job, and view tests continue to pass.

Commands:

```sh
go vet ./...
TEST_DATABASE_URL=postgres://wgswp:change-me@localhost:5432/wgswp?sslmode=disable go test ./...
```

### Manual

Run the app against the development database and exercise:

1. **Fresh import**: look up an up-to-date user → badge, counts, last-import
   date, no buttons.
2. **Stale import**: age the latest succeeded job past the refresh interval →
   `Update user`; click it → job panel, polling, success, refreshed counts.
3. **Failed import**: set the latest job to `failed` with an error → badge,
   error text, `Update user`; retry → success.
4. **Active import**: start an import, then look up the same name in another
   tab → status and progress, no buttons, polling completes.
5. **Never imported**: look up a Steam name that is not in the database →
   preview with persona/avatar, `—` counts, `Import User`; click it → profile
   saved and import starts.
6. **Unknown name**: look up a name Steam does not know → clear error, no rows
   created.
7. **Rapid clicks**: double-click `Update user` → exactly one active job.
8. **Navigation**: leave the page during an import and return → polling resumes
   for the active job.
9. **Accessibility**: keyboard-only lookup, Import, and Update; status changes
   are announced through the existing live region; badges have text labels.
10. **Narrow layout**: the action row and details wrap without overlap.

Useful SQL for manual state setup:

```sql
-- Make the latest job for a user stale.
UPDATE import_jobs
SET finished_at = NOW() - INTERVAL '30 days'
WHERE id = (
    SELECT id FROM import_jobs
    WHERE steam_id = 76561198000000001
    ORDER BY created_at DESC, id DESC
    LIMIT 1
);

-- Make the latest job fail.
UPDATE import_jobs
SET status = 'failed', error = 'manual test failure', finished_at = NOW()
WHERE id = (
    SELECT id FROM import_jobs
    WHERE steam_id = 76561198000000001
    ORDER BY created_at DESC, id DESC
    LIMIT 1
);
```

## Acceptance criteria

- Looking up a user never writes to `steam_users` and never queues a job.
- A stored user's lookup response includes their import state, latest job, and
  owned/wishlist counts.
- An up-to-date user shows status and no action button.
- A stale or failed user shows `Update user`, which always starts a new import
  when no job is active.
- A never-imported user shows a Steam preview and `Import User`, which saves the
  profile and starts the import.
- A queued/running user shows progress, polls automatically, and hides both
  actions until the job finishes.
- A name that resolves to a different Steam ID than the one being imported is
  rejected with `409`.
- `go vet ./...` and the full Go test suite pass.
- The manual checks above pass at desktop and narrow widths.

## Out of scope

- Authentication/authorisation changes for the import endpoints. The existing
  `TODO::Require user login with write permissions` remains.
- Cancelling or pausing a running import.
- Automatic background refreshes of existing users.
- A general "refresh profile" action that does not import the library.
- Changing the import worker, rate limiting, or job schema.
- Changing the View User page's existing status badge behaviour.

## Implementation checklist

- [ ] Add `ImportState`, `ImportSnapshot`, `Snapshot`, and
      `RequestForcedImport` to `internal/importer`.
- [ ] Refactor `RequestImport` through a shared force-aware helper.
- [ ] Add the shared Steam resolution helper and `errSteamUserNotFound`.
- [ ] Extend `GET /api/steamusers` with state, latest job, source, and counts.
- [ ] Add the read-only Steam preview and the renamed-vanity fallback.
- [ ] Extend `POST /api/steamuser` with `steamId` and `force`, including the
      `409` identity check.
- [ ] Make `steamUserView` timestamps optional for previews.
- [ ] Add the result summary, badge, error row, and action buttons to
      `index.html`.
- [ ] Add `.panel__actions` styling.
- [ ] Rewrite the import page JS for lookup, state rendering, forced imports,
      and polling refreshes.
- [ ] Add importer and API tests, including preview, force, and `409` cases.
- [ ] Run `go vet ./...` and the database-backed test suite.
- [ ] Complete the manual state matrix at desktop and narrow widths.

# Admin Panel import jobs list plan

Status: completed

Date: 2026-11-27

## Goal

Add a second box below the Admin Panel's "At a Glance" box that lists every
import job. Each row follows the Groups search-result pattern:

- The Steam user name appears where the group name appears, with the Steam ID
  underneath.
- A new column immediately left of the status shows
  `imported/total`, combining owned and wishlisted apps.
- The status column uses the job's actual state:
  `Queued`, `Running`, `Succeeded`, or `Failed`.
- The expand button opens a full breakdown of the job.

The list refreshes with the existing 5-second Admin Panel timer.

## Resolved decisions

| Topic               | Decision                                                                                                      |
| ------------------- | ------------------------------------------------------------------------------------------------------------- |
| Rows shown          | Every `import_jobs` row, newest first. A user can appear more than once.                                      |
| Imported count      | `processedApps + wishlistProcessedApps`                                                                       |
| Total count         | `totalApps + wishlistTotalApps`                                                                               |
| Status wording      | `Queued`, `Running`, `Succeeded`, `Failed` with the existing `queued`, `running`, `succeeded`, `failed` tones |
| Sorting and paging  | Newest first, client-side pages of 10/25/50 rows, no sort selectors                                           |
| Refresh             | Reloaded by the existing 5-second dashboard timer                                                             |
| Expanded details    | Full job breakdown, including owned and wishlist counts and the error text when present                       |

## Data already available

No schema migration is needed. `import_jobs` already stores:

- `status`, `total_apps`, `processed_apps`, `skipped_apps`, `failed_apps`
- `wishlist_total_apps`, `wishlist_processed_apps`, `wishlist_skipped_apps`,
  `wishlist_failed_apps`
- `error`, `created_at`, `started_at`, `finished_at`

`steam_users` already stores the display name. The only missing piece is a
list query and an HTTP endpoint.

---

## Phase 1: Backend list query and endpoint

### 1.1 Add the SQL query

Add to `internal/database/queries/import_jobs.sql`:

```sql
-- name: ListImportJobs :many
SELECT
    import_jobs.id,
    import_jobs.steam_id,
    import_jobs.status,
    import_jobs.total_apps,
    import_jobs.processed_apps,
    import_jobs.skipped_apps,
    import_jobs.failed_apps,
    import_jobs.wishlist_total_apps,
    import_jobs.wishlist_processed_apps,
    import_jobs.wishlist_skipped_apps,
    import_jobs.wishlist_failed_apps,
    import_jobs.error,
    import_jobs.created_at,
    import_jobs.started_at,
    import_jobs.finished_at,
    steam_users.name AS steam_user_name
FROM import_jobs
JOIN steam_users ON steam_users.steam_id = import_jobs.steam_id
ORDER BY import_jobs.created_at DESC, import_jobs.id DESC;
```

Run `sqlc generate`. This creates `dbgen.ListImportJobsRow` and
`Queries.ListImportJobs`.

The join is an inner join because `import_jobs.steam_id` has a foreign key to
`steam_users`.

### 1.2 Add the response view

In `internal/api/views.go`, add a list item view that promotes the existing
`importJobView` fields and adds the joined name:

```go
type importJobListView struct {
    importJobView
    SteamUserName string `json:"steamUserName"`
}

func newImportJobListView(row dbgen.ListImportJobsRow) importJobListView {
    return importJobListView{
        importJobView: newImportJobView(dbgen.ImportJob{
            ID:                    row.ID,
            SteamID:               row.SteamID,
            Status:                row.Status,
            TotalApps:             row.TotalApps,
            ProcessedApps:         row.ProcessedApps,
            SkippedApps:           row.SkippedApps,
            FailedApps:            row.FailedApps,
            WishlistTotalApps:     row.WishlistTotalApps,
            WishlistProcessedApps: row.WishlistProcessedApps,
            WishlistSkippedApps:   row.WishlistSkippedApps,
            WishlistFailedApps:    row.WishlistFailedApps,
            Error:                 row.Error,
            CreatedAt:             row.CreatedAt,
            StartedAt:             row.StartedAt,
            FinishedAt:            row.FinishedAt,
        }),
        SteamUserName: row.SteamUserName,
    }
}
```

Embedding keeps the JSON shape flat: all existing job fields plus
`steamUserName`.

### 1.3 Add the handler

In `internal/api/dashboard_handler.go`:

```go
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

    jobs := make([]importJobListView, 0, len(rows))
    for _, row := range rows {
        jobs = append(jobs, newImportJobListView(row))
    }
    httpx.RespondWithJSON(responseWriter, http.StatusOK, importJobsResponse{ImportJobs: jobs})
}
```

Initialising `jobs` with `make(..., 0, ...)` makes an empty list serialize as
`[]`, not `null`.

### 1.4 Register the route

In `internal/httpserver/app.go`, add the exact route before the `{id}` route:

```go
dynamicMux.HandleFunc("GET /api/import-jobs", apiHandler.ListImportJobs)
dynamicMux.HandleFunc("GET /api/import-jobs/{id}", apiHandler.GetImportJob)
```

Go's `ServeMux` prefers the exact pattern, so both routes work. The existing
`requireAuth` middleware already protects this endpoint.

### 1.5 Add the route to the test environment

In `internal/api/handler_test.go`, register the new route in
`newHandlerEnv`:

```go
mux.HandleFunc("GET /api/import-jobs", handler.ListImportJobs)
```

### 1.6 Tests

Add a handler test, for example `TestListImportJobs` in
`internal/api/handler_views_test.go`:

- No jobs returns `200` with `{"importJobs":[]}`.
- Two users with jobs at different `created_at` values return newest first.
- Each item includes the correct `steamUserName` from the join.
- Owned and wishlist counters survive serialization.
- A failed job includes its `error` text.
- A queued job serializes its zero counters and null timestamps.

Use the existing `seedSteamUser`, `seedImportJob`, and `env.do` helpers. Use a
direct `UPDATE import_jobs` to set the counter columns when needed.

### Phase 1 done when

- `sqlc generate` has been run and `dbgen` is updated.
- `GET /api/import-jobs` returns all jobs, newest first.
- `go test ./...` passes, including the new handler test when
  `TEST_DATABASE_URL` is set.

---

## Phase 2: Admin Panel markup and styles

### 2.1 Wrap the Admin Panel cards in a stack

The current dashboard section has one `.card`. Change it to a stack so the new
box sits below the At a Glance box with consistent spacing:

```html
<section id="view-dashboard" data-view="/admin-panel" hidden>
    <div class="card-stack">
        <div class="card">
            <!-- existing At a Glance card unchanged -->
        </div>

        <div class="card">
            <!-- new Import jobs card -->
        </div>
    </div>
</section>
```

Do not put `display: grid` directly on `#view-dashboard`, because it would
override the `hidden` attribute's `display: none`.

### 2.2 Add the Import jobs card

Place it after the existing At a Glance card, inside the new `.card-stack`.
Use the Groups search-result structure:

```html
<div class="card">
    <header class="card__header">
        <p class="eyebrow">Import jobs</p>
        <h2 class="section-title">Import jobs</h2>
        <p class="lede">
            Every import job, newest first. Expand a row for details.
        </p>
    </header>

    <p
        id="import-jobs-status"
        class="status"
        role="status"
        aria-live="polite"
    ></p>

    <div id="import-jobs-toolbar" class="app-toolbar" hidden>
        <p id="import-jobs-count" class="hint"></p>
        <label>
            Rows per page
            <select id="import-jobs-page-size">
                <option value="10">10</option>
                <option value="25">25</option>
                <option value="50">50</option>
            </select>
        </label>
    </div>

    <div id="import-jobs-results-panel" class="table-scroll" hidden>
        <table id="import-jobs-table" class="app-table">
            <thead>
                <tr>
                    <th scope="col">Steam user</th>
                    <th scope="col">Imported / Total</th>
                    <th scope="col">Status</th>
                    <th
                        scope="col"
                        class="app-expand-heading"
                        aria-label="Expand"
                    ></th>
                </tr>
            </thead>
            <tbody id="import-jobs-results"></tbody>
        </table>
    </div>

    <nav
        id="import-jobs-pagination"
        class="pagination"
        aria-label="Import job pages"
    ></nav>
</div>
```

The table column order is intentional: user, imported/total, status, expand.
This puts the count immediately left of the status, as requested.

### 2.3 Styles

Add to `web/public/styles.css`:

```css
.card-stack {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 1.5rem;
}

.import-job-count {
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
}
```

Everything else reuses the existing `app-table`, `app-cell`, `app-toolbar`,
`pagination`, `badge`, `details`, and `app-expand` classes.

### Phase 2 done when

- The Admin Panel shows a second box below At a Glance.
- The box has the table shell, toolbar, and pagination container.
- With no JavaScript data yet, the table and toolbar remain hidden.
- The page still lays out correctly on narrow screens because the table uses
  the existing `.table-scroll` wrapper.

---

## Phase 3: Frontend loading, rendering, and interaction

### 3.1 Selectors and state

Near the existing dashboard selectors in `web/public/app.js`, add:

```js
const importJobsStatus = document.querySelector("#import-jobs-status");
const importJobsToolbar = document.querySelector("#import-jobs-toolbar");
const importJobsCount = document.querySelector("#import-jobs-count");
const importJobsPageSize = document.querySelector("#import-jobs-page-size");
const importJobsResultsPanel = document.querySelector(
    "#import-jobs-results-panel",
);
const importJobsResults = document.querySelector("#import-jobs-results");
const importJobsPagination = document.querySelector("#import-jobs-pagination");
```

Near the dashboard state variables, add:

```js
let importJobs = [];
let importJobsPage = 1;
let importJobsRowsPerPage = 10;
const expandedImportJobs = new Set();
```

`expandedImportJobs` keeps expanded rows open across the 5-second refresh,
keyed by `String(job.id)`.

### 3.2 Load jobs with the dashboard

Keep the existing single refresh timer. In `loadDashboard`, fetch both
endpoints in parallel and render each section independently, so a failure in
one does not blank the other:

```js
const [statsResult, jobsResult] = await Promise.allSettled([
    fetchDashboardStats(controller.signal),
    fetchImportJobs(controller.signal),
]);
```

Extract the existing `/api/dashboard` fetch into `fetchDashboardStats` and add:

```js
async function fetchImportJobs(signal) {
    const response = await apiFetch("/api/import-jobs", {
        headers: { Accept: "application/json" },
        signal,
    });
    if (!response.ok) {
        throw await readError(response);
    }
    return response.json();
}
```

Rendering rules:

- If `statsResult` fulfils, call `renderDashboard(statsResult.value)`.
- If `jobsResult` fulfils, set `importJobs = sortImportJobs(body.importJobs)`
  and call `renderImportJobs()`.
- If either rejects and the request was not aborted, show the existing
  dashboard error message and set the import-jobs status to
  `"Could not load import jobs; retrying…"`.
- On a jobs error, keep the previously rendered rows visible rather than
  clearing them.
- When there are no jobs, call `clearImportJobs()` and show
  `"No import jobs yet."` in `#import-jobs-status`.

Do not schedule a separate timer. `scheduleDashboardRefresh()` continues to
drive both sections every 5 seconds.

### 3.3 Sorting and paging

The server already orders newest first, but sort defensively in the browser:

```js
function sortImportJobs(jobs) {
    return [...jobs].sort((left, right) => {
        const byCreated =
            new Date(right.createdAt ?? 0) - new Date(left.createdAt ?? 0);
        if (byCreated !== 0) {
            return byCreated;
        }
        return countValue(right.id) - countValue(left.id);
    });
}
```

Add a page-size listener:

```js
importJobsPageSize.addEventListener("change", () => {
    importJobsRowsPerPage = searchPageSizeFromValue(importJobsPageSize.value);
    importJobsPage = 1;
    renderImportJobs();
});
```

`renderImportJobs` mirrors `renderGroupSearchResults`:

```js
function renderImportJobs() {
    if (importJobs.length === 0) {
        clearImportJobs();
        return;
    }

    importJobsPageSize.value = String(importJobsRowsPerPage);
    const totalPages = totalPageCount(importJobs.length, importJobsRowsPerPage);
    importJobsPage = clampPage(importJobsPage, totalPages);
    const start = (importJobsPage - 1) * importJobsRowsPerPage;
    const pageJobs = importJobs.slice(start, start + importJobsRowsPerPage);

    importJobsResults.replaceChildren(
        ...pageJobs.flatMap((job, index) => buildImportJobRows(job, index)),
    );
    importJobsToolbar.hidden = false;
    importJobsResultsPanel.hidden = false;
    importJobsCount.hidden = false;
    importJobsCount.textContent = `Page ${importJobsPage} of ${totalPages} · ${formatCount(
        importJobs.length,
    )} ${importJobs.length === 1 ? "job" : "jobs"}`;
    renderPagination(
        importJobsPagination,
        importJobsPage,
        totalPages,
        goToImportJobsPage,
    );
}
```

`goToImportJobsPage` follows `goToSearchGroupsPage` and calls
`renderImportJobs()`.

### 3.4 Row builder

Add `buildImportJobRows(job, index)`:

```js
function buildImportJobRows(job, index) {
    const jobID = String(job.id ?? "unknown");
    const row = document.createElement("tr");
    row.className = "app-row";

    const nameCell = newElement("td", "app-cell");
    const body = newElement("div", "app-cell__body");
    const info = newElement("div", "app-cell__info");

    const nameLink = newElement("a", "app-cell__name", job.steamUserName ?? "");
    nameLink.href = `/user?name=${encodeURIComponent(job.steamUserName ?? "")}`;
    nameLink.dataset.route = "/user";

    info.append(
        nameLink,
        newElement("p", "app-cell__meta", `Steam ID: ${job.steamId ?? ""}`),
    );
    body.append(info);
    nameCell.append(body);

    const countCell = newElement("td", "import-job-count");
    countCell.textContent = `${formatCount(importedAppCount(job))}/${formatCount(
        totalAppCount(job),
    )}`;

    const status = importJobStatus(job.status);
    const statusCell = newElement("td");
    statusCell.append(badge(status.label, status.tone));

    const detailsID = `import-job-details-${jobID}-${index}`;
    const detailsRow = document.createElement("tr");
    detailsRow.className = "app-details-row";
    detailsRow.hidden = true;

    const detailsCell = newElement("td", "app-details-cell");
    detailsCell.colSpan = 4;

    const details = newElement("div", "app-details");
    details.id = detailsID;
    details.append(buildImportJobDetails(job));
    detailsCell.append(details);
    detailsRow.append(detailsCell);

    const expandCell = buildExpandToggle(
        job.steamUserName ?? "import job",
        detailsID,
        detailsRow,
        {
            expanded: expandedImportJobs.has(jobID),
            onToggle: (expanded) => {
                if (expanded) {
                    expandedImportJobs.add(jobID);
                } else {
                    expandedImportJobs.delete(jobID);
                }
            },
        },
    );

    row.append(nameCell, countCell, statusCell, expandCell);
    return [row, detailsRow];
}
```

The Steam name links to the existing View User page, matching how the group
name links to the Group page.

### 3.5 Preserve expanded rows across refresh

Extend the shared `buildExpandToggle` with optional state, keeping existing
callers unchanged:

```js
function buildExpandToggle(label, detailsID, detailsRow, options = {}) {
    const toggle = newElement("button", "button--secondary app-expand");
    toggle.type = "button";

    const expanded = options.expanded === true;
    toggle.setAttribute("aria-expanded", String(expanded));
    toggle.setAttribute("aria-controls", detailsID);
    toggle.setAttribute(
        "aria-label",
        expanded ? `Hide details for ${label}` : `Show details for ${label}`,
    );
    detailsRow.hidden = !expanded;

    // ...existing chevron creation...

    toggle.addEventListener("click", () => {
        const nextExpanded = toggle.getAttribute("aria-expanded") !== "true";
        toggle.setAttribute("aria-expanded", String(nextExpanded));
        toggle.setAttribute(
            "aria-label",
            nextExpanded
                ? `Hide details for ${label}`
                : `Show details for ${label}`,
        );
        detailsRow.hidden = !nextExpanded;
        options.onToggle?.(nextExpanded);
    });

    // ...existing expand cell return...
}
```

Without this, the 5-second refresh would collapse an expanded row.

### 3.6 Status and count helpers

Add a job-specific status helper rather than changing the existing
`importStatus`, which is used by user lists:

```js
function importJobStatus(status) {
    switch (status) {
        case "queued":
            return { label: "Queued", tone: "queued" };
        case "running":
            return { label: "Running", tone: "running" };
        case "succeeded":
            return { label: "Succeeded", tone: "succeeded" };
        case "failed":
            return { label: "Failed", tone: "failed" };
        default:
            return { label: "Unknown", tone: "unknown" };
    }
}

function importedAppCount(job) {
    return (
        countValue(job.processedApps) +
        countValue(job.wishlistProcessedApps)
    );
}

function totalAppCount(job) {
    return countValue(job.totalApps) + countValue(job.wishlistTotalApps);
}
```

A queued job shows `0/0` until the worker records the totals. That is expected
and matches the chosen "job progress" definition.

### 3.7 Expanded details

Add `buildImportJobDetails(job)` using the existing `detailRow` helper:

```js
function buildImportJobDetails(job) {
    const details = newElement("dl", "details app-details__list");
    details.append(
        detailRow("Job ID", String(job.id ?? "")),
        detailRow("Steam user name", job.steamUserName ?? ""),
        detailRow("Steam ID", String(job.steamId ?? "")),
        detailRow("Status", importJobStatus(job.status).label),
        detailRow("Created", formatDateTime(job.createdAt) || "—"),
        detailRow("Started", formatDateTime(job.startedAt) || "—"),
        detailRow("Finished", formatDateTime(job.finishedAt) || "—"),
        detailRow("Owned apps total", formatCount(job.totalApps)),
        detailRow("Owned apps processed", formatCount(job.processedApps)),
        detailRow("Owned apps skipped", formatCount(job.skippedApps)),
        detailRow("Owned apps failed", formatCount(job.failedApps)),
        detailRow("Wishlist apps total", formatCount(job.wishlistTotalApps)),
        detailRow(
            "Wishlist apps processed",
            formatCount(job.wishlistProcessedApps),
        ),
        detailRow("Wishlist apps skipped", formatCount(job.wishlistSkippedApps)),
        detailRow("Wishlist apps failed", formatCount(job.wishlistFailedApps)),
    );
    if (job.error) {
        details.append(detailRow("Error", job.error));
    }
    return details;
}
```

### 3.8 Clearing and status helpers

Add:

```js
function clearImportJobs() {
    importJobs = [];
    importJobsResults.replaceChildren();
    importJobsResultsPanel.hidden = true;
    importJobsToolbar.hidden = true;
    importJobsCount.textContent = "";
    importJobsCount.hidden = true;
    importJobsPagination.replaceChildren();
}

function setImportJobsStatus(message, state) {
    importJobsStatus.textContent = message;
    if (state) {
        importJobsStatus.dataset.state = state;
    } else {
        delete importJobsStatus.dataset.state;
    }
}
```

`clearImportJobs` must not clear `expandedImportJobs` on every refresh, or the
expanded state is lost. Clear it only when leaving the dashboard, if desired.

### Phase 3 done when

- Opening `/admin-panel` loads the list from `/api/import-jobs`.
- Rows show the Steam user name, Steam ID, imported/total, and status.
- Rows are newest first and paginate at 10/25/50 per page.
- Expanding a row shows the full job breakdown.
- Expanded rows stay open across the 5-second refresh.
- The list updates alongside the At a Glance metrics.
- Empty and error states are visible and accessible.

---

## Phase 4: Verification and polish

### 4.1 Automated checks

- `sqlc generate` has no uncommitted drift after regeneration.
- `go test ./...` passes.
- Run the database-backed tests with `TEST_DATABASE_URL` set so the new
  query and handler tests actually execute.
- `go vet ./...`.

### 4.2 Manual checks

1. Start the server and open `/admin-panel`.
2. With no jobs, confirm "No import jobs yet." appears and the table is hidden.
3. Import a new Steam user and confirm:
   - A queued row appears within 5 seconds.
   - The row changes to Running and the imported/total count increases.
   - The row changes to Succeeded when the job finishes.
4. Expand a running job and confirm the details update on refresh without the
   row collapsing.
5. Confirm owned and wishlist counts both contribute to the displayed
   imported/total number.
6. Force a failed job and confirm the Failed badge and error text.
7. Confirm pagination appears only when needed, and changing the page size
   resets to page 1.
8. Confirm the Steam name links to `/user?name=...` and the Steam ID is shown
   underneath.
9. Check the layout at a narrow viewport; the table should scroll
   horizontally rather than overflow the page.

### 4.3 Edge cases to confirm

- A queued job displays `0/0` until totals are known.
- A job with an empty error does not render an Error row.
- A job whose user name contains spaces or special characters links
  correctly through `encodeURIComponent`.
- Refreshing while on a later page keeps the page when possible and clamps to
  the last page when the list shrinks.
- A failed dashboard request does not clear a previously loaded job list.

### 4.4 Out of scope

- Server-side pagination or a cap on the number of jobs returned. The endpoint
  returns all jobs, matching the Groups search pattern. Revisit if the job
  history grows into the thousands.
- Filtering the list by status, user, or date.
- Push updates (SSE/WebSocket). The list uses the existing 5-second polling.
- Changing the existing user-list `importStatus` wording. The new
  `importJobStatus` helper is separate.

---

## Completed record

### Files changed

- `internal/database/queries/import_jobs.sql` — `ListImportJobs :many`.
- `internal/database/dbgen/import_jobs.sql.go` — regenerated.
- `internal/api/views.go` — `importJobListView` + `newImportJobListView`.
- `internal/api/dashboard_handler.go` — `importJobsResponse` + `ListImportJobs`.
- `internal/httpserver/app.go` — `GET /api/import-jobs` before the `{id}` route.
- `internal/api/handler_test.go` — the route registered in `newHandlerEnv`.
- `internal/api/handler_views_test.go` — `TestListImportJobsEmptyIsArray`,
  `TestListImportJobsNewestFirst`, and the counter/timestamp seed helpers.
- `web/public/index.html` — `.card-stack` wrapper plus the Import jobs card.
- `web/public/styles.css` — `.card-stack` and `.import-job-count`.
- `web/public/app.js` — selectors, state, the two fetchers inside
  `loadDashboard`, sorting, paging, row and details builders, status helpers,
  and the optional `options` argument on `buildExpandToggle`.

### One deviation from the plan

`.card-stack` also needs `grid-template-columns: minmax(0, 1fr)`, as written in
Phase 2.3 above. With the default `auto` track the grid sized the column to the
table's `min-width: 36rem`, so at a 380px viewport the page overflowed
horizontally (`scrollWidth` 636) instead of the table scrolling inside
`.table-scroll`. The Phase 2 snippet was updated to match the shipped rule.

### Verification

- `sqlc generate` is idempotent: regenerating produced byte-identical files.
- `go test ./...` and `go test -race` (api, httpserver, importer) pass with
  `TEST_DATABASE_URL` pointed at a throwaway Postgres 18 cluster, so
  `TestListImportJobs*` execute rather than skip. `go vet ./...` and `gofmt` are
  clean.
- Anonymous `GET /api/import-jobs` returns 401; authenticated it returns
  `{"importJobs":[]}` when empty and every job newest first with
  `steamUserName`, counters, `error` and timestamps after seeding.
- The browser list was driven in headless Chrome against the running server:
  40 checks covering every manual item in 4.2 and every edge case in 4.3, each
  DOM assertion compared against a live `/api/import-jobs` response.
- A real importer run (server restarted with a queued job and a dummy Steam
  key) produced `queued -> running -> failed` with the error text
  `get owned games: GetOwnedGames returned status 401`, which the list renders
  as a Failed badge plus an Error row. The `running -> succeeded` leg is
  covered by the importer tests and by fixture rows, not by a live Steam
  import.

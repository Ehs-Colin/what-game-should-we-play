# Import Steam User by Steam ID with optional custom username

Status: proposed

Date: 2026-11-25

## Goal

Let the Import Steam User page add users who have **no custom vanity URL** by
importing them with their **SteamID64**. While keeping the existing vanity flow
unchanged:

1. Input in the single lookup field is auto-detected: a bare SteamID64 or a
   `steamcommunity.com/profiles/<id>` link is treated as an ID; anything else
   stays a vanity name.
2. An **optional custom username** can be supplied alongside the Steam ID to
   identify the user inside the app. When it is blank, the SteamID64 digits
   become the stored name.
3. A new **`custom_username` flag** marks users whose stored name is an
   app-assigned label rather than a real Steam vanity.
4. After the profile row exists, the import path is **identical** to today's:
   same upsert, same `RequestImport`/`RequestForcedImport`, same jobs, states,
   polling, and 409 preview-safety. Only the vanity-resolution step is skipped
   and the Steam ID is taken from the entered value.

## Decisions

| Topic              | Decision                                                                                                                                                                                                 |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ID detection       | Auto-detect: the trimmed input that parses as a positive int64 (after stripping the `/profiles/` URL form) is a SteamID64; everything else is a vanity name. No mode toggle.                             |
| Custom username    | Optional for ID imports. Blank falls back to the canonical SteamID64 digits as the name. Existing rows keep their current name when re-imported by ID without a label.                                   |
| Flag semantics     | `custom_username = true` means the stored `name` is app-assigned (label or digits fallback). A real vanity import sets it back to `false`.                                                               |
| Name uniqueness    | Enforced case-insensitively **for custom usernames only**: a partial unique index on `lower(name) WHERE custom_username`, plus a friendly pre-check. Vanity imports keep today's no-uniqueness behavior. |
| Lookup by ID       | `GET /api/steamusers` accepts the same auto-detected ID/URL input: stored row by ID first, then stored name, then a read-only Steam summary preview. Nothing is written.                                 |
| Vanity wins        | Importing later by a real vanity name replaces the custom label, clears the flag, and keeps the import history (jobs are keyed by `steam_id`).                                                           |
| Import engine      | No changes to `internal/importer`. It already works from `steam_id` alone.                                                                                                                               |
| Profile validation | The ID is validated by fetching `GetPlayerSummaries` read-only; an empty player list is `404 steam user not found` for both lookup and import, so fake IDs never create rows.                            |
| Label with vanity  | `customUsername` sent with a resolvable `vanityUrl` is ignored: the vanity is the name and the flag is cleared.                                                                                          |

## Current behaviour

- `POST /api/steamuser` requires `vanityUrl`; resolution (`ResolveVanityURL` →
  `GetPlayerSummaries`) is the only way a Steam ID enters the system. The upsert
  stores `name = vanityUrl` and never touches `custom_username` (the column does
  not exist yet).
- `GET /api/steamusers?name=X` matches by name, then resolves the name as a
  vanity with read-only Steam calls; numeric input goes through the same path
  and fails to resolve.
- `steam_users.name` has a non-unique `lower(name)` index; case-twin names are
  allowed and name lookups break ties by newest `updated_at`.
- The Import page has one text field; `normalizeName` strips only the
  `https://steamcommunity.com/id/` prefix.
- `internal/api/handler_groups_test.go` asserts at compile time that
  `dbgen.ListGroupMembersRow` and `dbgen.SearchSteamUsersRow` have identical
  columns, because both feed `steamUserSummaryView`.

## Desired user flow

### A. Import a user by Steam ID with a custom username

1. User pastes `76561198000000333` (or a profiles URL) into the same field.
2. The page shows a preview: ID-derived name, persona and avatar from the
   profile summary, counts `—`, badge **Not imported**.
3. A **Custom username (optional)** field appears; the user types `RaidBuddy`.
4. Clicking **Import User** saves the row with `name = RaidBuddy`,
   `custom_username = true`, queues the first import, and polls to completion.
5. The source row reads `Stored in the database (custom username)`.

### B. Import by Steam ID without a custom username

Same as A, but the label stays blank: the stored name is the canonical
`76561198000000333` digits. Re-importing the same ID with a blank label keeps
whatever name the row already has.

### C. Look up a stored custom-username user

Typing the custom username, the Steam ID, or the profiles URL all resolve to
the stored row with its import state, latest job, counts, and the same
Import/Update rules as vanity users.

### D. A vanity import replaces a custom username

If the user later creates a Steam vanity and someone imports by it (or looks
it up and clicks Update/Import with the vanity name), the stored name becomes
the vanity and the flag is cleared. Import history is untouched.

### E. Invalid input

- Non-existent or non-numeric-but-numeric-looking IDs (no Steam profile) →
  `Steam user not found. Check the name and try again.` on lookup; `404` on
  import; nothing written.
- A custom username already used by another custom-named user →
  `That username is already used by another user. Choose a different one.`
  with no row or job written.

## Data model

### Migration `00004_steam_user_custom_username.sql`

```sql
-- +goose Up
ALTER TABLE steam_users
ADD COLUMN custom_username BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX steam_users_custom_name_unique
ON steam_users (lower(name))
WHERE custom_username;

-- +goose Down
DROP INDEX IF EXISTS steam_users_custom_name_unique;
ALTER TABLE steam_users DROP COLUMN custom_username;
```

The partial index is the race-proof backstop: two concurrent ID imports that
pick the same label cannot both commit. Existing rows default to `false`, so
today's vanity users are unaffected, and case-twin vanity rows (the seeded
`CaseTwin`/`casetwin`) stay legal because the index only covers custom names.

### Query changes (`internal/database/queries/`, then regenerate sqlc)

- `UpsertSteamUser`: add `custom_username = @custom_username` to both the
  INSERT columns and the ON CONFLICT UPDATE.
- `SearchSteamUsers`: add `su.custom_username` to the SELECT list.
- `ListGroupMembers` (in `groups.sql`): add `su.custom_username` **in the same
  column position** — the compile-time row-equality assertion in
  `handler_groups_test.go` fails the build otherwise.
- `GetSteamUser`, `GetSteamUserByName`, `UpdateSteamUser`, profile columns:
  `SELECT *`-based queries pick the column up automatically.

## API changes

### `GET /api/steamusers?name=<name-or-id>`

Same route and handler. The trimmed value is auto-detected:

```
parseSteamID(value) succeeds → ID path, otherwise vanity path
```

ID path order (all read-only, nothing ever written):

1. `GetSteamUserByName` — a stored row whose name happens to be all digits
   (custom label fallback or a numeric vanity) must still win.
2. `GetSteamUser(parsedID)` — renamed-vanity-style hit by ID; report that row
   with `source = "database"`.
3. Read-only `GetPlayerSummaries([id])` → empty list is `404`; otherwise the
   preview payload, with `steamUser.name` set to the canonical ID digits and
   `customUsername = false` (nothing chosen yet).

Vanity path is unchanged (name → Steam resolve → renamed-ID fallback →
preview). The lookup response `steamUser` object gains the field below.

### `POST /api/steamuser`

```json
{
    "vanityUrl": "",
    "steamId": "76561198000000333",
    "customUsername": "RaidBuddy",
    "force": false
}
```

Rules:

1. At least one of `vanityUrl` / `steamId` is required → `400` otherwise.
   `vanityUrl` (when present) and `steamId` (when present) must each be valid;
   invalid `steamId` is `400` as today.
2. **Vanity path** (`vanityUrl` non-empty): unchanged, except the upsert now
   also sets `custom_username = false` (vanity wins, decision table). The
   resolved-vs-sent `steamId` mismatch keeps returning `409`.
   `customUsername` is ignored here.
3. **ID path** (only `steamId`):
    - Canonicalize: `steamID = parseSteamID(steamId)`; use `strconv.FormatInt`
      for anything stored or returned as a name.
    - Fetch `GetPlayerSummaries([steamID])`; empty → `404 steam user not found`.
      No `ResolveVanityURL` call happens on this path.
    - Resolve the effective name and flag:
        - `customUsername` provided → trim, validate length (≤ 255, reuse the
          existing name rules), pre-check
          `GetSteamUserByCustomName (lower(name) = $1 AND custom_username)` →
          `409` if it matches a **different** steam_id;
          `name = customUsername`, `custom_username = true`.
        - blank + stored row exists → keep that row's `name` and
          `custom_username`.
        - blank + new row → `name = canonicalIDDigits`, `custom_username = true`.
    - Upsert profile fields exactly as the vanity path does.
4. Request the import with `force ? RequestForcedImport : RequestImport`,
   then respond `202` + `Location` (created) or `200` (reused) with the same
   `{steamUser, importJob}` shape.

### Views

- `steamUserView` gains `CustomUsername bool \`json:"customUsername"\`` — the
  single shared view carries it for lookup, import response, and View User.
- `steamUserSummaryView` (Search Users rows and group members) gains the same
  field; the JS badge is derived from it.
- New lookup-only query needed for the uniqueness pre-check:

```sql
-- name: GetSteamUserByCustomName :one
SELECT * FROM steam_users
WHERE custom_username AND lower(name) = lower(sqlc.arg(name))
LIMIT 1;
```

## Phases

Each phase is independently implementable, testable, and shippable; later
phases depend on earlier ones.

### Phase 1 — Schema and data layer

**Scope:** migration + sqlc regeneration only. No behavior changes.

- [x] Add `00004_steam_user_custom_username.sql` (above).
- [x] Update `UpsertSteamUser`, `SearchSteamUsers`, `ListGroupMembers`, and add
      `GetSteamUserByCustomName` in `internal/database/queries/`.
- [x] Run `sqlc generate` (sqlc v1.31.1 per the codegen header).
- [x] Fix the compile-time `ListGroupMembersRow`/`SearchSteamUsersRow`
      assertion drift by keeping both SELECT lists aligned.
- [x] `dbtest`-backed smoke: upsert with `custom_username = true` twice under
      the same lower-cased name fails with a unique violation; a false-flag
      duplicate still succeeds.

**Done when:** `go vet ./...` and the full suite pass; every existing response
byte-for-byte unchanged (the new column is not yet exposed in views).

### Phase 2 — API: ID lookup and ID import

**Scope:** both handlers accept IDs; views expose the flag. The Import page UI
still only sends vanities, so the web app keeps working after this phase.

- [x] Extend `resolveSteamPlayer` usage: add a small `fetchSteamPlayerByID`
      helper (GetPlayerSummaries by canonical ID + empty-list → 404) so the ID
      path reuses the same error vocabulary without touching vanity resolution.
- [x] `GetSteamUserByName`: implement the ID-path order (stored name → stored
      ID → read-only preview); previews of ID input carry the digits name.
- [x] `InsertSteamUser`: optional `vanityUrl`, new `customUsername` field,
      effective-name/flag resolution, uniqueness pre-check → `409`,
      `custom_username` written by the upsert, unique-violation mapped to `409`.
- [x] `steamUserView` + `steamUserSummaryView` expose `customUsername`;
      `GetSteamUserView`, `SearchSteamUsers`, and `GetGroupView` populate it.
- [x] Tests (see Automated, Phase 2).

**Done when:** curl/Go-test evidence of ID import end-to-end (row saved with
the right name + flag, job queued) and ID lookup, with the vanity flows'
existing tests untouched and passing.

### Phase 3 — Import page frontend

**Scope:** the Import page gains ID input, the optional label field, and the
custom-name source row. Lookup states, polling, and buttons stay as built by
the previous plan.

- [x] `normalizeName`: also strip `https://steamcommunity.com/profiles/`
      (new `PROFILE_ID_PREFIX` next to `PROFILE_PREFIX`) plus trailing slash.
- [x] Add `detectSteamID(value)` (regex `^\d{1,19}$` then `Number.isSafeInteger`
      is **not** used — compare as string; the server validates) driving a
      `lookupMode = "id" | "vanity"` decision stored with `currentLookup`.
- [x] Show a `Custom username (optional)` text input (id `custom-username`)
      only while the current input detects as an ID; hide + clear it for
      vanities. Update the hint line to mention IDs and profile links.
- [x] `search()`: unchanged request (the field already carries whatever was
      typed); render `customUsername` in the source row:
      `Stored in the database (custom username)` / otherwise as today.
- [x] `startImport(force)`: ID mode posts
      `{ steamId, customUsername, force }` (no `vanityUrl`); vanity mode keeps
      `{ vanityUrl, steamId, force }`. On `409` from a taken username, show the
      server message and keep the label field focused for a retry.
- [x] Keep `currentLookup` updates and polling behavior identical; the response
      `steamUser` (with its name + flag) replaces the preview as today.

**Done when:** manual pass of flows A–E below at desktop and narrow widths.

### Phase 4 — Surface the flag on Search and View User

**Scope:** users with custom names are recognizable everywhere the summary
view appears.

- [x] Search Users rows and View User summary: a small `badge("Custom name",
    "none")` next to the name (or status) column when `customUsername`.
- [ ] Group member rows get it for free (same view); verify visually.
- [x] No new API work — the field ships in Phase 2.

**Done when:** the manual UI checks pass and the suite is green.

## Edge cases

- **Digits that are actually a vanity** — a real all-digit vanity stored by
  name lookup wins over the ID path in GET (order: name, then ID, then
  Steam), so existing rows never change state because of detection.
- **Leading zeros / non-canonical IDs** — `07656…` parses, and the canonical
  digits (`FormatInt`) are what get stored, matched by ID, and previewed.
- **Overflow input** (`99999999999999999999`) — fails `parseSteamID`, falls to
  the vanity path, and Steam returns no match → normal `404` message. No
  special handling.
- **Custom label colliding with a vanity name** — allowed: the partial unique
  index only covers `custom_username = true` rows, and lookup checks the
  stored name first, so `GetSteamUserByName` may legitimately resolve to
  either row exactly like today's case-twins. Only custom-vs-custom collides.
- **Re-import by ID with a blank label** — keeps the existing name and flag;
  never clobbers a vanity name with digits.
- **Vanity import of a custom-named user** — overwrites the name, clears the
  flag, keeps jobs (keyed by `steam_id`).
- **Race on the same custom label** — pre-check returns `409`; if two requests
  pass the check simultaneously, the partial unique index rejects the second
  and its `23505` error is mapped to the same `409` message.
- **Private profiles by ID** — `GetPlayerSummaries` still returns the profile,
  so preview and import work like the vanity path; wishlist privacy is
  already handled by the importer.
- **`/id/` URL typed with digits after it** (`steamcommunity.com/id/76561…`) —
  normalizes to the digits and is treated as an ID; the profiles URL is the
  canonical form for IDs, but accepting this costs nothing.
- **Lookup busy / stale responses** — token + abort guards already cover the
  new paths; `startImport` sends the ID from `currentLookup`, never from a
  re-read of the input field.

## Testing plan

### Automated — Phase 1

- Migration applies and down-applies cleanly in `dbtest` schemas.
- `UpsertSteamUser` round-trips `custom_username`.
- Partial unique index: duplicate lower-cased custom names rejected; duplicate
  vanity (flag false) names and case-twin mixtures still allowed.

### Automated — Phase 2 (extend `internal/api/handler_test.go`)

- GET with numeric input:
    - stored by-ID row → `source: "database"`, correct state;
    - stored digits-**name** row wins over the ID match (numeric vanity);
    - unknown ID with profile → `source: "steam"` preview, `not_imported`,
      digits name, `customUsername: false`, zero rows/jobs written;
    - unknown ID without profile (empty players) → `404`.
- POST ID path:
    - new ID + label → `202`, row has `name = label`, `custom_username = true`,
      job queued; response carries `customUsername`;
    - new ID, blank label → name is the canonical digits, flag `true`;
    - existing ID, blank label → name/flag preserved;
    - existing vanity row re-imported by ID with label → renamed + flag `true`;
    - duplicate custom username (other steam_id) → `409` + nothing written;
    - the unique-violation mapping is defensive (the pre-check always fires
      first in single-request tests); `isUniqueViolation` detection itself is
      pinned by `TestIsUniqueViolationDetectsConstraintErrors`;
    - unknown ID → `404`, no row/job; both blank → `400`; bad label (empty-ish,
        > 255) → `400`;
    - vanity path regression: existing insert/409/force tests still pass, and a
      vanity import now clears `custom_username` (assert on the stored row).
- Views tests: `customUsername` present in lookup, insert, search, and group
  member payloads.

### Automated — Phases 3–4

- No JS test harness exists in this repo; rely on the manual matrix plus the
  existing Go response tests for the fields the UI reads.

### Manual matrix (test environment, `cmd/test/start.sh`)

1. **ID + label (flow A):** paste a real SteamID64 of a small-library account
   with no vanity (or an existing test account), set a label, Import → preview,
   queued → running → complete, row shows `Stored in the database (custom
username)`, and lookups by label, ID, and profiles URL all resolve.
2. **ID blank (flow B):** import by ID with no label → name is the digits;
   Update user keeps the digits name.
3. **Duplicate label:** import a second ID with a used label → 409 message,
   field stays editable, retry with a new label succeeds.
4. **Bad ID:** `76561197960265729` (no profile) → not-found message; nothing
   written (SQL check).
5. **Vanity wins (flow D):** take a custom-named test user, look them up by a
   real vanity if one exists — otherwise re-import a vanity-known user and
   confirm flag cleared (`SELECT custom_username FROM steam_users …`).
6. **Profile URL paste:** `https://steamcommunity.com/profiles/<id>/` → same
   behavior as the bare ID.
7. **Narrow layout:** the new label field and hints wrap cleanly at ≤ 30rem.
8. **Phase 4 badges:** the `Custom name` badge appears in Search Users and
   View User for the created users, and not for vanity users.

## Acceptance criteria

- A SteamID64 (bare or via a profiles URL) can be looked up and imported
  without any vanity resolution step, end to end, into the same states,
  polling, and job lifecycle as vanity users.
- The optional custom username is stored as the user's name and
  `custom_username = true`; blank labels fall back to the canonical digits and
  never overwrite an existing name.
- Custom usernames are case-insensitively unique among custom-named users,
  enforced by a partial unique index and surfaced as `409` with guidance.
- Vanity imports always clear the flag and win the name.
- No database write happens on any lookup miss.
- `go vet ./...` and the full suite pass after every phase; the manual matrix
  passes at desktop and narrow widths.

## Out of scope

- Renaming a custom username through the UI (re-import with a label is the
  supported path).
- Merging duplicate rows created by importing the same person under a vanity
  and an ID before this feature existed.
- SteamID2/`STEAM_X:Y:Z` and `[U:1:...]` conversions.
- Importing by numeric-only "vanity" auto-detection in the **View User** page
  search (the shared `normalizeName` may be reused there later; no changes
  here).
- Changing the import worker, job schema, or refresh-interval semantics.

## Implementation checklist

### Phase 1 — Schema and data layer

- [x] `00004_steam_user_custom_username.sql` (column + partial unique index).
- [x] `UpsertSteamUser` gains `custom_username`; SELECT lists in
      `SearchSteamUsers` and `ListGroupMembers` gain `su.custom_username`.
- [x] Add `GetSteamUserByCustomName`.
- [x] `sqlc generate`; fix row-shape assertion drift.
- [x] Index behavior tests; suite green.

### Phase 2 — API

- [x] `fetchSteamPlayerByID` helper (read-only summary by ID, 404 vocabulary).
- [x] GET lookup: numeric detection + name → ID → preview order.
- [x] POST: optional `vanityUrl`, `customUsername`, ID path, name/flag
      resolution, uniqueness `409`, upsert flag write.
- [x] `customUsername` in `steamUserView` + `steamUserSummaryView`.
- [x] Phase 2 test list; vanity regressions untouched; suite green.

### Phase 3 — Import page

- [x] `normalizeName` profiles-URL support + `detectSteamID` / `lookupMode`.
- [x] Conditional `Custom username (optional)` input in `#view-import`.
- [x] `startImport` posts the ID-mode body; source row shows the custom-name
      variant; 409-on-label UX keeps the field focused.
- [x] Manual matrix items 1–7.

### Phase 4 — Flag surfacing

- [x] `Custom name` badge in Search Users rows and View User summary.
- [x] Verify group member rows render it via the shared view.
- [ ] Manual matrix item 8; final full-suite run.

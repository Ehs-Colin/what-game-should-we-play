# Test environment

`start.sh` builds the server, recreates an isolated PostgreSQL database, runs
the server against it, and loads representative data for reviewing the
dashboard, search, user, and common games pages. `stop.sh` stops the server.

The development database is never touched: the test database is created next to
it and dropped/recreated on every start.

## Usage

```sh
cmd/test/start.sh
cmd/test/stop.sh
```

In OpenCode, the same commands are available as `/start-test-server` and
`/stop-test-server`.

`start.sh` prints the URL (default <http://localhost:3999>), the database name,
and the log file path. Data lives in `data/testenv/` (gitignored).

The database URL is derived from `GOOSE_DBSTRING` (falling back to
`DATABASE_URL`) by replacing the database name, so the usual `.env` setup works
unchanged.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `TEST_ENV_PORT` | `3999` | HTTP port for the test server. |
| `TEST_ENV_DB_NAME` | `wgswp_test` | Database name to create and drop. |
| `TEST_ENV_DIR` | `data/testenv` | PID, log, and built binary location. |
| `TEST_ENV_BASE_DATABASE_URL` | `GOOSE_DBSTRING`, then `DATABASE_URL` | Admin connection used to create the test database. |
| `TEST_ENV_DATABASE_URL` | derived | Full URL of the test database, for setups where deriving it is not possible. |

## Seeded data

- Dashboard: 9 users, 63 apps, 1 running import job, 1 queued job.
- `SomePlayer`: 34 owned apps (two pages at 25 per page) and 27 wishlist apps.
- Common games: 20 multiplayer apps with release dates. `SomePlayer` +
  `WishlistOnly` match all 20, so the default limit of 10 hides half.
- Common games eligibility: `PlayerTwo` (queued), `casetwin` (failed only), and
  `Alice` (running only) are ineligible. `BobTheBuilder` is eligible through an
  older succeeded import.
- `SomePlayer` + `CaseTwin` + `WishlistOnly` + `PrivateWishlist` produce mixed
  percentages; `Counter-Strike 2` is owned by three and wishlisted by one for a
  combined 4 of 4 (100%).
- `CaseTwin` and `casetwin` differ only by case; View User resolves to
  `CaseTwin`, which has the newer `updated_at`.
- Empty, wishlist-only, and wishlist-private users.
- Failed, succeeded, running, and queued import jobs.
- Removed rows and duplicate aliases to exercise deduplication.
- App details: several apps have a short description, website, price, and
  platform flags for the View User expandable details panel.
- Two apps named `Counter-Strike` for name/app-ID ordering, an app with no
  name, and an app with no header image.

The running and queued jobs are inserted after the server starts so the
background importer does not pick them up. Restarting the server re-enqueues
active jobs and may contact Steam.

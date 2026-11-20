# Steam Decision Engine

> What Game Should We Play?

WIP Custom api program to suggest common owned or wishlisted games between multiple steam users.

## Local development

The server uses PostgreSQL. Create a `.env` file in the project root:

```sh
STEAM_API_KEY=your-steam-api-key

# Used by docker-compose.yml (POSTGRES_DB, POSTGRES_USER and POSTGRES_PORT are optional)
POSTGRES_PASSWORD=change-me

# Required by the server
DATABASE_URL=postgres://wgswp:change-me@localhost:5432/wgswp?sslmode=disable
# Optional, defaults to 10
DATABASE_MAX_OPEN_CONNS=10

# Used only when the database has no accounts yet
DEFAULT_ADMIN_USERNAME=admin
DEFAULT_ADMIN_PASSWORD=change-this-before-first-login
```

Start the database and the server:

```sh
docker compose up -d
go run ./cmd/server
```

Migrations are applied automatically on startup. After editing files in
`internal/database/queries` or `internal/database/migrations`, regenerate the
query code with `sqlc generate`.

## Signing in

The application is protected by a local account. On first startup against an
empty database, the server creates a default account from
`DEFAULT_ADMIN_USERNAME` and `DEFAULT_ADMIN_PASSWORD`. If no accounts exist and
either variable is missing, startup fails.

Sign in with the default account, then create the real account when prompted.
Creating it disables the default account, deletes its sessions, and signs you
in as the real account. After that the environment variables are ignored;
changing them does not reset the account. There is exactly one real account and
no password recovery, so keep the real password safe.

## Tests

Tests that need a database read `TEST_DATABASE_URL` and are skipped when it
isn't set. Each test runs in its own schema, which is dropped afterwards, so
you can point it at the development database:

```sh
TEST_DATABASE_URL=postgres://wgswp:change-me@localhost:5432/wgswp?sslmode=disable go test ./...
```

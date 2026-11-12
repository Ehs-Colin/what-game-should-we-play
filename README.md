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
```

Start the database and the server:

```sh
docker compose up -d
go run ./cmd/server
```

Migrations are applied automatically on startup. After editing files in
`internal/database/queries` or `internal/database/migrations`, regenerate the
query code with `sqlc generate`.

# Game Picker Utility

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

# Optional, defaults to 0. See "Behind a reverse proxy" below.
# TRUSTED_PROXY_HOPS=1

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

## Behind a reverse proxy

The server speaks plain HTTP and is designed to sit behind a TLS-terminating
reverse proxy such as nginx, Caddy, or Traefik. Deploy it at the **root of a
(sub)domain** (for example `https://games.example.com`); mounting it under a
subpath like `/wgswp/` is not supported.

Two environment variables matter:

```sh
# The public origin exactly as the browser sees it. This drives the CSRF
# origin check and marks the session cookie Secure, so it must be the HTTPS
# origin in production.
APP_ORIGIN=https://games.example.com

# Number of reverse proxies between the client and this server (1 for a
# single nginx/Caddy/Traefik hop). Rate limiting and login throttling then
# use the real client address from X-Forwarded-For. Defaults to 0.
TRUSTED_PROXY_HOPS=1
```

> **Security:** `TRUSTED_PROXY_HOPS` trusts the right-most entries of
> `X-Forwarded-For`. Only enable it when clients cannot reach the server
> directly: bind it to loopback or an internal Docker network and do not
> publish its port. Otherwise anyone can spoof their IP and bypass the
> per-IP rate limits.

Other notes:

- Steam-backed requests can be slow; the server allows up to 300 seconds per
  response. Raise the proxy's read/send timeouts to match (nginx and Traefik
  default to 60 seconds).
- `GET /health` is public and returns `200 {"ok":true,...}`. Use it for
  proxy and container health checks.
- The frontend uses relative URLs and polling (no WebSockets), so no rewrite
  or connection-upgrade rules are needed.
- Enable compression at the proxy if desired; do not cache HTML or `/api`
  responses (they are personalized and marked `no-store`).

### nginx

```nginx
server {
    listen 443 ssl;
    server_name games.example.com;
    # ssl_certificate / ssl_certificate_key ...

    location / {
        proxy_pass http://127.0.0.1:3030;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
```

### Caddy

Caddy provisions TLS and appends `X-Forwarded-For` automatically:

```caddyfile
games.example.com {
    encode gzip
    reverse_proxy 127.0.0.1:3030
}
```

### Traefik (Docker labels)

Run the app on an internal network with no published ports:

```yaml
labels:
    - "traefik.enable=true"
    - "traefik.http.routers.wgswp.rule=Host(`games.example.com`)"
    - "traefik.http.routers.wgswp.entrypoints=websecure"
    - "traefik.http.routers.wgswp.tls.certresolver=letsencrypt"
    - "traefik.http.services.wgswp.loadbalancer.server.port=3030"
    - "traefik.http.services.wgswp.loadbalancer.healthcheck.path=/health"
```

Traefik appends `X-Forwarded-For` by default. If you also use Traefik's
`rateLimit` middleware, configure its `ipStrategy` to read the forwarded
client IP, or leave rate limiting to the app.

## Tests

Tests that need a database read `TEST_DATABASE_URL` and are skipped when it
isn't set. Each test runs in its own schema, which is dropped afterwards, so
you can point it at the development database:

```sh
TEST_DATABASE_URL=postgres://wgswp:change-me@localhost:5432/wgswp?sslmode=disable go test ./...
```

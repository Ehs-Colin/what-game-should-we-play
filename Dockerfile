# syntax=docker/dockerfile:1

# --- Build stage -----------------------------------------------------------
# Compiles the server. Only Go module metadata and source are used here; the
# web assets and everything else are excluded by .dockerignore.
FROM golang:1.27-alpine AS build

WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

# Source needed to compile ./cmd/server. Migrations are embedded via
# //go:embed migrations/*.sql in internal/database, so they must be present at
# build time (they are, as part of internal/) and are NOT needed at runtime.
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/wgswp ./cmd/server

# Pre-create the log directory owned by the distroless nonroot UID (65532) so a
# mounted named volume inherits writable ownership for data/wgswp.log.
RUN mkdir -p /out/data && chown -R 65532:65532 /out/data

# --- Runtime stage ---------------------------------------------------------
# Minimal, non-root Debian base that ships CA certificates (needed for HTTPS to
# the Steam Web API). The binary is CGO-free, so a static runtime is fine.
FROM gcr.io/distroless/base:nonroot AS final

WORKDIR /app

# The working directory is the base for the paths the server reads:
#   web/public      -> static files served via os.OpenRoot (required at startup)
#   data/wgswp.log  -> created at startup by logging.Open (needs a writable dir)
COPY --from=build /out/wgswp ./wgswp
COPY --from=build /out/data ./data
COPY web/public ./web/public

EXPOSE 3030

ENTRYPOINT ["/app/wgswp"]

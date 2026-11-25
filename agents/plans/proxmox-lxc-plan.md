# Proxmox LXC deployment plan

Status: proposed

## Goal

Add a Proxmox VE LXC installer and updater for Game Picker Utility that follows
the structure and behaviour of the community-scripts `ct/*.sh` and
`install/*-install.sh` scripts:

- create an unprivileged Debian 13 container,
- install PostgreSQL locally,
- install the application from a versioned GitHub release,
- start it under systemd with a generated environment file,
- and support in-place updates from inside the container with `/usr/bin/update`.

The existing `Dockerfile` and `docker-compose.yml` remain the local/Docker
deployment path. The Proxmox scripts are a native, no-Docker alternative.

## Decisions

| Topic              | Decision                                                                                                                            |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------- |
| Install source     | Prebuilt release binary from GitHub Releases. No Go toolchain in the container.                                                     |
| Script location    | `deploy/proxmox/ct/wgswp.sh` and `deploy/proxmox/install/wgswp-install.sh`.                                                         |
| Script audience    | Self-hosted in this repository. Keep the code close enough to community-scripts that it can be submitted upstream later.            |
| Release pipeline   | Tag-driven GitHub Actions workflow publishing `linux/amd64` and `linux/arm64` tarballs.                                             |
| Database           | PostgreSQL 18 inside the container, installed with `setup_postgresql` from PGDG.                                                    |
| First-run secrets  | Prompt for the Steam API key and admin username; generate the bootstrap admin password and save it to `/root/wgswp.creds`.          |
| Network            | Plain HTTP on `:3030`, `APP_ORIGIN=http://<container-ip>:3030`, `TRUSTED_PROXY_HOPS=0`. TLS is the operator's responsibility.       |
| Updates            | Back up the database, update OS/PostgreSQL packages, deploy the new app release, health-check it, and roll back the app on failure. |
| Architectures      | amd64 and arm64.                                                                                                                    |
| Container defaults | Debian 13, unprivileged, 1 vCPU, 1024 MiB RAM, 8 GiB disk.                                                                          |
| App slug           | `WGSWP`. The framework derives `NSAPP=wgswp`, so the files are `ct/wgswp.sh` and `install/wgswp-install.sh`.                        |

## Repository layout

```text
.github/workflows/release.yml
deploy/proxmox/
  README.md
  ct/wgswp.sh
  install/wgswp-install.sh
```

The `ct/` and `install/` directories must remain siblings. The community-scripts
engine resolves `install/<NSAPP>-install.sh` relative to `COMMUNITY_SCRIPTS_URL`,
which points at the `deploy/proxmox` directory in this repository.

The ct script must set the scripts base before sourcing `build.func`, otherwise a
curl'd fork falls back to the official community-scripts repository:

```bash
# deploy/proxmox/ct/wgswp.sh
export COMMUNITY_SCRIPTS_URL="${COMMUNITY_SCRIPTS_URL:-https://raw.githubusercontent.com/Ehs-Colin/what-game-should-we-play/main/deploy/proxmox}"

_cs_boot="${COMMUNITY_SCRIPTS_CORE_DIR:-$(dirname "${BASH_SOURCE[0]}")/../../core}/core/build.func"
source "$_cs_boot" 2>/dev/null || source <(curl -fsSL "${COMMUNITY_SCRIPTS_CORE_URL:-https://raw.githubusercontent.com/community-scripts/core/main}/core/build.func")
```

The engine itself continues to come from `community-scripts/core`. Only the
application scripts come from this repository.

## Container defaults

Set these at the top of `ct/wgswp.sh`:

| Variable           | Value          | Reason                                                            |
| ------------------ | -------------- | ----------------------------------------------------------------- |
| `APP`              | `WGSWP`        | Yields the `wgswp` slug and update entrypoint name.               |
| `var_tags`         | `gaming;steam` | Matches the application domain.                                   |
| `var_cpu`          | `1`            | Plenty for one Go process plus PostgreSQL.                        |
| `var_ram`          | `1024`         | PostgreSQL and the app fit comfortably; allows for Steam imports. |
| `var_disk`         | `8`            | OS, PostgreSQL, binary, web assets, and database backups.         |
| `var_os`           | `debian`       | PostgreSQL 18 and systemd are well supported.                     |
| `var_version`      | `13`           | Current stable Debian template.                                   |
| `var_arm64`        | `yes`          | The server is pure Go and PostgreSQL supports arm64.              |
| `var_unprivileged` | `1`            | Default secure container type.                                    |

Add optional non-interactive overrides so the installer can run unattended:

```bash
var_steam_api_key="${var_steam_api_key:-}"
var_admin_username="${var_admin_username:-}"
var_admin_password="${var_admin_password:-}"
export var_steam_api_key var_admin_username var_admin_password
```

The `export` is important: build variables reach the install script through
`lxc-attach`'s environment.

## Release artifact contract

The installer and updater depend on this exact contract.

### Trigger

A pushed tag matching `v*.*.*`, for example `v1.0.0`.

### Assets

| Asset                             | Contents                                                                                |
| --------------------------------- | --------------------------------------------------------------------------------------- |
| `wgswp_v1.0.0_linux_amd64.tar.gz` | Top-level directory `wgswp_v1.0.0_linux_amd64/` containing `wgswp` and `web/public/**`. |
| `wgswp_v1.0.0_linux_arm64.tar.gz` | Same layout, arm64 binary.                                                              |
| `sha256sums.txt`                  | Checksums for both archives.                                                            |

Do not include `data/`, `.env`, credentials, or the source tree in the archive.
The release archive is the runtime payload only.

### Build

Reuse the Dockerfile's build settings so native and container builds behave the
same:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=<amd64|arm64> \
  go build -trimpath -ldflags "-s -w" -o dist/wgswp ./cmd/server
```

### Workflow outline

`.github/workflows/release.yml`:

1. Trigger on `push.tags: ['v*.*.*']`; `permissions: contents: write`.
2. A matrix job builds `amd64` and `arm64` archives with `actions/setup-go`
   reading `go-version-file: go.mod` (or `go-version: '1.27.x'` if the exact
   patch is not yet available), then uploads them as workflow artifacts.
3. A publish job downloads both artifacts, generates `sha256sums.txt`, and
   creates the GitHub release with `gh release create` or
   `softprops/action-gh-release`.
4. Generate release notes from commits.

Optionally add a `verify` job that runs `go vet ./...` and `go test ./...`.
Database tests skip themselves when `TEST_DATABASE_URL` is unset, so the job
works without a PostgreSQL service.

### How the scripts consume it

```bash
export CLEAN_INSTALL=1
export CLEAN_INSTALL_KEEP="data"
fetch_and_deploy_gh_release \
  "wgswp" \
  "Ehs-Colin/what-game-should-we-play" \
  "prebuild" \
  "latest" \
  "/opt/wgswp" \
  "wgswp_*_linux_$(arch_resolve amd64 arm64).tar.gz"
```

`prebuild` extracts the archive, strips the single top-level directory, copies
the payload into `/opt/wgswp`, and records the version in `~/.wgswp` for the
next `check_for_gh_release` call. `CLEAN_INSTALL=1` removes stale files;
`CLEAN_INSTALL_KEEP="data"` preserves the `data` symlink described below.

## Runtime layout

| Path                                | Purpose                                                                                            |
| ----------------------------------- | -------------------------------------------------------------------------------------------------- |
| `/opt/wgswp/wgswp`                  | Application binary.                                                                                |
| `/opt/wgswp/web/public/`            | Static frontend required at startup.                                                               |
| `/opt/wgswp/data`                   | Symlink to `/var/lib/wgswp`. The server writes `data/wgswp.log` relative to its working directory. |
| `/var/lib/wgswp/wgswp.log`          | JSON application log.                                                                              |
| `/etc/wgswp/wgswp.env`              | Environment file, `0600 root:root`.                                                                |
| `/var/backups/wgswp/`               | Database dumps and rollback copies, `0700 root:root`.                                              |
| `/root/wgswp.creds`                 | Generated credentials summary, `0600 root:root`.                                                   |
| `/etc/systemd/system/wgswp.service` | systemd unit.                                                                                      |
| `/etc/logrotate.d/wgswp`            | Log rotation for the JSON log.                                                                     |

Do **not** write an `.env` file into `/opt/wgswp`. `config.Load` reads an
optional `.env` from the working directory before applying the process
environment; keeping configuration only in the systemd environment file avoids
two competing sources.

Create the service account and writable data directory before downloading the
release:

```bash
adduser --system --group --home /var/lib/wgswp --shell /usr/sbin/nologin wgswp
mkdir -p /opt/wgswp /var/lib/wgswp
chown wgswp:wgswp /var/lib/wgswp
ln -sfn /var/lib/wgswp /opt/wgswp/data
chown -h wgswp:wgswp /opt/wgswp/data
```

The release helper runs as root, so re-apply ownership after every deploy:

```bash
chown -R wgswp:wgswp /opt/wgswp
chown -h wgswp:wgswp /opt/wgswp/data
```

## Install flow

### 1. Framework setup

`install/wgswp-install.sh` starts like the community-scripts examples:

```bash
source /dev/stdin <<<"$FUNCTIONS_FILE_PATH"
color
verb_ip6
catch_errors
setting_up_container
network_check
update_os
```

Abort if `/opt/wgswp/wgswp` already exists, and point at the update path instead
of overwriting an existing installation.

### 2. Dependencies

Install only what the app and helper scripts need:

```bash
$STD apt install -y ca-certificates curl jq tar
```

PostgreSQL packages are installed by `setup_postgresql` in the next step.

### 3. PostgreSQL 18

```bash
PG_VERSION="18" setup_postgresql
PG_DB_NAME="wgswp" PG_DB_USER="wgswp" setup_postgresql_db
```

`setup_postgresql` adds the PGDG repository, installs PostgreSQL 18, and starts
the service. `setup_postgresql_db` creates the database and role, generates a
password when `PG_DB_PASS` is empty, grants ownership, and exports
`PG_DB_PASS` for the environment file.

Pin PostgreSQL to 18 in the installer and updater. A future major upgrade to 19
is a separate migration and must not be triggered implicitly by an app update.

### 4. Create the service account and directories

Create the `wgswp` system user, `/var/lib/wgswp`, and the `/opt/wgswp/data`
symlink as described under "Runtime layout" before downloading the release.

### 5. Download the application

Use `fetch_and_deploy_gh_release` as shown above. After it returns:

- verify `/opt/wgswp/wgswp` is executable,
- verify `/opt/wgswp/web/public/index.html` exists,
- fail with a clear message when no release asset matches. A repository with no
  tags has no installable release; the first `v*.*.*` tag must exist before the
  installer can succeed.

### 6. Collect first-run settings

The installer prompts for values unless they were supplied through `var_*`:

| Value          | Behaviour                                                                                                                                 |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Steam API key  | Required. Hidden input with `read -r -s`. Warn, but do not abort, when the value is not 32 hex characters.                                |
| Admin username | Default `admin`. Validate against the app's policy: 3–64 characters, starts alphanumeric, then `A-Z a-z 0-9 . _ -`. Re-prompt on failure. |
| Admin password | Generated with `random_password 24` unless `var_admin_password` is set. Enforce 12–128 characters for supplied values.                    |

Display the generated admin password once at the end of installation and write
all credentials to `/root/wgswp.creds` with mode `0600`.

The generated password only bootstraps the default account. On first browser
login the user is forced to create the real account; after that the default
account is disabled and the `DEFAULT_ADMIN_*` variables are ignored by the app.

### 7. Environment file

Write `/etc/wgswp/wgswp.env` with mode `0600` and owner `root:root`:

```bash
PORT=3030
APP_ORIGIN="http://<LOCAL_IP>:3030"
TRUSTED_PROXY_HOPS=0

DATABASE_URL="postgres://wgswp:<PG_DB_PASS>@127.0.0.1:5432/wgswp?sslmode=disable"
DATABASE_MAX_OPEN_CONNS=10

STEAM_API_KEY="<steam-api-key>"
DEFAULT_ADMIN_USERNAME="<admin-username>"
DEFAULT_ADMIN_PASSWORD="<generated-or-supplied-password>"
```

Notes:

- Quote every value. systemd's `EnvironmentFile` parser does not run a shell,
  but quotes protect `#`, spaces, and URL punctuation. Reject or escape values
  containing `"`, `\`, newlines, or leading/trailing whitespace. Generated
  passwords are alphanumeric, so the default path is safe.
- `LOCAL_IP` comes from the framework. Fall back to
  `hostname -I | awk '{print $1}'` and abort if it is still empty.
- `random_password` produces alphanumeric output, so the password needs no URL
  escaping. If a supplied password contains reserved characters, URL-encode it
  before writing `DATABASE_URL`.
- The app requires `APP_ORIGIN` to be the exact browser origin. A DHCP address
  change breaks CSRF checks and the login cookie. Document a DHCP reservation
  or static address as a prerequisite.
- Leave `TRUSTED_PROXY_HOPS=0`. Only set it to `1` after the app is placed
  behind exactly one reverse proxy and direct access to port 3030 is blocked.
- `STEAM_API_TIMEOUT` and `STEAM_REFRESH_INTERVAL` keep their application
  defaults unless the operator adds them later.

### 8. systemd unit

```ini
[Unit]
Description=Game Picker Utility (WGSWP)
Wants=network-online.target
After=network-online.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=wgswp
Group=wgswp
WorkingDirectory=/opt/wgswp
EnvironmentFile=/etc/wgswp/wgswp.env
ExecStart=/opt/wgswp/wgswp
Restart=on-failure
RestartSec=5
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/wgswp

[Install]
WantedBy=multi-user.target
```

`ProtectSystem=strict` makes `/opt` read-only while `ReadWritePaths` keeps the
log directory writable. The binary and web assets are read-only at runtime.

Enable and start:

```bash
systemctl daemon-reload
systemctl enable -q --now wgswp
```

Wait up to 60 seconds for `GET /health` on `127.0.0.1:3030`:

```bash
healthy=0
for _ in $(seq 1 30); do
  if curl -fsS --max-time 5 http://127.0.0.1:3030/health >/dev/null; then
    healthy=1
    break
  fi
  sleep 2
done
[[ "$healthy" -eq 1 ]] || {
  systemctl status wgswp --no-pager || true
  journalctl -u wgswp -n 50 --no-pager || true
  msg_error "WGSWP did not become healthy"
  exit 1
}
```

### 9. Log rotation

The app writes JSON to `/var/lib/wgswp/wgswp.log` and does not reopen the file
on rotation. Use `copytruncate`:

```text
/var/lib/wgswp/wgswp.log {
    weekly
    rotate 4
    compress
    missingok
    notifempty
    copytruncate
    su wgswp wgswp
}
```

### 10. Finish

```bash
motd_ssh
customize
cleanup_lxc
```

Print the access URL, the credentials path, and a reminder that the first login
must create the real account.

## Update flow

`ct/wgswp.sh` defines `update_script` and `update_deb_based`. The framework's
`run_os_update` dispatches to `update_deb_based` on Debian.

```bash
function update_script() {
  header_info
  check_container_storage
  check_container_resources
  if [[ ! -f /opt/wgswp/wgswp ]]; then
    msg_error "No ${APP} Installation Found!"
    exit
  fi
  run_os_update
  exit
}
```

`update_deb_based` performs these steps:

1. **Update Debian packages.**

    ```bash
    msg_info "Updating Debian Packages"
    $STD apt-get update
    $STD apt-get -y upgrade
    msg_ok "Updated Debian Packages"
    ```

    Keep PostgreSQL at 18 and refresh its packages explicitly:

    ```bash
    PG_VERSION="18" setup_postgresql
    ```

    A PostgreSQL package upgrade can restart PostgreSQL. If the app is still
    running and no app update follows, restart `wgswp` after this step so its
    connection pool is rebuilt, then verify `/health`.

2. **Check for an application release.**

    ```bash
    if check_for_gh_release "wgswp" "Ehs-Colin/what-game-should-we-play"; then
      ...
    fi
    ```

    If no newer release exists, finish with "Updated successfully" after the OS
    package step. Do not back up the database when there is no app update.

3. **Back up the database before stopping the app.**

    ```bash
    install -d -m 0700 /var/backups/wgswp
    PG_BIN="/usr/lib/postgresql/$(ls /usr/lib/postgresql | sort -V | tail -1)/bin"
    dump="/var/backups/wgswp/wgswp-$(date +%Y%m%d-%H%M%S).dump"
    runuser -u postgres -- "$PG_BIN/pg_dump" \
      --format=custom --no-owner wgswp >"$dump" || {
        rm -f "$dump"
        msg_error "Database backup failed; aborting update"
        return 1
      }
    chmod 600 "$dump"
    ```

    Resolve the installed PostgreSQL version rather than relying on
    `/usr/bin/pg_dump`; `setup_postgresql` writes its PATH to `/etc/environment`,
    which a non-login update shell may not inherit. The intended version is 18,
    but the same command keeps working if PGDG falls back to a distro package.
    The redirection is intentional: `pg_dump` runs as `postgres`, but the backup
    directory is `0700 root:root`, so `postgres` writes to stdout and root owns
    the resulting file. Keep the newest three dumps:

    ```bash
    ls -1t /var/backups/wgswp/wgswp-*.dump | tail -n +4 | xargs -r rm -f --
    ```

4. **Save a rollback copy outside the deployment directory.**

    ```bash
    rm -rf /var/backups/wgswp/rollback
    mkdir -p /var/backups/wgswp/rollback
    cp -a /opt/wgswp/wgswp /var/backups/wgswp/rollback/wgswp
    cp -a /opt/wgswp/web /var/backups/wgswp/rollback/web
    PREVIOUS_VERSION="$(cat ~/.wgswp 2>/dev/null || true)"
    ```

5. **Stop, deploy, start.**

    ```bash
    systemctl stop wgswp
    CLEAN_INSTALL=1 CLEAN_INSTALL_KEEP="data" \
      fetch_and_deploy_gh_release "wgswp" \
        "Ehs-Colin/what-game-should-we-play" "prebuild" "latest" \
        "/opt/wgswp" "wgswp_*_linux_$(arch_resolve amd64 arm64).tar.gz"
    chown -R wgswp:wgswp /opt/wgswp
    chown -h wgswp:wgswp /opt/wgswp/data
    systemctl start wgswp
    ```

    Migrations are embedded in the binary and run automatically on startup, so
    the new schema is applied before the service reports healthy.

6. **Health-check and roll back on failure.**

    ```bash
    healthy=0
    for _ in $(seq 1 30); do
      if curl -fsS --max-time 5 http://127.0.0.1:3030/health >/dev/null; then
        healthy=1
        break
      fi
      sleep 2
    done
    ```

    On failure (`healthy=0`), or if `fetch_and_deploy_gh_release` itself fails:

    - stop the service,
    - restore `wgswp` and `web` from `/var/backups/wgswp/rollback`,
    - restore `~/.wgswp` to `PREVIOUS_VERSION` so the next update retries; if
      there was no previous version stamp, remove the file instead,
    - start the service and report the failure with the database dump path.

    Migrations are forward-only. If the failed release already changed the
    schema in an incompatible way, automatic rollback is not sufficient; the
    operator must restore the dump with `pg_restore` and start the previous
    binary. The update output must say this explicitly rather than pretending the
    rollback is complete.

7. **Verify data and service persistence.**

    Confirm `/health` returns 200, `systemctl is-active wgswp` is `active`, and
    `/var/lib/wgswp/wgswp.log` still receives records. The `data` symlink and
    `/etc/wgswp/wgswp.env` must survive `CLEAN_INSTALL`.

## Security and operational notes

- The container is unprivileged. PostgreSQL listens on localhost only; port 5432
  is not exposed.
- `/etc/wgswp/wgswp.env` contains the Steam key and database password. It is
  readable only by root. It is not included in release archives or logs.
- `/root/wgswp.creds` contains the generated bootstrap password. The operator
  can delete it after creating the real account.
- After the real account exists, `DEFAULT_ADMIN_USERNAME` and
  `DEFAULT_ADMIN_PASSWORD` are ignored. They can be removed from the environment
  file during a later maintenance window.
- The app speaks plain HTTP. Do not expose port 3030 directly to the internet
  without a TLS-terminating reverse proxy.
- If a reverse proxy is added, set `APP_ORIGIN` to the exact HTTPS origin and
  `TRUSTED_PROXY_HOPS=1`, and block direct access to 3030. Mounting the app
  under a subpath is not supported.
- `GET /health` is public and is the health check for systemd verification,
  proxies, and monitoring.
- Database backups do not include the application log or the release payload.
  They contain users, sessions, the Steam cache, libraries, and wishlists.
- The GitHub release check is unauthenticated. It is well within GitHub's
  anonymous API rate limit for one container checking on demand.

## Backup and restore

Automatic dumps are written to `/var/backups/wgswp/wgswp-YYYYmmdd-HHMMSS.dump`.
To restore one manually:

```bash
systemctl stop wgswp
PG_BIN="/usr/lib/postgresql/$(ls /usr/lib/postgresql | sort -V | tail -1)/bin"
runuser -u postgres -- "$PG_BIN/dropdb" --if-exists wgswp
runuser -u postgres -- "$PG_BIN/createdb" --owner=wgswp wgswp
runuser -u postgres -- "$PG_BIN/pg_restore" \
  --no-owner --dbname=wgswp /var/backups/wgswp/<dump>
systemctl start wgswp
```

Restore the matching previous binary from `/var/backups/wgswp/rollback` first
when the dump predates a schema migration. The application applies embedded
migrations on startup, so a dump from an older release can be brought forward,
but a dump cannot be downgraded.

## Testing plan

### Static checks

- `shellcheck deploy/proxmox/ct/wgswp.sh deploy/proxmox/install/wgswp-install.sh`
- `bash -n` both scripts.
- Verify the archive pattern matches the actual release asset names.

### Fresh install

- Proxmox VE 8.x and 9.x, Debian 13, unprivileged, amd64.
- Repeat on arm64 if hardware is available; otherwise record it as untested.
- Confirm PostgreSQL 18 is installed and the `wgswp` database exists.
- Confirm the service starts and `/health` returns 200.
- Confirm the first browser login works with the generated admin password and
  that creating the real account disables the default account.
- Confirm a Steam import and a common-games request succeed.
- Confirm `/var/lib/wgswp/wgswp.log` is written and logrotate parses the config.
- Reboot the container and confirm PostgreSQL and WGSWP start automatically.

### Update

- Publish `v1.0.0`, install it, then publish `v1.0.1` and run `/usr/bin/update`.
- Confirm a timestamped `pg_dump` exists before the service stops.
- Confirm the new binary and web assets are deployed and the old release is
  available under `/var/backups/wgswp/rollback`.
- Confirm migrations run and existing users/data survive.
- Confirm only three dumps are retained.
- Run `/usr/bin/update` again with no new release; confirm the app is untouched
  and the command succeeds after OS package updates.
- Simulate a bad release that fails health checks; confirm the previous binary
  is restored, the service restarts, and the next update is attempted again.
- Confirm `/etc/wgswp/wgswp.env` and `/var/lib/wgswp` survive
  `CLEAN_INSTALL=1`.

### Failure modes

- No GitHub release exists: installer exits with a clear "no release asset"
  message.
- Steam API key missing: installer re-prompts or exits non-zero.
- PostgreSQL fails to start: installer exits with the PostgreSQL status.
- Invalid `APP_ORIGIN`: service fails at startup and the health check reports it.
- Disk full during deploy: helper reports the disk error and does not start a
  half-deployed release.

## Implementation checklist

1. Add `.github/workflows/release.yml` with amd64/arm64 build, checksums, and
   release publication.
2. Push a `v0.1.0` tag and verify both assets and `sha256sums.txt`.
3. Add `deploy/proxmox/ct/wgswp.sh` with container defaults, the
   `COMMUNITY_SCRIPTS_URL` override, and `update_script`/`update_deb_based`.
4. Add `deploy/proxmox/install/wgswp-install.sh` with the install flow above.
5. Add `deploy/proxmox/README.md` with the one-liner, prerequisites, first-login
   steps, update instructions, backup/restore notes, and reverse-proxy note.
6. Run shellcheck, a fresh install, an update, and a rollback test.
7. Document any deviations from community-scripts conventions before considering
   an upstream submission.

## Acceptance criteria

- A new Proxmox user can run one command against the raw `ct/wgswp.sh` URL and
  get a working container with the app reachable at `http://<ip>:3030`.
- The same container can be updated from inside with `/usr/bin/update`, without
  losing users, sessions, libraries, wishlists, or the Steam cache.
- The database is backed up before every application update and the newest
  three backups are retained.
- A failed release does not leave the container permanently broken; the
  previous binary is restored and the failure is actionable.
- No secrets are stored in `/opt/wgswp`, release archives, or logs.
- The app survives a container reboot with PostgreSQL and WGSWP enabled.

## Open questions

None. The decisions above were confirmed:

- prebuilt release binary,
- scripts under `deploy/proxmox/`,
- tag-driven release workflow,
- local PostgreSQL 18,
- prompted secrets with a generated admin password,
- plain HTTP on `:3030`,
- database backup plus OS updates on update,
- amd64 and arm64.

One naming detail is deliberately fixed by this plan: `APP="WGSWP"` keeps the
container/update slug short and matches the codebase's existing `wgswp` binary,
log file, and database names. If the Proxmox UI must display "Steam Decision
Engine" instead, the application name and all derived file names must be
changed together; they cannot differ.

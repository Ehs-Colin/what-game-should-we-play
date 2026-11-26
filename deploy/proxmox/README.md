# Game Picker Utility on Proxmox VE

This directory contains a native Proxmox VE deployment for Game Picker Utility
(WGSWP). It creates an unprivileged Debian 13 LXC container, installs
PostgreSQL 18 locally, deploys a prebuilt release under systemd, and supports
in-place updates with `/usr/bin/update`.

The existing `Dockerfile` and `docker-compose.yml` remain the Docker/local
deployment path. These scripts are the no-Docker alternative.

## Prerequisites

- Proxmox VE 8.x or 9.x, with a root shell on the host.
- A Debian 13 LXC template that the host can download or already has cached.
- A published `v*.*.*` GitHub release containing the `amd64` and `arm64`
  archives. The installer fetches the latest release.
- A Steam Web API key from <https://steamcommunity.com/dev/apikey>.
- A **static IP address or DHCP reservation** for the container. `APP_ORIGIN`
  is written at install time; if the container's address changes later, CSRF
  checks and the login cookie will fail until `APP_ORIGIN` is updated.
- Roughly 1 vCPU, 1 GiB RAM, and 8 GiB disk. These are the container defaults.

The installer does not expose PostgreSQL outside the container. The app listens
on plain HTTP port `3030`.

## Install

Run this on the Proxmox host:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Ehs-Colin/what-game-should-we-play/main/deploy/proxmox/ct/wgswp.sh)"
```

The helper asks for the Steam API key and the bootstrap admin username
(default `admin`), creates the container, and prints the access URL when it is
healthy. If no admin password is supplied, a 24-character password is generated,
shown once at the end, and stored in `/root/wgswp.creds`.

### Container defaults

| Setting           | Default        |
| ----------------- | -------------- |
| OS                | Debian 13      |
| CPU               | 1 vCPU         |
| RAM               | 1024 MiB       |
| Disk              | 8 GiB          |
| Unprivileged      | yes            |
| Architecture      | amd64 or arm64 |

Override them with environment variables before `bash`, for example:

```bash
var_cpu=2 var_ram=2048 var_disk=16 \
  bash -c "$(curl -fsSL https://raw.githubusercontent.com/Ehs-Colin/what-game-should-we-play/main/deploy/proxmox/ct/wgswp.sh)"
```

### Unattended install

The first-run values can be supplied without prompts:

```bash
var_steam_api_key="your-32-character-key" \
var_admin_username="admin" \
var_admin_password="a-strong-password" \
  bash -c "$(curl -fsSL https://raw.githubusercontent.com/Ehs-Colin/what-game-should-we-play/main/deploy/proxmox/ct/wgswp.sh)"
```

- Omit `var_admin_password` to generate a 24-character password.
- A supplied admin password must be between 12 and 128 characters.
- The admin username must be 3-64 characters, start with a letter or number,
  and contain only letters, numbers, `.`, `_`, or `-`.

## First login

Open:

```text
http://<container-ip>:3030
```

Find the container ID with `pct list`. The container writes credentials to
`/root/wgswp.creds` with mode `0600`. It contains the PostgreSQL credentials,
the bootstrap admin username and password, and the Steam API key. From the
Proxmox host:

```bash
pct exec <ctid> -- cat /root/wgswp.creds
```

Sign in with the bootstrap account and create the real account when prompted.
Creating the real account disables the bootstrap account and deletes its
sessions. After that, `DEFAULT_ADMIN_USERNAME` and `DEFAULT_ADMIN_PASSWORD` in
`/etc/wgswp/wgswp.env` are ignored. There is exactly one real account and no
password recovery, so store the real password safely.

Once the real account exists, you can remove `/root/wgswp.creds` after copying
the database password and Steam key somewhere safe.

Useful checks:

```bash
pct exec <ctid> -- systemctl status wgswp --no-pager
pct exec <ctid> -- curl -fsS http://127.0.0.1:3030/health
pct exec <ctid> -- journalctl -u wgswp -n 100 --no-pager
```

The application log is `/var/lib/wgswp/wgswp.log`. Log rotation is configured
in `/etc/logrotate.d/wgswp` with `copytruncate`, weekly rotation, four retained
files, and compression.

## Updating

From inside the container:

```bash
pct enter <ctid>
/usr/bin/update
```

Or without an interactive shell:

```bash
pct exec <ctid> -- /usr/bin/update
```

The update path:

1. Updates Debian packages and refreshes PostgreSQL 18 packages. The version
   is pinned so a future PostgreSQL 19 is not installed implicitly.
2. Checks GitHub for a newer application release.
3. If a release exists, dumps the database **before** stopping the service.
   A failed dump aborts the update while the old release is still running.
   Otherwise it saves the current binary and web assets under
   `/var/backups/wgswp/rollback`, stops the service, deploys the release,
   restores ownership, starts the service, and waits for `/health`.
4. If deployment or the health check fails, restores the previous binary and
   web assets, restarts the service, and leaves the database dump in place.
5. If no newer release exists, only the OS/PostgreSQL packages are updated and
   a running application is restarted so its connection pool is rebuilt.

Only the three newest database dumps are retained. A `CLEAN_INSTALL` deployment
preserves `/opt/wgswp/data` and `/etc/wgswp/wgswp.env`.

Migrations are embedded in the binary and run automatically on startup.
Migrations are **forward-only**: if a failed release already changed the schema
in an incompatible way, restoring the previous binary is not enough. Restore
the matching database dump as described below.

## Backup and restore

Database dumps are created before every application update:

```text
/var/backups/wgswp/wgswp-YYYYmmdd-HHMMSS.dump
```

The directory is `0700 root:root` and dumps use PostgreSQL's custom format. They
contain users, sessions, the Steam cache, libraries, and wishlists; they do not
contain the application log or the deployed release. Automatic dumps are not a
scheduled backup: copy them off the container if you need disaster recovery.

After an update, the previous release is kept at:

```text
/var/backups/wgswp/rollback/wgswp
/var/backups/wgswp/rollback/web
```

The rollback copy contains only the binary and web assets, not the database or
the environment file.

The environment file, which contains the database password and Steam API key,
is `/etc/wgswp/wgswp.env` (`0600 root:root`). Back it up separately if you need
full disaster recovery.

### Restore a dump

Choose the dump that matches the binary you intend to run. If the dump predates
a schema migration, restore the matching previous binary from the rollback
directory first.

```bash
pct exec <ctid> -- systemctl stop wgswp
pct exec <ctid> -- bash -c '
  set -euo pipefail
  PG_BIN="/usr/lib/postgresql/$(ls /usr/lib/postgresql | sort -V | tail -1)/bin"
  runuser -u postgres -- "$PG_BIN/dropdb" --if-exists wgswp
  runuser -u postgres -- "$PG_BIN/createdb" --owner=wgswp wgswp
  runuser -u postgres -- "$PG_BIN/pg_restore" \
    --no-owner --dbname=wgswp /var/backups/wgswp/<dump>
'
pct exec <ctid> -- systemctl start wgswp
```

A dump from an older release can be brought forward by the new binary's
migrations on startup. A dump from a newer release cannot be downgraded.

## Reverse proxy and TLS

The app speaks plain HTTP on `:3030` and does not terminate TLS itself. Do not
expose port `3030` directly to the internet without a TLS-terminating reverse
proxy.

If you place it behind exactly one reverse proxy:

1. Set `APP_ORIGIN` in `/etc/wgswp/wgswp.env` to the exact public HTTPS origin,
   for example `APP_ORIGIN="https://games.example.com"`.
2. Set `TRUSTED_PROXY_HOPS=1`.
3. Block direct access to port `3030` from clients.
4. Restart the service:

   ```bash
   pct exec <ctid> -- systemctl restart wgswp
   ```

Leave `TRUSTED_PROXY_HOPS=0` when the app is accessed directly; it is the
installer default. Mount the app at the **root of a host or subdomain**.
Mounting it under a subpath such as `/wgswp/` is not supported.

`TRUSTED_PROXY_HOPS=1` makes rate limiting and login throttling trust the
right-most `X-Forwarded-For` entry. Only enable it when clients cannot reach the
container directly, otherwise they can spoof their address and bypass the
per-IP limits. See the main repository README for nginx, Caddy, and Traefik
examples, including the longer timeouts needed for Steam-backed requests.

`GET /health` is public and is used by the installer, systemd checks, proxies,
and monitoring.

## Container layout

| Path                                | Purpose                                                                  |
| ----------------------------------- | ------------------------------------------------------------------------ |
| `/opt/wgswp/wgswp`                  | Application binary.                                                      |
| `/opt/wgswp/web/public/`            | Static frontend.                                                         |
| `/opt/wgswp/data`                   | Symlink to `/var/lib/wgswp`; the app writes `data/wgswp.log` here.       |
| `/var/lib/wgswp/wgswp.log`          | JSON application log.                                                    |
| `/etc/wgswp/wgswp.env`              | Environment file, `0600 root:root`.                                      |
| `/var/backups/wgswp/`               | Dumps and rollback copies, `0700 root:root`.                             |
| `/root/wgswp.creds`                 | Generated credentials summary, `0600 root:root`.                         |
| `/etc/systemd/system/wgswp.service` | systemd unit.                                                            |
| `/etc/logrotate.d/wgswp`            | Log rotation for `wgswp.log`.                                            |

Do not create an `.env` file in `/opt/wgswp`. Configuration belongs only in
`/etc/wgswp/wgswp.env`. The release payload under `/opt/wgswp` contains no
secrets; the environment file and `/root/wgswp.creds` are readable only by root.
PostgreSQL and `wgswp` are both enabled, so the container returns to service
after a reboot.

## Troubleshooting

- **Already installed:** the installer refuses to overwrite an existing
  `/opt/wgswp/wgswp`. Use `/usr/bin/update`.
- **No installable release:** publish a `v*.*.*` tag with the amd64 and arm64
  archives and `sha256sums.txt`.
- **Service not healthy:** check `systemctl status wgswp` and
  `journalctl -u wgswp -n 100`.
- **PostgreSQL problems:** check `systemctl status postgresql` and
  `journalctl -u postgresql`.
- **IP address changed:** update `APP_ORIGIN` in `/etc/wgswp/wgswp.env` and
  restart `wgswp`.
- **Disk full:** free space in `/var/backups/wgswp` or grow the container disk;
  the update fails before starting a half-deployed release.

## Fork and branch testing

The update helper and the container installer are fetched from the
`deploy/proxmox` directory on `main`:

```text
deploy/proxmox/ct/wgswp.sh
deploy/proxmox/install/wgswp-install.sh
```

Keep these directories siblings. To test a branch or fork, override the base
URL before running the creator:

```bash
COMMUNITY_SCRIPTS_URL="https://raw.githubusercontent.com/<owner>/<repo>/<branch>/deploy/proxmox" \
  bash -c "$(curl -fsSL https://raw.githubusercontent.com/<owner>/<repo>/<branch>/deploy/proxmox/ct/wgswp.sh)"
```

## Differences from community-scripts conventions

These scripts follow community-scripts structure and reuse its engine
(`community-scripts/core`), but they are maintained inside the application
repository. New scripts are submitted to `community-scripts/DevScripts` first
and only later promoted to `community-scripts/ProxmoxVE`, so an upstream
submission has to close the gaps listed below. Each entry names the convention,
what this repository does instead, and why.

### Layout and bootstrap

- **Script location.** Convention: `ct/<app>.sh` and `install/<app>-install.sh`
  at the root of a scripts repository. Here: both files live under
  `deploy/proxmox/`. Reason: the deployment scripts ship with the application.
  Upstream action: copy `ct/wgswp.sh` and `install/wgswp-install.sh` into a
  DevScripts fork; they must stay siblings for the engine to resolve the
  install script.
- **Scripts root.** Convention: DevScripts scripts pin the scripts root with
  `_CS_DEFAULT_URL` on the line after the shebang, pointing at the DevScripts
  raw content path. Here: `ct/wgswp.sh` exports `COMMUNITY_SCRIPTS_URL`
  instead, defaulting to this repository's raw `deploy/proxmox` path, and does
  not set `_CS_DEFAULT_URL`. Reason: without the override, a curl'd fork fetches
  `install/wgswp-install.sh` from the official repository, which does not
  contain it. Upstream action: delete the export and add `_CS_DEFAULT_URL`.
- **File header.** Convention: `Copyright (c) 2021-2026 community-scripts ORG`,
  an `Author:` line, and a license URL pointing at the scripts repository, with
  `Source:` naming the upstream project. Here: the header credits Ehs-Colin and
  links this repository's `LICENSE`; there is no `Author:` line. Upstream
  action: restore the community-scripts attribution block and keep `Source:`
  pointing at this application.
- **`var_testurl`.** Convention: DevScripts scripts set `var_testurl` to the
  issue where the script was tested. Here: not set. Upstream action: add it.

### Naming and platform

- **`APP="WGSWP"`.** Convention: `APP` is the display name shown in the Proxmox
  UI, for example `APP="Bark-Server"`. Here: an abbreviation, because the engine
  derives `NSAPP` from it, and `wgswp` is already the binary, service, database,
  log, and credentials name. Upstream action: if a reviewer requires the full
  product name, every derived path must change together; the short name cannot
  differ from `APP`.
- **`var_arm64` default `yes`.** Convention: leave it unset to ask the user, and
  set `yes` only when arm64 has actually been verified. Here: `yes` is the
  default because the release workflow publishes both architectures, but the
  install/update/rollback flow has only been exercised on one architecture.
  Upstream action: test arm64, or comment the line out so the engine asks.
- **PostgreSQL 18.** Convention: most helpers pin the distribution's version
  (commonly 16 or 17). Here: `PG_VERSION="18"` from PGDG on Debian 13, pinned in
  both the installer and the update path so a major-version jump never happens
  implicitly. Upstream action: confirm PGDG offers 18 for the Debian release
  that upstream supports at submission time.

### Update behaviour

- **`update_script` delegates to `run_os_update`, and the real work lives in
  `update_deb_based`.** Convention: current published scripts put the whole
  update path inside `update_script`. Here: `update_deb_based` is the engine's
  documented OS-family hook (`run_os_update` dispatches to
  `update_deb_based`/`update_alpine`), which requires a recent
  `community-scripts/core`. Upstream action: keep it and mention the engine
  dependency, or inline the function into `update_script`.
- **The update also upgrades OS packages** with `apt-get update` and
  `apt-get -y upgrade`. Convention: most helpers only replace the application.
  Reason: this deployment is intended for a self-hosted container that nobody
  logs into to run unattended upgrades. Upstream action: call it out in the
  pull-request description; reviewers may want it behind a variable.
- **Database backup uses `pg_dump` to `/var/backups/wgswp` with three-dump
  retention, and the binary rollback is a separate copy under
  `/var/backups/wgswp/rollback`.** Convention: helpers call `create_backup` and
  `restore_backup`, which copy files into `/opt/<nsapp>.backup`. Reason: a
  file-level copy of a live PostgreSQL data directory is not a consistent
  snapshot, and the engine's restore would also overwrite a schema the previous
  binary cannot read. Migrations are forward-only, so a dump is restored by hand
  and the automation only puts back the binary and web assets. Upstream action:
  keep; this is the main behavioural difference reviewers need to see.
- **The installer waits for `/health` before reporting success.** Convention:
  helpers usually `systemctl enable --now` and finish. Here: up to 60 seconds of
  polling, then `systemctl status` and `journalctl` output and a non-zero exit.
  Upstream action: keep.

### Runtime layout and secrets

- **Configuration is `/etc/wgswp/wgswp.env`, data is `/var/lib/wgswp`, and the
  service runs as the `wgswp` system user** with `ProtectSystem=strict` and
  `ReadWritePaths=/var/lib/wgswp`. Convention: helpers commonly run as root and
  keep configuration in `/opt/<app>_data/.env`. Reason: `CLEAN_INSTALL=1` wipes
  `/opt/wgswp` on every update, so secrets and user data must live outside it;
  `/opt/wgswp/data` is a symlink into `/var/lib/wgswp` and is preserved with
  `CLEAN_INSTALL_KEEP="data"`. Upstream action: keep; note the coupling on
  `CLEAN_INSTALL_KEEP`.
- **Credentials land in `/root/wgswp.creds`.** The name matches the engine's
  `setup_postgresql_db` default (`~/<application>.creds`), and the installer
  appends the admin and Steam values to the same file. Upstream action: if
  `APP` is renamed, pass `PG_DB_CREDS_FILE` explicitly so a single file is
  written.
- **The installer refuses to run over an existing install** and points at
  `/usr/bin/update`. Convention: install scripts assume a fresh container.
  Upstream action: harmless; can stay.

### Style

- Prompts read into local variables (`steam_api_key`, `admin_username`) guarded
  by `-t 0`, rather than prompting into the exported `var_*` names. Same
  unattended behaviour, different idiom.
- Shared logic is factored into `_wgswp_*` helper functions instead of inlined,
  and `ct/wgswp.sh` declares `STD="${STD:-}"` so shellcheck is clean when the
  file is linted outside the engine.

## Verification record

- Static checks run in this workspace on 2026-11-25: shellcheck 0.10.0 reports
  no findings for both scripts, `bash -n` is clean, and the asset pattern
  `wgswp_*_linux_$(arch_resolve amd64 arm64).tar.gz` matches the published
  `wgswp_v0.1.1_linux_amd64.tar.gz` and `wgswp_v0.1.1_linux_arm64.tar.gz`
  archives plus `sha256sums.txt`.
- Fresh install, update, and rollback were run against a real Proxmox VE host on
  2026-11-25 and completed successfully.
- Not yet recorded: the Proxmox VE and template versions used for that run, an
  arm64 run, and the container-reboot check. Capture them in this section before
  submitting upstream.

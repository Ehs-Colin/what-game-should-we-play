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

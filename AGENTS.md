# Project instructions

## Testing

Use the project test environment for manual and browser verification. Do not
start a scratch PostgreSQL instance and do not run a server on another port.

The environment is `cmd/test/start.sh`: it recreates `wgswp_test`, builds the
server, seeds fixtures, and listens on <http://localhost:3999>. Stop it with
`cmd/test/stop.sh`.

`start.sh` exits 0 without recreating the database when an environment is
already running, so always restart before testing:

```sh
cmd/test/stop.sh
cmd/test/start.sh
```

Run both from the repository root. There is one shared environment: stopping
another agent's server is expected, but leave a freshly started one running
when you finish.

Sign in with the seeded account `testuser` / `test-password-123`. The fixtures
already include every import-job status, users, groups, and common games, so no
custom seeding is needed. See `cmd/test/README.md` for the fixture list.

`go test ./...` is separate: it reads `TEST_DATABASE_URL` as described in the
root README and does not use this environment.

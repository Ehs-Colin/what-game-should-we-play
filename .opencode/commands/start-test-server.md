---
description: Restart the seeded test server for UI review
---

Restart the isolated test environment so its database is recreated, the server
rebuilt, and the fixtures reseeded. Stopping first is required: `start.sh`
alone exits early without reseeding when a server is already running.

!`cmd/test/stop.sh && cmd/test/start.sh`

Report the URL, database name, log path, and stop command from the output. If
the script failed, report the error and the last lines of
`data/testenv/server.log`; do not try alternative setup steps.

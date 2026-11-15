#!/usr/bin/env bash
# Stop the test environment started by cmd/test/start.sh.
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "${script_dir}/../.." && pwd)
run_dir=${TEST_ENV_DIR:-${repo_root}/data/testenv}
pid_file=${run_dir}/server.pid

if [[ ! -f ${pid_file} ]]; then
  echo "No test environment is running (${pid_file} not found)."
  exit 0
fi

pid=$(cat "${pid_file}" 2>/dev/null || true)
if [[ -z ${pid} ]] || ! kill -0 "${pid}" 2>/dev/null; then
  rm -f "${pid_file}"
  echo "Test environment is not running."
  exit 0
fi

kill "${pid}"
for _ in $(seq 1 50); do
  if ! kill -0 "${pid}" 2>/dev/null; then
    break
  fi
  sleep 0.2
done
if kill -0 "${pid}" 2>/dev/null; then
  echo "Server did not stop within 10 seconds; sending SIGKILL." >&2
  kill -9 "${pid}" 2>/dev/null || true
fi

rm -f "${pid_file}"
echo "Test environment stopped. The database is kept; start.sh recreates it on the next run."

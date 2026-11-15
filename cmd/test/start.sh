#!/usr/bin/env bash
# Start an isolated test environment: a fresh database, the server, and seed
# data for reviewing the UI. Stop it with cmd/test/stop.sh.
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd -- "${script_dir}/../.." && pwd)
cd "${repo_root}"

set -a
if [[ -f .env ]]; then
  # shellcheck disable=SC1091
  . ./.env
fi
set +a

port=${TEST_ENV_PORT:-3999}
db_name=${TEST_ENV_DB_NAME:-wgswp_test}
run_dir=${TEST_ENV_DIR:-${repo_root}/data/testenv}
pid_file=${run_dir}/server.pid
log_file=${run_dir}/server.log
binary=${run_dir}/wgswp-server

fail() {
  echo "test env: $*" >&2
  exit 1
}

for command in psql curl go; do
  command -v "${command}" >/dev/null 2>&1 || fail "${command} is required"
done

[[ ${db_name} =~ ^[A-Za-z0-9_]+$ ]] || fail "TEST_ENV_DB_NAME may only contain letters, numbers, and underscores"
[[ ${port} =~ ^[0-9]+$ ]] || fail "TEST_ENV_PORT must be a number"

if [[ -f ${pid_file} ]]; then
  pid=$(cat "${pid_file}" 2>/dev/null || true)
  if [[ -n ${pid} ]] && kill -0 "${pid}" 2>/dev/null; then
    echo "Test environment already running at http://localhost:${port} (pid ${pid})."
    echo "Run cmd/test/stop.sh before starting a new one."
    exit 0
  fi
  rm -f "${pid_file}"
fi

if curl -fsS -o /dev/null --max-time 1 "http://127.0.0.1:${port}/" 2>/dev/null; then
  fail "port ${port} is already in use; set TEST_ENV_PORT to another port"
fi

base_url=${TEST_ENV_BASE_DATABASE_URL:-${GOOSE_DBSTRING:-${DATABASE_URL:-}}}
[[ -n ${base_url} ]] || fail "set GOOSE_DBSTRING or DATABASE_URL in .env, or set TEST_ENV_BASE_DATABASE_URL"

# Derive the review database URL from the base URL by swapping its database
# name. URLs are the common case; a key=value DSN is also accepted.
db_url=${TEST_ENV_DATABASE_URL:-}
if [[ -z ${db_url} ]]; then
  if [[ ${base_url} == *://* ]]; then
    db_url=$(printf '%s' "${base_url}" | sed -E "s#/[^/?]+(\?.*)?\$#/${db_name}\1#")
  elif [[ ${base_url} =~ (^|[[:space:]])dbname=[^[:space:]]* ]]; then
    db_url=$(printf '%s' "${base_url}" | sed -E "s/(^|[[:space:]])dbname=[^[:space:]]*/\1dbname=${db_name}/")
  else
    db_url="${base_url} dbname=${db_name}"
  fi
  [[ ${db_url} != "${base_url}" ]] || fail "could not derive the test database URL; set TEST_ENV_DATABASE_URL"
fi

base_db=$(psql "${base_url}" -Atqc 'select current_database()') || fail "could not connect with the base database URL"
if [[ ${base_db} == "${db_name}" ]]; then
  fail "refusing to drop ${db_name} because it is the base database; choose another TEST_ENV_DB_NAME"
fi

echo "Recreating database ${db_name} ..."
psql "${base_url}" -v ON_ERROR_STOP=1 -q -c "DROP DATABASE IF EXISTS \"${db_name}\" WITH (FORCE)"
psql "${base_url}" -v ON_ERROR_STOP=1 -q -c "CREATE DATABASE \"${db_name}\""

mkdir -p "${run_dir}"
chmod 700 "${run_dir}" 2>/dev/null || true

echo "Building server ..."
go build -o "${binary}" ./cmd/server

echo "Starting server on port ${port} ..."
# Detach into a new session so the server survives the launching shell's
# process group being torn down.
launcher=()
if command -v setsid >/dev/null 2>&1; then
  launcher=(setsid)
fi
DATABASE_URL="${db_url}" PORT="${port}" "${launcher[@]}" "${binary}" >"${log_file}" 2>&1 &
server_pid=$!
echo "${server_pid}" >"${pid_file}"

ready=0
for _ in $(seq 1 120); do
  if ! kill -0 "${server_pid}" 2>/dev/null; then
    tail -n 20 "${log_file}" >&2 || true
    rm -f "${pid_file}"
    fail "server exited during startup"
  fi
  if curl -fsS -o /dev/null --max-time 1 "http://127.0.0.1:${port}/api/dashboard" 2>/dev/null; then
    ready=1
    break
  fi
  sleep 0.5
done
if [[ ${ready} -ne 1 ]]; then
  kill "${server_pid}" 2>/dev/null || true
  rm -f "${pid_file}"
  fail "server did not become ready; see ${log_file}"
fi

echo "Seeding test data ..."
if ! psql "${db_url}" -v ON_ERROR_STOP=1 -q -f "${script_dir}/seed.sql"; then
  kill "${server_pid}" 2>/dev/null || true
  rm -f "${pid_file}"
  fail "seeding failed"
fi

echo
echo "Test environment ready: http://localhost:${port}"
echo "  database: ${db_name}"
echo "  log:      ${log_file}"
echo "  stop:     cmd/test/stop.sh"

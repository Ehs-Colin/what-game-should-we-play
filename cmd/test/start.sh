#!/usr/bin/env bash
# Start an isolated test environment: a fresh database, the server, and seed
# data for reviewing the UI. Stop it with cmd/test/stop.sh.
#
# Usage: cmd/test/start.sh [--nouser]
#
# After seeding, the first real account is created from TEST_USER_USERNAME and
# TEST_USER_PASSWORD in cmd/test/.env. Pass --nouser, or leave either value
# unset, to skip it and keep the default account's forced account creation.
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

# Loaded without export so the credentials are not passed to the server.
if [[ -f ${script_dir}/.env ]]; then
  # shellcheck disable=SC1091
  . "${script_dir}/.env"
fi

port=${TEST_ENV_PORT:-3999}
app_origin=${TEST_ENV_APP_ORIGIN:-http://localhost:${port}}
db_name=${TEST_ENV_DB_NAME:-wgswp_test}
run_dir=${TEST_ENV_DIR:-${repo_root}/data/testenv}
pid_file=${run_dir}/server.pid
log_file=${run_dir}/server.log
binary=${run_dir}/wgswp-server

fail() {
  echo "test env: $*" >&2
  exit 1
}

create_user=1
for arg in "$@"; do
  case ${arg} in
    --nouser | --no-user) create_user=0 ;;
    *) fail "unknown argument: ${arg} (usage: cmd/test/start.sh [--nouser])" ;;
  esac
done

test_username=${TEST_USER_USERNAME:-}
test_password=${TEST_USER_PASSWORD:-}
if [[ ${create_user} -eq 1 && ( -z ${test_username} || -z ${test_password} ) ]]; then
  echo "TEST_USER_USERNAME or TEST_USER_PASSWORD is not set in cmd/test/.env; skipping user creation."
  create_user=0
fi

required_commands=(psql curl go)
if [[ ${create_user} -eq 1 ]]; then
  required_commands+=(jq)
fi
for command in "${required_commands[@]}"; do
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
DATABASE_URL="${db_url}" PORT="${port}" APP_ORIGIN="${app_origin}" "${launcher[@]}" "${binary}" >"${log_file}" 2>&1 &
server_pid=$!
echo "${server_pid}" >"${pid_file}"

ready=0
for _ in $(seq 1 120); do
  if ! kill -0 "${server_pid}" 2>/dev/null; then
    tail -n 20 "${log_file}" >&2 || true
    rm -f "${pid_file}"
    fail "server exited during startup"
  fi
  if curl -fsS -o /dev/null --max-time 1 "http://127.0.0.1:${port}/health" 2>/dev/null; then
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

response_file=${run_dir}/auth-response.json
cookie_jar=${run_dir}/auth-cookies.txt

# POST a JSON body read from stdin, keeping credentials out of the process
# list. Prints the HTTP status; the response body is left in response_file.
api_post() {
  local path=$1 csrf_token=${2:-}
  local headers=(-H "Origin: ${app_origin}" -H "Content-Type: application/json")
  if [[ -n ${csrf_token} ]]; then
    headers+=(-H "X-CSRF-Token: ${csrf_token}")
  fi
  curl -sS --max-time 10 -b "${cookie_jar}" -c "${cookie_jar}" "${headers[@]}" \
    -o "${response_file}" -w '%{http_code}' --data-binary @- \
    "http://127.0.0.1:${port}${path}"
}

# Sign in as the bootstrap default account, then promote it to the real
# account the same way the create-account page does.
create_first_user() {
  local status csrf_token
  rm -f "${cookie_jar}" "${response_file}"

  [[ -n ${DEFAULT_ADMIN_USERNAME:-} && -n ${DEFAULT_ADMIN_PASSWORD:-} ]] || {
    echo "DEFAULT_ADMIN_USERNAME and DEFAULT_ADMIN_PASSWORD must be set in .env" >&2
    return 1
  }

  status=$(jq -nc --arg username "${DEFAULT_ADMIN_USERNAME}" --arg password "${DEFAULT_ADMIN_PASSWORD}" \
    '{username: $username, password: $password}' | api_post /api/auth/login) || return 1
  if [[ ${status} != 200 ]]; then
    echo "default account login returned HTTP ${status}: $(cat "${response_file}" 2>/dev/null)" >&2
    return 1
  fi
  csrf_token=$(jq -r '.csrfToken // empty' "${response_file}")
  [[ -n ${csrf_token} ]] || {
    echo "default account login did not return a CSRF token" >&2
    return 1
  }

  status=$(jq -nc --arg defaultPassword "${DEFAULT_ADMIN_PASSWORD}" --arg username "${test_username}" --arg password "${test_password}" \
    '{defaultPassword: $defaultPassword, username: $username, password: $password}' |
    api_post /api/auth/account "${csrf_token}") || return 1
  if [[ ${status} != 201 ]]; then
    echo "account creation returned HTTP ${status}: $(cat "${response_file}" 2>/dev/null)" >&2
    return 1
  fi
}

if [[ ${create_user} -eq 1 ]]; then
  echo "Creating user ${test_username} ..."
  if ! create_first_user; then
    rm -f "${cookie_jar}" "${response_file}"
    kill "${server_pid}" 2>/dev/null || true
    rm -f "${pid_file}"
    fail "user creation failed"
  fi
  rm -f "${cookie_jar}" "${response_file}"
fi

echo
echo "Test environment ready: ${app_origin}"
echo "  database: ${db_name}"
if [[ ${create_user} -eq 1 ]]; then
  echo "  user:     ${test_username}"
else
  echo "  user:     none (sign in with the default account to create one)"
fi
echo "  log:      ${log_file}"
echo "  stop:     cmd/test/stop.sh"

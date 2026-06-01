#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

PROJECT="${ORION_EXAMPLE_PROJECT:-orion-python-sleep-smoke}"
CORE_PORT="${ORION_EXAMPLE_CORE_PORT:-18999}"
APP_PORT="${ORION_EXAMPLE_APP_PORT:-18080}"
CORE_URL="${ORION_EXAMPLE_CORE_URL:-http://127.0.0.1:${CORE_PORT}}"
APP_URL="${ORION_EXAMPLE_APP_URL:-http://127.0.0.1:${APP_PORT}}"
AGENT_BIN="${ORION_AGENT_BIN:-${REPO_ROOT}/apps/agent/orion-agent}"
STATE_FILE="${ORION_EXAMPLE_STATE:-${REPO_ROOT}/tmp/python-sleep-compose-smoke-state.db}"
CONFIG_FILE="${ORION_EXAMPLE_CONFIG:-${REPO_ROOT}/tmp/python-sleep-compose-smoke-config.yaml}"

ORION_HTTP_PORT="${CORE_PORT}"
ORION_EXAMPLE_APP_PORT="${APP_PORT}"
export ORION_HTTP_PORT ORION_EXAMPLE_APP_PORT

if [ -n "${ORION_EXAMPLE_CORE_IMAGE:-}" ]; then
  ORION_CORE_IMAGE="${ORION_EXAMPLE_CORE_IMAGE}"
  export ORION_CORE_IMAGE
else
  ORION_CORE_IMAGE="orion-core:example-smoke"
  export ORION_CORE_IMAGE
  docker build -f "${REPO_ROOT}/apps/core/Dockerfile" -t "${ORION_CORE_IMAGE}" "${REPO_ROOT}"
fi

compose() {
  docker compose -p "${PROJECT}" -f "${SCRIPT_DIR}/docker-compose.yaml" "$@"
}

reset_demo() {
  compose down -v >/dev/null 2>&1 || true
  rm -f "${STATE_FILE}" "${CONFIG_FILE}"
}

cleanup() {
  if [ "${ORION_EXAMPLE_KEEP:-0}" = "1" ]; then
    return
  fi
  reset_demo
}

wait_for() {
  local label="$1"
  local url="$2"

  for _ in $(seq 1 60); do
    if curl -fsS "${url}" >/dev/null 2>&1; then
      printf '%s ready\n' "${label}"
      return 0
    fi
    sleep 1
  done

  printf 'timed out waiting for %s at %s\n' "${label}" "${url}" >&2
  return 1
}

run_agent_once() {
  local label="$1"
  local expected_health="$2"
  local log_file

  log_file="$(mktemp "${TMPDIR:-/tmp}/orion-example-agent.XXXXXX")"
  printf 'running server collection: %s\n' "${label}"
  if ! "${AGENT_BIN}" \
    -config "${CONFIG_FILE}" \
    -state "${STATE_FILE}" \
    run -once -verbose 2>&1 | tee "${log_file}"; then
    rm -f "${log_file}"
    return 1
  fi

  if ! grep -q "name=python-health .*health=${expected_health}" "${log_file}"; then
    printf 'expected python-health to report %s during %s run\n' "${expected_health}" "${label}" >&2
    rm -f "${log_file}"
    return 1
  fi

  if ! grep -q "monitor report sent: monitor=python-health" "${log_file}"; then
    printf 'expected python-health report to be sent during %s run\n' "${label}" >&2
    rm -f "${log_file}"
    return 1
  fi

  rm -f "${log_file}"
}

mkdir -p "$(dirname "${STATE_FILE}")"
trap cleanup EXIT
reset_demo
sed \
  -e "s#^core_url: .*#core_url: ${CORE_URL}#" \
  -e "s#url: http://127.0.0.1:8080/health#url: ${APP_URL}/health#" \
  -e "s#port: 8080#port: ${APP_PORT}#" \
  "${SCRIPT_DIR}/server-config.yaml" >"${CONFIG_FILE}"

if [ ! -x "${AGENT_BIN}" ]; then
  make -C "${REPO_ROOT}" agent-build VERSION=example-smoke
fi

compose up -d --build
wait_for "python app" "${APP_URL}/health"
wait_for "orion core" "${CORE_URL}/health"

run_agent_once "healthy" "up"

compose exec -T python-app sh -c 'touch /tmp/orion-example-fail'
if curl -fsS "${APP_URL}/health" >/dev/null 2>&1; then
  printf 'python app stayed healthy after fail marker\n' >&2
  exit 1
fi
run_agent_once "failing" "down"

compose exec -T python-app sh -c 'rm -f /tmp/orion-example-fail'
wait_for "python app recovery" "${APP_URL}/health"
run_agent_once "recovered" "up"

printf 'first-run Python sleep demo smoke passed\n'

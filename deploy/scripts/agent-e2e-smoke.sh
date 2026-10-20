#!/usr/bin/env bash
# End-to-end smoke: a real Core and the real agent on this machine, no privileges.
# Enrolls, runs the daemon, and checks what the CLI says about it.
#   make agent-e2e
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK="$(mktemp -d /tmp/orion-e2e.XXXXXX)"   # short path: unix socket paths are limited to ~100 bytes
PORT="${ORION_E2E_PORT:-18999}"
CORE_PID="" AGENT_PID=""
cleanup() {
  [ -z "$AGENT_PID" ] || kill "$AGENT_PID" 2>/dev/null || true
  [ -z "$CORE_PID" ] || kill "$CORE_PID" 2>/dev/null || true
  rm -rf "$WORK"
}
trap cleanup EXIT

fail() { printf 'FAIL %s\n' "$1" >&2; [ -z "${2:-}" ] || printf '%s\n' "$2" >&2; exit 1; }
expect() { # name text output
  printf '%s' "$3" | grep -Fq -- "$2" || fail "$1 (missing: $2)" "$3"
  printf 'ok   %s\n' "$1"
}

(cd "$ROOT/apps/core" && CGO_ENABLED=1 go build -o "$WORK/core" ./cmd/api)
(cd "$ROOT/apps/agent" && CGO_ENABLED=0 go build -o "$WORK/oa" .)

ORION_ALLOW_INSECURE_NO_AUTH=true ORION_DATA_DIR="$WORK/data" ORION_PORT="$PORT" "$WORK/core" >"$WORK/core.log" 2>&1 &
CORE_PID=$!
for _ in $(seq 1 50); do
  python3 -c "import urllib.request,sys; urllib.request.urlopen('http://127.0.0.1:$PORT/health', timeout=1)" 2>/dev/null && break
  sleep 0.2
done
python3 -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:$PORT/health', timeout=2)" || fail "Core did not start" "$(tail -5 "$WORK/core.log")"

A() { "$WORK/oa" --config "$WORK/config.yaml" --state "$WORK/state.db" --socket "$WORK/a.sock" --no-color "$@"; }
export ORION_LOG_PATH="$WORK/agent.log"

out="$(A setup --core-url "http://127.0.0.1:$PORT" --accept-discovered --yes --init-state 2>&1)"
expect "setup writes the config" "Config written" "$out"
out="$(A enroll --core-url "http://127.0.0.1:$PORT" --token e2e 2>&1)"
expect "enroll registers the host" "Enrolled as" "$out"
out="$(A enroll --core-url "http://127.0.0.1:$PORT" --token e2e 2>&1)"
expect "enrolling again does nothing" "Already enrolled" "$out"

A run >"$WORK/run.log" 2>&1 &
AGENT_PID=$!
for _ in $(seq 1 60); do A status --short 2>/dev/null | grep -q connected && break; sleep 0.5; done

out="$(A status 2>&1)"
expect "status says connected" "connected to" "$out"
out="$(A status --short 2>&1)"
expect "status --short is one line" "connected" "$out"
[ "$(printf '%s\n' "$out" | wc -l | tr -d ' ')" = "1" ] || fail "status --short must be one line" "$out"
out="$(A status --json 2>&1)"
expect "status --json is JSON" '"registered": true' "$out"
out="$(A doctor 2>&1 || true)"
expect "doctor sees the scan" "Last scan" "$out"
expect "doctor reaches Core" "Core reachable" "$out"
out="$(A discover --json 2>&1)"
expect "discover --json lists items" '"items"' "$out"
out="$(A maintenance down "e2e" --for 1h 2>&1)"
expect "maintenance on" "Maintenance on" "$out"
expect "Core was told" "Core told" "$out"
out="$(A maintenance up 2>&1)"
expect "maintenance off" "Maintenance off" "$out"
out="$(A usage --window 1s --top 3 2>&1)"
expect "usage lists apps" "cpu" "$out"
out="$(A config show 2>&1)"
expect "config show names the file" "$WORK/config.yaml" "$out"
out="$(A config validate 2>&1)"
expect "config validate passes" "Config valid" "$out"
printf 'agent e2e smoke passed.\n'

#!/usr/bin/env bash
# End-to-end smoke: a real Core and the real node on this machine, no privileges.
# Enrolls, runs the daemon, and checks what the CLI says about it.
#   make node-e2e
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORK="$(mktemp -d /tmp/orion-e2e.XXXXXX)"   # short path: unix socket paths are limited to ~100 bytes
PORT="${ORION_E2E_PORT:-18999}"
CORE_PID="" NODE_PID=""
cleanup() {
  [ -z "$NODE_PID" ] || kill "$NODE_PID" 2>/dev/null || true
  [ -z "$CORE_PID" ] || kill "$CORE_PID" 2>/dev/null || true
  [ -z "$NODE_PID" ] || wait "$NODE_PID" 2>/dev/null || true
  [ -z "$CORE_PID" ] || wait "$CORE_PID" 2>/dev/null || true
  rm -rf "$WORK"
}
trap cleanup EXIT

fail() { printf 'FAIL %s\n' "$1" >&2; [ -z "${2:-}" ] || printf '%s\n' "$2" >&2; exit 1; }
expect() { # name text output
  printf '%s' "$3" | grep -Fq -- "$2" || fail "$1 (missing: $2)" "$3"
  printf 'ok   %s\n' "$1"
}

(cd "$ROOT/apps/core" && CGO_ENABLED=1 go build -o "$WORK/core" ./cmd/api)
(cd "$ROOT/apps/node" && CGO_ENABLED=0 go build -o "$WORK/oa" .)

ORION_ALLOW_INSECURE_NO_AUTH=true ORION_DATA_DIR="$WORK/data" ORION_PORT="$PORT" "$WORK/core" >"$WORK/core.log" 2>&1 &
CORE_PID=$!
healthy() { curl -fsS --max-time 1 "http://127.0.0.1:$PORT/health" >/dev/null 2>&1; }
for _ in $(seq 1 50); do
  kill -0 "$CORE_PID" 2>/dev/null || break   # Core exited: the port is probably taken
  healthy && break
  sleep 0.2
done
kill -0 "$CORE_PID" 2>/dev/null && healthy || fail "Core did not start on port $PORT (is it in use? set ORION_E2E_PORT)" "$(tail -5 "$WORK/core.log")"

A() { "$WORK/oa" --config "$WORK/config.yaml" --state "$WORK/state.db" --socket "$WORK/a.sock" --no-color "$@"; }
export ORION_LOG_PATH="$WORK/node.log"

out="$(A setup --core-url "http://127.0.0.1:$PORT" --accept-discovered --yes --init-state 2>&1)"
expect "setup writes the config" "Config written" "$out"
out="$(A enroll --core-url "http://127.0.0.1:$PORT" --token e2e 2>&1)"
expect "enroll registers the host" "Enrolled as" "$out"
out="$(A enroll --core-url "http://127.0.0.1:$PORT" --token e2e 2>&1)"
expect "enrolling again does nothing" "Already enrolled" "$out"

"$WORK/oa" --config "$WORK/config.yaml" --state "$WORK/state.db" --socket "$WORK/a.sock" --no-color run >"$WORK/run.log" 2>&1 &
NODE_PID=$!
for _ in $(seq 1 60); do A status --short 2>/dev/null | grep -q connected && break; sleep 0.5; done

out="$(A status 2>&1)"
expect "status says connected" "connected to" "$out"
out="$(A status --short 2>&1)"
expect "status --short is one line" "connected" "$out"
[ "$(printf '%s\n' "$out" | wc -l | tr -d ' ')" = "1" ] || fail "status --short must be one line" "$out"
out="$(A status --json 2>&1)"
expect "status --json is JSON" '"registered": true' "$out"
NODE_ID="$(curl -fsS "http://127.0.0.1:$PORT/v1/nodes" | grep -o '"id":"node-[^"]*"' | head -1 | cut -d'"' -f4)"
[ -n "$NODE_ID" ] || fail "Core lists no node"
for _ in $(seq 1 60); do
  curl -fsS "http://127.0.0.1:$PORT/v1/nodes/$NODE_ID/discovery" | grep -q '"items":\[{' && break
  sleep 0.5
done
out="$(curl -fsS "http://127.0.0.1:$PORT/v1/nodes/$NODE_ID/discovery")"
expect "Core received the discovery scan" '"received_at"' "$out"
expect "the scan lists what was found" '"items":[{' "$out"
out="$(A doctor 2>&1 || true)"
expect "doctor sees the scan" "Last scan" "$out"
expect "doctor reaches Core" "Core reachable" "$out"
out="$(A discover --json 2>&1)"
expect "discover --json lists items" '"id":' "$out"
out="$(A maintenance down "e2e" --for 1h 2>&1)"
expect "maintenance on" "Maintenance on" "$out"
expect "Core was told" "Core told" "$out"
out="$(A maintenance up 2>&1)"
expect "maintenance off" "Maintenance off" "$out"
out="$(A usage --window 1s --top 3 2>&1)"
expect "usage lists apps" "name cpu memory" "$out"
[ "$(printf '%s\n' "$out" | wc -l | tr -d ' ')" -ge 2 ] || fail "usage printed no app rows" "$out"
out="$(A config show 2>&1)"
expect "config show names the file" "$WORK/config.yaml" "$out"
out="$(A config validate 2>&1)"
expect "config validate passes" "Config valid" "$out"
kill "$NODE_PID" "$CORE_PID" 2>/dev/null || true
wait "$NODE_PID" "$CORE_PID" 2>/dev/null || true
NODE_PID="" CORE_PID=""
if pgrep -f "$WORK" >/dev/null 2>&1; then fail "processes started by the smoke are still running" "$(pgrep -fl "$WORK")"; fi
printf 'node e2e smoke passed.\n'

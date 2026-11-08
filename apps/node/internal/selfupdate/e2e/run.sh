#!/usr/bin/env bash
# End-to-end test of updating a Node from the Console, with real processes: a real Core, a real Node
# supervised like systemd supervises it, and a local mirror of signed releases. About five minutes.
#
#   apps/node/internal/selfupdate/e2e/run.sh
#
# What stands in for the outside world: a loop that restarts the Node when it exits with a failure
# (systemd's Restart=on-failure), a local web server for the release files, and a private signing
# authority instead of Sigstore. The test binaries carry three hooks that release builds do not
# (go build -tags orion_e2e): the mirror address, the trusted authority and a crash after the start
# check. Core is not asked for the update, because it would look the release up on GitHub; the test
# queues the same request Core would, straight into its database.
# Needs: go, sqlite3, python3.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../../../.." && pwd)"
W="$(mktemp -d)"
CORE_PORT=18999 MIRROR_PORT=19100
GOOS_="$(go env GOOS)" GOARCH_="$(go env GOARCH)"
ASSET="orion-node-$GOOS_-$GOARCH_"
PIDS=()
cleanup() { for p in "${PIDS[@]:-}"; do kill "$p" 2>/dev/null || true; done; pkill -f "$W/node/bin/orion-node" 2>/dev/null || true; }
trap cleanup EXIT
fail() { echo "FAIL: $*"; echo "--- supervisor"; cat "$W/supervisor.log" 2>/dev/null; echo "--- node log (tail)"; tail -15 "$W/node.log" 2>/dev/null; exit 1; }
in_use() { python3 -c "import socket,sys;s=socket.socket();s.settimeout(1);sys.exit(0 if s.connect_ex(('127.0.0.1',int(sys.argv[1]))) == 0 else 1)" "$1"; }
for port in "$CORE_PORT" "$MIRROR_PORT"; do
  # A server left over from an earlier run would answer in place of this run's, with other signatures.
  if in_use "$port"; then echo "FAIL: port $port is already in use; stop the leftover server from an earlier run"; exit 1; fi
done
step() { printf '\n== %s\n' "$*"; }
sql() { sqlite3 "$W/core/orion.db" "$@"; }
wait_for() { # description, sql query, expected, seconds
  local end=$((SECONDS + $4)) got=""
  while [ $SECONDS -lt $end ]; do got="$(sql "$2" 2>/dev/null || true)"; [ "$got" = "$3" ] && return 0; sleep 2; done
  fail "$1: wanted '$3', got '$got'"
}
queue() { # id version sha
  sql "insert into node_discovery_requests (id,node_id,type,payload,status,result,created_by,created_at,expires_at) values ('$1','$NODE','update_node','{\"version\":\"$2\",\"tag\":\"$2\",\"sha256\":\"$3\"}','queued','','e2e',datetime('now'),datetime('now','+1 day'));"
}
release() { # version binary signing-authority-dir -> prints sha256
  (cd "$ROOT/apps/node/internal/selfupdate/e2e" && go run mkrelease.go -dir "$W/release" -ca "$W/$3" -version "$1" -binary "$2" -asset "$ASSET")
}

step "build"
mkdir -p "$W/build" "$W/node/bin" "$W/node/state" "$W/node/etc" "$W/core"
build_node() { (cd "$ROOT/apps/node" && go build -tags orion_e2e -ldflags "-X orion/node/internal.Version=$1" -o "$W/build/node-$1" .); }
for v in 0.9.2 0.9.3 0.9.4; do build_node "$v"; done
(cd "$ROOT/apps/core" && go build -ldflags "-X orion/core/internal/buildinfo.Version=0.9.3" -o "$W/build/core" ./cmd/api)
cp "$W/build/node-0.9.2" "$W/node/bin/orion-node"

step "start the mirror, Core and a supervised Node 0.9.2"
SUM3="$(release 0.9.3 "$W/build/node-0.9.3" ca)"
SUM4="$(release 0.9.4 "$W/build/node-0.9.4" ca)"
(cd "$W/release" && exec python3 -m http.server "$MIRROR_PORT" --bind 127.0.0.1 >"$W/mirror.log" 2>&1) & PIDS+=($!)
ORION_DATA_DIR="$W/core" ORION_LISTEN_HOST=127.0.0.1 ORION_PORT="$CORE_PORT" ORION_REQUIRE_FRONTEND_AUTH=true ORION_ADMIN_USERNAME=admin ORION_ADMIN_PASSWORD=e2e-admin-password-2026 \
  ORION_JWT_SECRET=e2e-jwt-secret-for-the-update-test-2026 ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET=e2e-subscriber-secret-2026 \
  "$W/build/core" >"$W/core.log" 2>&1 & PIDS+=($!)
for _ in $(seq 30); do python3 -c "import urllib.request;urllib.request.urlopen('http://127.0.0.1:$CORE_PORT/health',timeout=2)" 2>/dev/null && break; sleep 1; done
printf 'core_url: http://127.0.0.1:%s\ninterval: 15s\n' "$CORE_PORT" >"$W/node/etc/config.yaml"
cat >"$W/supervise.sh" <<SUP
#!/bin/bash
export ORION_E2E_CRASH_VERSION=0.9.4 INVOCATION_ID=e2e ORION_E2E_RELEASE_BASE=http://127.0.0.1:$MIRROR_PORT ORION_E2E_ROOTS=$W/ca/roots.pem
n=0
while true; do
  n=\$((n+1)); echo "[supervisor] start \$n: \$($W/node/bin/orion-node version 2>&1)" >>$W/supervisor.log
  $W/node/bin/orion-node run --config $W/node/etc/config.yaml --state $W/node/state/state.db --socket /tmp/orion-e2e-\$\$.sock >>$W/node.log 2>&1; code=\$?
  echo "[supervisor] exit \$code" >>$W/supervisor.log
  [ \$code -eq 0 ] && break; [ \$n -ge 12 ] && break; sleep 3
done
SUP
chmod +x "$W/supervise.sh"; "$W/supervise.sh" & PIDS+=($!)
wait_for "the Node registers" "select count(*) from nodes" 1 60
NODE="$(sql "select id from nodes limit 1")"
wait_for "the Node offers self_update" "select instr(capabilities,'self_update')>0 from node_discoveries" 1 90

step "1. update 0.9.2 -> 0.9.3"
queue dr_1 0.9.3 "$SUM3"
wait_for "the update is reported applied" "select status from node_discovery_requests where id='dr_1'" applied 240
wait_for "Core sees the new version" "select node_version from node_reports order by created_at desc limit 1" 0.9.3 90
[ "$("$W/node/bin/orion-node" version)" = "orion-node 0.9.3" ] || fail "the binary on disk is not 0.9.3"
grep -q "exit 75" "$W/supervisor.log" || fail "the Node did not exit 75 to be restarted"
echo "ok: swapped, restarted by the supervisor, reported applied, Core sees 0.9.3"

step "2. update to 0.9.4, which crashes at start, is rolled back"
queue dr_2 0.9.4 "$SUM4"
wait_for "the failed update is reported" "select status from node_discovery_requests where id='dr_2'" failed 240
[ "$("$W/node/bin/orion-node" version)" = "orion-node 0.9.3" ] || fail "0.9.3 was not restored"
echo "ok: $(sql "select result from node_discovery_requests where id='dr_2'")"

step "3. refusals leave the binary alone"
before="$(shasum -a 256 "$W/node/bin/orion-node" | cut -c1-20)"
S5="$(release 0.9.5 "$W/build/node-0.9.3" ca2)"                       # signed by another authority
S6="$(release 0.9.6 "$W/build/node-0.9.3" ca)"; echo tampered >>"$W/release/0.9.6/$ASSET"   # changed after signing
queue dr_5 0.9.5 "$S5"; queue dr_6 0.9.6 "$S6"; queue dr_7 0.9.3 "0000000000000000000000000000000000000000000000000000000000000000"
for id in dr_5 dr_6; do wait_for "$id is refused" "select status from node_discovery_requests where id='$id'" failed 120; done
sql "select id, result from node_discovery_requests where id in ('dr_5','dr_6')"
[ "$(shasum -a 256 "$W/node/bin/orion-node" | cut -c1-20)" = "$before" ] || fail "a refused update changed the binary"
echo "ok: bad signature and tampered download refused, binary untouched"

step "supervisor log"; cat "$W/supervisor.log"
echo; echo "ALL PASSED"

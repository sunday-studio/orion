#!/bin/sh
# Runs as root only to repair ownership of a data volume created by an older
# (root-running) image, then execs the real command as the orion user. When the
# container is started as a non-root user (for example `user: 10001`), it just execs.
set -eu

DATA_DIR="${ORION_DATA_DIR:-/data}"

if [ "$(id -u)" = "0" ]; then
  orion_uid="$(id -u orion)"
  if [ -d "$DATA_DIR" ] && [ "$(stat -c %u "$DATA_DIR")" != "$orion_uid" ]; then
    echo "docker-entrypoint: taking ownership of $DATA_DIR for the orion user" >&2
    # Best effort: with all capabilities dropped root cannot enter directories owned by
    # another uid, so a partial failure must not crash-loop the container.
    chown -R orion:orion "$DATA_DIR" || echo "docker-entrypoint: warning: could not change ownership of every file in $DATA_DIR" >&2
    chmod 0700 "$DATA_DIR" || true
  fi
  exec su-exec orion "$@"
fi

exec "$@"

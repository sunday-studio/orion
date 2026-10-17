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
    chown -R orion:orion "$DATA_DIR"
    chmod 0700 "$DATA_DIR"
  fi
  exec su-exec orion "$@"
fi

exec "$@"

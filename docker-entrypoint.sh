#!/bin/sh
set -e

MAX_RESTARTS=10
RESTART_DELAY=2
count=0
ENGINE="./engine"

while true; do
	"$ENGINE" "$@"
	code=$?
	[ "$code" -eq 0 ] && exit 0
	count=$((count + 1))
	[ "$count" -ge "$MAX_RESTARTS" ] && exit "$code"
	echo "[entrypoint] engine exited ($code), restart $count in ${RESTART_DELAY}s"
	sleep "$RESTART_DELAY"
done

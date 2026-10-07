#!/bin/sh
set -eu

attempt=0
until migrate; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 15 ]; then
    echo "migrations failed" >&2
    exit 1
  fi
  echo "waiting for database" >&2
  sleep 2
done

exec api

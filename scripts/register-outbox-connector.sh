#!/usr/bin/env sh

set -eu

CONNECT_URL="${CONNECT_URL:-http://localhost:8083}"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
CONNECTOR_FILE="${SCRIPT_DIR}/../deployments/local/connectors/outbox-postgres-connector.json"

response_file="$(mktemp)"
status_code="$(
  curl -sS -o "$response_file" -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    --data @"$CONNECTOR_FILE" \
    "${CONNECT_URL}/connectors"
)"

if [ "$status_code" = "201" ] || [ "$status_code" = "200" ]; then
  echo "outbox connector registered"
elif [ "$status_code" = "409" ]; then
  echo "outbox connector already exists"
else
  echo "failed to register outbox connector (status: $status_code)"
  cat "$response_file"
  exit 1
fi

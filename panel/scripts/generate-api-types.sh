#!/usr/bin/env bash
# Generate the typed API client from the gateway's OpenAPI spec.
#
# The gateway exposes its admin OpenAPI document at
# `/admin/v1/openapi.json`. This script downloads it (or reads it from
# a local file) and runs `openapi-typescript` to produce
# `lib/api/types.ts`, which is consumed by `lib/api/client.ts`.
#
# Usage:
#   bash scripts/generate-api-types.sh                    # http://localhost:8080/admin/v1/openapi.json
#   GATEWAY_URL=http://gw:8080 bash scripts/generate-api-types.sh
#   SPEC_FILE=../gateway/openapi.yaml bash scripts/generate-api-types.sh

set -euo pipefail

cd "$(dirname "$0")/.."

OUTPUT="lib/api/types.ts"

if [[ -n "${SPEC_FILE:-}" ]]; then
  echo "Generating ${OUTPUT} from local spec ${SPEC_FILE}…"
  npx --yes openapi-typescript "${SPEC_FILE}" -o "${OUTPUT}"
else
  GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"
  SPEC_URL="${GATEWAY_URL%/}/admin/v1/openapi.json"
  echo "Generating ${OUTPUT} from ${SPEC_URL}…"
  npx --yes openapi-typescript "${SPEC_URL}" -o "${OUTPUT}"
fi

echo "Wrote ${OUTPUT}."
echo "Note: this file is gitignored and must be regenerated on each clone."

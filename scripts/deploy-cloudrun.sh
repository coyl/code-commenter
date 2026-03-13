#!/usr/bin/env bash
# Deploy API and frontend to Google Cloud Run and wire URLs (API URL in frontend, CORS on API).
# Usage: ./scripts/deploy-cloudrun.sh [GCP_PROJECT_ID]
#   If GCP_PROJECT_ID is given, switches gcloud to that project before deploying.
# Runs deploy-cloudrun-api.sh, then deploy-cloudrun-web.sh (frontend deploy + CORS update with EXTRA_ALLOWED_ORIGINS).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/deploy-cloudrun-api.sh" "$@"
"$SCRIPT_DIR/deploy-cloudrun-web.sh" "$@"

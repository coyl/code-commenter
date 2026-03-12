#!/usr/bin/env bash
# Deploy API and frontend to Google Cloud Run and wire URLs (API URL in frontend, CORS on API).
# Usage: ./scripts/deploy-cloudrun.sh [GCP_PROJECT_ID]
#   If GCP_PROJECT_ID is given, switches gcloud to that project before deploying.
# Runs deploy-cloudrun-api.sh, then deploys the frontend, then sets the API's ALLOWED_ORIGINS.

set -e

REGION="${REGION:-europe-west1}"
API_SERVICE="code-commenter-api"
WEB_SERVICE="code-commenter-web"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load .env.prod for API_URL resolution and web deploy
if [[ -f "$ROOT_DIR/.env.prod" ]]; then
  echo "Loading env from .env.prod"
  set -a
  # shellcheck source=/dev/null
  source "$ROOT_DIR/.env.prod"
  set +a
fi

"$SCRIPT_DIR/deploy-cloudrun-api.sh" "$@"

PROJECT_ID="$(gcloud config get-value project 2>/dev/null)"
if [[ -z "$PROJECT_ID" ]]; then
  echo "Error: No GCP project set. Run: gcloud config set project YOUR_PROJECT_ID" >&2
  exit 1
fi
PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)' 2>/dev/null)"
if [[ -z "$PROJECT_NUMBER" ]]; then
  echo "Error: Could not resolve GCP project number for '$PROJECT_ID'." >&2
  exit 1
fi

API_URL="$(gcloud run services describe "$API_SERVICE" --region "$REGION" --format='value(status.url)' | sed 's|/$||')"
echo "API URL: $API_URL"

echo "--- Deploying frontend ---"
cd "$ROOT_DIR/web"
gcloud run deploy "$WEB_SERVICE" \
  --source . \
  --region "$REGION" \
  --allow-unauthenticated \
  --port 8080 \
  --set-env-vars "API_URL=$API_URL" \
  --quiet

WEB_URL="https://${WEB_SERVICE}-${PROJECT_NUMBER}.${REGION}.run.app"
echo "Web URL: $WEB_URL"

echo "--- Updating API CORS (ALLOWED_ORIGINS) ---"
gcloud run services update "$API_SERVICE" \
  --region "$REGION" \
  --update-env-vars "ALLOWED_ORIGINS=$WEB_URL" \
  --quiet

echo "--- Done ---"
echo "API:  $API_URL"
echo "Web:  $WEB_URL"

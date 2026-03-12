#!/usr/bin/env bash
# Deploy only the frontend (web) to Google Cloud Run.
# Uses gcloud run deploy --source (same as API; no local Docker required).
# Usage: ./scripts/deploy-cloudrun-web.sh [GCP_PROJECT_ID]
#   If GCP_PROJECT_ID is given, switches gcloud to that project before deploying.
# Requires: API_URL (backend URL). Loaded from .env.prod or env; set as Cloud Run env so the container writes config.json at start.
#   If API_URL is not set, the script tries to read the API service URL from the already-deployed code-commenter-api service.
# After deploy, updates the API service's ALLOWED_ORIGINS to include the new web URL.

set -e

REGION="${REGION:-europe-west1}"
API_SERVICE="code-commenter-api"
WEB_SERVICE="code-commenter-web"
REPO_NAME="code-commenter"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load .env.prod for API_URL (and optional project)
if [[ -f "$ROOT_DIR/.env.prod" ]]; then
  echo "Loading env from .env.prod"
  set -a
  # shellcheck source=/dev/null
  source "$ROOT_DIR/.env.prod"
  set +a
fi

if [[ -n "${1:-}" ]]; then
  echo "Setting GCP project to: $1"
  gcloud config set project "$1"
fi

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

# Resolve API URL: use env/.env.prod or discover from deployed API service
if [[ -z "${API_URL:-}" ]]; then
  echo "API_URL not set; discovering from deployed API service..."
  API_URL="$(gcloud run services describe "$API_SERVICE" --region "$REGION" --format='value(status.url)' 2>/dev/null | sed 's|/$||')"
fi
if [[ -z "$API_URL" ]]; then
  echo "Error: API_URL is not set. Set it in .env.prod (e.g. API_URL=https://code-commenter-api-xxxxx.run.app) or deploy the API first." >&2
  exit 1
fi

echo "Project: $PROJECT_ID  Region: $REGION  API: $API_URL"
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
echo "Web: $WEB_URL"

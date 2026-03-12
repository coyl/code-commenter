#!/usr/bin/env bash
# Deploy only the API (backend) to Google Cloud Run.
# Usage: ./scripts/deploy-cloudrun-api.sh [GCP_PROJECT_ID]
#   If GCP_PROJECT_ID is given, switches gcloud to that project before deploying.
# Loads GEMINI_API_KEY, S3_*, and optional auth/job-index (GOOGLE_CLIENT_*, SESSION_SECRET,
# FIRESTORE_PROJECT_ID, FIRESTORE_DATABASE_ID or DATASTORE_PROJECT_ID) from .env.prod in the repo root if present.
# AUTH_CALLBACK_URL is set automatically to the deployed API URL + /auth/callback when OAuth is configured.

set -e

REGION="${REGION:-europe-west1}"
API_SERVICE="code-commenter-api"
WEB_SERVICE="code-commenter-web"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load .env.prod so GEMINI_API_KEY and S3_* (etc.) are available
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

# Prefer the canonical service URL format over hashed a.run.app URL:
# https://SERVICE-PROJECT_NUMBER.REGION.run.app
PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)' 2>/dev/null)"
if [[ -z "$PROJECT_NUMBER" ]]; then
  echo "Error: Could not resolve GCP project number for '$PROJECT_ID'." >&2
  exit 1
fi

# Resolve ALLOWED_ORIGINS from deployed Cloud Run web service URL.
# We intentionally do not use localhost or .env override here.
if ! gcloud run services describe "$WEB_SERVICE" --region "$REGION" >/dev/null 2>&1; then
  echo "Error: Could not resolve Cloud Run web service URL for '$WEB_SERVICE' in region '$REGION'." >&2
  echo "Deploy the web service first (or run ./scripts/deploy-cloudrun.sh)." >&2
  exit 1
fi
WEB_URL="https://${WEB_SERVICE}-${PROJECT_NUMBER}.${REGION}.run.app"
ALLOWED_ORIGINS="$WEB_URL"
echo "Using ALLOWED_ORIGINS from Cloud Run web URL: $ALLOWED_ORIGINS"

if [[ -z "${GEMINI_API_KEY:-}" ]]; then
  echo "Error: GEMINI_API_KEY is not set. Set it in .env.prod or in the environment." >&2
  exit 1
fi

# Canonical API URL (for AUTH_CALLBACK_URL)
API_BASE_URL="https://${API_SERVICE}-${PROJECT_NUMBER}.${REGION}.run.app"

# Build env file for API service (YAML map format required by gcloud --env-vars-file)
escape_yaml_val() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }
API_ENV_FILE="$(mktemp)"
trap 'rm -f "$API_ENV_FILE"' EXIT
{
  printf '%s: "%s"\n' "GEMINI_API_KEY" "$(escape_yaml_val "${GEMINI_API_KEY}")"
  [[ -n "${ALLOWED_ORIGINS:-}" ]] && printf '%s: "%s"\n' "ALLOWED_ORIGINS" "$(escape_yaml_val "${ALLOWED_ORIGINS}")"
  [[ -n "${S3_BUCKET:-}" ]]    && printf '%s: "%s"\n' "S3_BUCKET"    "$(escape_yaml_val "${S3_BUCKET}")"
  printf '%s: "%s"\n' "S3_REGION" "$(escape_yaml_val "${S3_REGION:-}")"
  [[ -n "${AWS_REGION:-}" ]]   && printf '%s: "%s"\n' "AWS_REGION"   "$(escape_yaml_val "${AWS_REGION}")"
  [[ -n "${S3_ENDPOINT:-}" ]]  && printf '%s: "%s"\n' "S3_ENDPOINT"  "$(escape_yaml_val "${S3_ENDPOINT}")"
  [[ -n "${S3_ACCESS_KEY:-}" ]] && printf '%s: "%s"\n' "S3_ACCESS_KEY" "$(escape_yaml_val "${S3_ACCESS_KEY}")"
  [[ -n "${S3_SECRET_KEY:-}" ]] && printf '%s: "%s"\n' "S3_SECRET_KEY" "$(escape_yaml_val "${S3_SECRET_KEY}")"
  # Auth (optional: when set in .env.prod, generation requires sign-in)
  [[ -n "${GOOGLE_CLIENT_ID:-}" ]]    && printf '%s: "%s"\n' "GOOGLE_CLIENT_ID"    "$(escape_yaml_val "${GOOGLE_CLIENT_ID}")"
  [[ -n "${GOOGLE_CLIENT_SECRET:-}" ]] && printf '%s: "%s"\n' "GOOGLE_CLIENT_SECRET" "$(escape_yaml_val "${GOOGLE_CLIENT_SECRET}")"
  [[ -n "${GOOGLE_CLIENT_ID:-}" ]]    && printf '%s: "%s"\n' "AUTH_CALLBACK_URL"   "$(escape_yaml_val "${API_BASE_URL}/auth/callback")"
  [[ -n "${SESSION_SECRET:-}" ]]      && printf '%s: "%s"\n' "SESSION_SECRET"      "$(escape_yaml_val "${SESSION_SECRET}")"
  # Job index: Firestore (Native) or Datastore / Firestore in Datastore mode (optional: enables "My jobs" when auth is enabled)
  [[ -n "${FIRESTORE_PROJECT_ID:-}" ]]   && printf '%s: "%s"\n' "FIRESTORE_PROJECT_ID"   "$(escape_yaml_val "${FIRESTORE_PROJECT_ID}")"
  [[ -n "${FIRESTORE_DATABASE_ID:-}" ]]  && printf '%s: "%s"\n' "FIRESTORE_DATABASE_ID"  "$(escape_yaml_val "${FIRESTORE_DATABASE_ID}")"
  [[ -n "${DATASTORE_PROJECT_ID:-}" ]]   && printf '%s: "%s"\n' "DATASTORE_PROJECT_ID"   "$(escape_yaml_val "${DATASTORE_PROJECT_ID}")"
  [[ -n "${DATASTORE_DATABASE_ID:-}" ]]  && printf '%s: "%s"\n' "DATASTORE_DATABASE_ID"  "$(escape_yaml_val "${DATASTORE_DATABASE_ID}")"
} >> "$API_ENV_FILE"

echo "Project: $PROJECT_ID  Region: $REGION"
echo "--- Deploying API ---"
cd "$ROOT_DIR/api"
gcloud run deploy "$API_SERVICE" \
  --source . \
  --region "$REGION" \
  --allow-unauthenticated \
  --env-vars-file "$API_ENV_FILE" \
  --quiet

API_URL="$(gcloud run services describe "$API_SERVICE" --region "$REGION" --format='value(status.url)' | sed 's|/$||')"
echo "--- Done ---"
echo "API URL: $API_URL"

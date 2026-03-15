# Deployment and environment

This document covers **environment variables** and **deployment** options (local, Docker, Cloud Run scripts, production configs, and GitHub Actions). For quick start and running locally, see the [main README](../README.md).

## Environment

### Backend

Create a `.env` in the project root (or set in shell):

```bash
# Required for backend
export GEMINI_API_KEY="your-gemini-api-key"
# Or: export GOOGLE_API_KEY="your-key"

# Optional
export PORT=8080
export GEMINI_MODEL=gemini-3-flash-preview
export ALLOWED_ORIGINS=http://localhost:3010
# TTS: default is one batched call per task (saves RPD). Set to "on" for one TTS request per segment.
# export TTS_PER_SEGMENT=on
# Model for audio timestamp detection in batched TTS (default: gemini-2.5-flash).
# export TIMESTAMP_MODEL=gemini-2.5-flash

# Optional: Google OAuth + session (when set, generation requires sign-in)
# export GOOGLE_CLIENT_ID=your-oauth-client-id
# export GOOGLE_CLIENT_SECRET=your-oauth-client-secret
# AUTH_CALLBACK_URL = frontend OAuth callback (Google redirects here; register this in Google Cloud Console)
#   Local: http://localhost:3010/auth/callback   Prod: https://your-app-domain.com/auth/callback
# export AUTH_CALLBACK_URL=https://your-app-domain.com/auth/callback
# export SESSION_SECRET=at-least-32-byte-random-string-for-cookie-signing
# To disable auth even when OAuth vars are set: export DISABLE_AUTH=yes

# Optional: Job index for "My jobs" (use one when auth is enabled)
# Firestore (Native mode):
# export FIRESTORE_PROJECT_ID=your-gcp-project-id
# export FIRESTORE_DATABASE_ID=your-database-id   # optional; omit for (default)
# Datastore / Firestore in Datastore mode (use when Firestore API is not available):
# export DATASTORE_PROJECT_ID=your-gcp-project-id
# export DATASTORE_DATABASE_ID=code-commenter   # named database; omit for (default)
```

### Frontend

For the frontend, create `web/.env.local` (optional):

```bash
NEXT_PUBLIC_API_URL=http://localhost:8090
```

The frontend also reads the API URL at **runtime** from `web/public/config.json`. That file is fetched in the browser on first use; if missing or invalid, the app falls back to `NEXT_PUBLIC_API_URL` or `http://localhost:8080`. For local dev, the repo includes a `config.json` pointing at localhost. For production, you can overwrite `config.json` at container start so one build works for any backend URL.

---

## Deploy both to Cloud Run (staging — fast path)

The scripts in `scripts/` are a **fast solution for staging deployments**: they use `gcloud run deploy --source` (no pre-built images), load env from `.env.prod`, and wire API ↔ web URLs and CORS. For production, prefer the **declarative configs** in `deploy/` with images built by CI (see [GitHub Actions](#github-actions-build-push-deploy) and [Deploy configs](#deploy-configs-production-style)).

From the repo root, create a `.env.prod` file (not committed; see `.gitignore`) with at least `GEMINI_API_KEY` and optionally S3 and other API env vars:

```bash
# .env.prod (required)
GEMINI_API_KEY=your-gemini-api-key

# Optional: S3 for job storage
S3_BUCKET=your-bucket
S3_REGION=eu-central-1
# S3_ACCESS_KEY=...
# S3_SECRET_KEY=...
# S3_ENDPOINT=...   # e.g. for MinIO
```

Then run:

```bash
./scripts/deploy-cloudrun.sh
```

The script loads `.env.prod`, deploys the API (with those env vars), then the frontend (with the API URL inlined), then sets the API's `ALLOWED_ORIGINS` to the frontend URL. Optional: pass a GCP project ID to switch to that project before deploying:

```bash
./scripts/deploy-cloudrun.sh my-gcp-project-id
```

Requires `gcloud` CLI and an existing GCP project (builds run in Cloud Build; local Docker not required). The script uses region `europe-west1` (Frankfurt) unless you set `REGION` in the environment.

To deploy only the frontend (e.g. after changing the web app or switching API URL), use `./scripts/deploy-cloudrun-web.sh [GCP_PROJECT_ID]`. It needs the API base URL: set `API_URL` in `.env.prod` or in the environment, or have the API already deployed in the same project/region so the script can discover it. To allow a custom domain (e.g. `https://code.vasiliy.pro`) in CORS, set `EXTRA_ALLOWED_ORIGINS` in `.env.prod` (comma-separated list); the script merges it with the Cloud Run web URL when updating the API's `ALLOWED_ORIGINS`.

**Secrets:** Any YAML in `deploy/` that holds secret values (e.g. for local or CI use) is **encrypted with [SOPS](https://github.com/getsops/sops)**. Decrypt with `sops -d deploy/<file>.yaml` before use; do not commit decrypted secret files.

---

## Deploy backend to Google Cloud Run (manual)

```bash
cd api
gcloud run deploy code-commenter-api \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars GEMINI_API_KEY=your-key
```

Then set `NEXT_PUBLIC_API_URL` to your Cloud Run URL when building/serving the frontend.

---

## Deploy frontend to Google Cloud Run (manual)

The frontend reads the API URL at **runtime** from `/config.json` (see [Environment](#frontend) above). You can deploy one image and set the backend URL when the container starts.

**Option A — Runtime config (recommended):** Build the image once (no API URL at build time). At container start, write `public/config.json` with your backend URL, then start the app. Example entrypoint:

```bash
# Before starting the Node server, write config from env
echo "{\"apiUrl\":\"${API_URL:-http://localhost:8080}\"}" > /app/public/config.json
exec node server.js
```

Set Cloud Run env var `API_URL` to your backend URL (e.g. `https://code-commenter-api-xxxxx.run.app`). The Dockerfile can be updated to copy a small entrypoint script that does the above, or you use a custom image that writes config then runs `node server.js`.

**Option B — Build-time URL:** Build with the API URL inlined (same as before):

```bash
cd web
export NEXT_PUBLIC_API_URL=https://code-commenter-api-xxxxx.run.app
docker build --build-arg NEXT_PUBLIC_API_URL="${NEXT_PUBLIC_API_URL}" -t code-commenter-web .
# Tag, push to Artifact Registry, then deploy (see below)
```

**Deploy the image:**

```bash
# Tag for Artifact Registry (replace PROJECT_ID)
docker tag code-commenter-web us-central1-docker.pkg.dev/PROJECT_ID/code-commenter/code-commenter-web:latest
gcloud auth configure-docker us-central1-docker.pkg.dev
docker push us-central1-docker.pkg.dev/PROJECT_ID/code-commenter/code-commenter-web:latest

gcloud run deploy code-commenter-web \
  --image us-central1-docker.pkg.dev/PROJECT_ID/code-commenter/code-commenter-web:latest \
  --region us-central1 \
  --allow-unauthenticated \
  --port 3000
```

- **Artifact Registry:** Create a repository if needed: `gcloud artifacts repositories create code-commenter --repository-format=docker --location=us-central1`.
- **CORS:** Set the API's `ALLOWED_ORIGINS` to include your frontend's Cloud Run URL.

---

## Deploy configs (production-style)

The **`deploy/`** directory holds Cloud Run–oriented configs for production-style deployments:

- **`deploy/env.prod.yaml`** — Stub env vars and Secret Manager secret links (no real values). Use with `gcloud run deploy --env-vars-file` (after substituting stubs) and `--set-secrets` for values from [Google Secret Manager](https://cloud.google.com/secret-manager). See `deploy/README.md` for usage.
- **`deploy/secrets-to-create.md`** — List of secrets to create in Secret Manager; create these once per project, then reference them in `env.prod.yaml` or in Cloud Run service configs.
- Any YAML in `deploy/` that contains actual secret values is **encrypted with SOPS**; decrypt before use.

Images for these configs are built by the GitHub Actions workflow below and stored in Google Artifact Registry.

---

## GitHub Actions (build, push, deploy)

A GitHub Actions workflow builds the API and web Docker images, pushes them to **Google Artifact Registry**, and **deploys** both services to Cloud Run (mirroring `scripts/deploy-cloudrun-api.sh` and `scripts/deploy-cloudrun-web.sh`).

- **Workflow:** [`.github/workflows/build-push-images.yaml`](../.github/workflows/build-push-images.yaml)
- **Triggers:** Push to `main`, or manual `workflow_dispatch`.
- **Secrets (repository):** `GCP_PROJECT_ID`, `GCP_SA_KEY` (service account with Artifact Registry Writer, Cloud Run Admin, Service Account User, and Secret Manager Secret Accessor for API secrets). Alternatively use [Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation) and set `GCP_WORKLOAD_IDENTITY_PROVIDER` / `GCP_SERVICE_ACCOUNT`.
- **Deploy:** API is deployed with `GEMINI_API_KEY` from Secret Manager (`code-commenter-gemini-api-key`); optional env (S3, Firestore, OAuth) via repository **variables** (`S3_BUCKET`, `S3_REGION`, `S3_ENDPOINT`, `AWS_REGION`, `FIRESTORE_PROJECT_ID`, `GOOGLE_CLIENT_ID`, `EXTRA_ALLOWED_ORIGINS`, etc.). Create secrets per [deploy/secrets-to-create.md](../deploy/secrets-to-create.md).
- **Output:** Images in `REGION-docker.pkg.dev/PROJECT_ID/code-commenter/code-commenter-api:latest` and `code-commenter-web:latest`; both services deployed to Cloud Run in `REGION`.

# Code Commenter Live Agent

Describe a coding task (text or live voice) and get code with just-in-time streaming and Gemini Live API voiceover. The UI shows live progress stages (e.g. “Generating task spec”, “Generating CSS”, “Generating voiceover”) while the backend streams spec, CSS, code segments, and audio.

## Requirements

- **Gemini 3.1** for all generation (task spec, CSS, code).
- **Gemini Live API** for real-time voice (mandatory).
- **Google Cloud** for hosting the backend.

## Quick start

### 1. Environment

Create a `.env` in the project root (or set in shell):

```bash
# Required for backend
export GEMINI_API_KEY="your-gemini-api-key"
# Or: export GOOGLE_API_KEY="your-key"

# Optional
export PORT=8080
export GEMINI_MODEL=gemini-3-flash-preview
export GEMINI_LIVE_MODEL=gemini-2.5-flash-preview-native-audio-05-20
export ALLOWED_ORIGINS=http://localhost:3000
# TTS: default is one batched call per task (saves RPD). Set to "on" for one TTS request per segment.
# export TTS_PER_SEGMENT=on
# Model for audio timestamp detection in batched TTS (default: gemini-2.5-flash).
# export TIMESTAMP_MODEL=gemini-2.5-flash
```

For the frontend, create `web/.env.local` (optional):

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

The frontend also reads the API URL at **runtime** from `web/public/config.json`. That file is fetched in the browser on first use; if missing or invalid, the app falls back to `NEXT_PUBLIC_API_URL` or `http://localhost:8080`. For local dev, the repo includes a `config.json` pointing at localhost. For production, you can overwrite `config.json` at container start so one build works for any backend URL.

### 2. Run locally

**Backend (Go):**

```bash
cd api
go mod tidy
go run ./cmd/server
```

Server listens on `:8080` by default.

**Frontend (Next.js):**

```bash
cd web
npm install
# If you see peer dependency conflicts: npm install --legacy-peer-deps
npm run dev
```

Open [http://localhost:3000](http://localhost:3000). Enter a task (e.g. “A React counter with increment and decrement”), choose language, click Generate.

### 3. Run with Docker Compose

From the repo root:

```bash
docker-compose up --build
```

- API: [http://localhost:8080](http://localhost:8080)
- Web UI: [http://localhost:3000](http://localhost:3000)

Set `GEMINI_API_KEY` in the environment or in `docker-compose.yaml` (or use a `.env` file and `env_file` in the compose file).

### 4. Deploy both to Cloud Run (script)

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

The script loads `.env.prod`, deploys the API (with those env vars), then the frontend (with the API URL inlined), then sets the API’s `ALLOWED_ORIGINS` to the frontend URL. Optional: pass a GCP project ID to switch to that project before deploying:

```bash
./scripts/deploy-cloudrun.sh my-gcp-project-id
```

Requires `gcloud` CLI and an existing GCP project (builds run in Cloud Build; local Docker not required). The script uses region `europe-west1` (Frankfurt) unless you set `REGION` in the environment.

To deploy only the frontend (e.g. after changing the web app or switching API URL), use `./scripts/deploy-cloudrun-web.sh [GCP_PROJECT_ID]`. It needs the API base URL: set `API_URL` in `.env.prod` or in the environment, or have the API already deployed in the same project/region so the script can discover it.

### 5. Deploy backend to Google Cloud Run (manual)

```bash
cd api
gcloud run deploy code-commenter-api \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars GEMINI_API_KEY=your-key
```

Then set `NEXT_PUBLIC_API_URL` to your Cloud Run URL when building/serving the frontend.

### 6. Deploy frontend to Google Cloud Run (manual)

The frontend reads the API URL at **runtime** from `/config.json` (see Environment above). You can deploy one image and set the backend URL when the container starts.

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
- **CORS:** Set the API’s `ALLOWED_ORIGINS` to include your frontend’s Cloud Run URL.

## Repo layout

- **`api/`** — Go backend: WebSocket GET `/task/stream` (streaming spec/CSS/code/audio + stage events), WebSocket `GET /live` (Live API proxy).
- **`web/`** — Next.js frontend: task input, generation progress (stage labels + %), code view with typing effect, dynamic CSS.
- **`doc/architecture.md`** — Architecture and data flow.

## Architecture diagram

See [doc/architecture.md](doc/architecture.md) for the Mermaid diagram (Browser ↔ Backend ↔ Gemini 3.1 + Gemini Live API).

## API summary

| Method | Path | Description |
|--------|------|-------------|
| GET    | `/task/stream` | WebSocket: stream task with `stage`, `spec`, `css`, `segment`, `audio`, `code_done`, `error`. |
| GET    | `/live` | WebSocket: proxy to Gemini Live API for voice in/out. |

## Embeddable job player

You can embed a previously generated job player on any site using one script.

### Script usage

```html
<div id="my-player"></div>
<script
  src="https://your-web-domain.com/embed-player.js"
  data-code-commenter-embed
  data-job-id="YOUR_JOB_UUID"
  data-target="#my-player"
  data-width="100%"
  data-height="640"
></script>
```

Supported script attributes:

- `data-job-id` (required): job UUID to render.
- `data-target` (optional): CSS selector for mount element. If omitted, a mount node is inserted after the script tag.
- `data-width` (optional): iframe width (`100%`, `900px`, etc). Default `100%`.
- `data-height` (optional): iframe height in px or CSS units. Default `640`.
- `data-min-height` (optional): minimum iframe height. Default `360`.
- `data-autoplay` (optional): `true`/`1` to append autoplay hint.

You can also pass the job id in the script URL query:

```html
<script src="https://your-web-domain.com/embed-player.js?jobId=YOUR_JOB_UUID"></script>
```

### Deployment requirements

- Web app serves `embed-player.js` and the embed route `/embed/{jobId}`.
- Frontend env: `NEXT_PUBLIC_API_URL` points to your API deployment.
- API env: `ALLOWED_ORIGINS` includes your web domain origin (and any local dev origin you need), for example:

```bash
ALLOWED_ORIGINS=https://your-web-domain.com,http://localhost:3000
```

If a job cannot be loaded, the embed route shows an in-frame error message.

For how to run and verify the app locally and in CI, see [doc/testing.md](doc/testing.md).

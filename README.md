# Anee Explainee

**A multi-agent AI system that autonomously transforms coding tasks into immersive, multimodal walkthroughs — with real-time voice, typing animations, and AI-generated visuals.**

Describe a coding task (text or live voice) and the agentic pipeline decomposes it, generates styled code, produces a synchronized voiceover in your chosen language, creates visual assets, and publishes a shareable interactive player — all streamed in real time.

<p align="center">
  <img src="doc/architecture.svg" alt="Anee Explainee — Multi-Agent Architecture" width="960"/>
</p>

## Agentic architecture

The backend runs a **multi-agent orchestration pipeline** where seven specialized agents execute autonomously, each backed by a purpose-selected Gemini model:

| Agent | Gemini Model | Role |
|-------|-------------|------|
| **Spec Agent** | Gemini 3 Flash | Task decomposition — turns a user prompt into a structured spec |
| **Style Agent** | Gemini 3 Flash | Dynamic CSS theming — generates a cohesive code viewer theme |
| **Code Agent** | Gemini 3 Flash | Structured code generation — produces segmented code with schema-constrained JSON output |
| **Narrator Agent** | Gemini 3 Flash | Voiceover scripting — writes per-segment narration in the selected language |
| **Voice Agent** | Gemini TTS + Live API | Audio synthesis — batched TTS with LLM-driven audio timestamp detection for alignment |
| **Visual Agent** | Gemini 3.1 Flash Image | Multimodal image generation — thumbnail + technical illustration (parallel execution) |
| **Story Agent** | Gemini 3 Flash | Article generation — blog-style write-up with embedded interactive player |

The orchestrator chains these agents through an **event-driven architecture** (`EventSink` with typed events: `stage`, `spec`, `css`, `segment`, `audio`, `story`, `visuals`, `code_done`) — streaming results to the frontend in real time over WebSocket. Sub-tasks run with **concurrent execution** (image generation runs in parallel via goroutines with `sync.WaitGroup`; batched TTS uses rate-limited concurrency).

### Key technical patterns

- **Hexagonal / Ports & Adapters architecture** — clean separation of domain logic from infrastructure (`ports/` interfaces, `adapters/` implementations); agents are swappable and testable
- **Schema-constrained structured output** — `genai.Schema` with `ResponseMIMEType: "application/json"` ensures deterministic agentic data flow between pipeline stages
- **Multi-model orchestration** — five distinct Gemini models selected per sub-task for optimal performance (text, code, image, audio, timestamp detection)
- **Human-in-the-loop** — the agent adapts its execution path based on user intent (text vs. voice input, task generation vs. user-code narration mode)
- **Multimodal I/O** — text in, voice in (Live API), code + CSS out, audio out (TTS), image out (generated visuals), HTML out (story article)

## Mandatory tech

- **Gemini 3 Flash** (`gemini-3-flash-preview`) — all text/code/CSS/story generation via Google GenAI SDK (`google.golang.org/genai`). Override with `GEMINI_MODEL`.
- **Gemini Live API** (WebSocket) — real-time bidirectional voice: live voice task input and voiceover output. Override model with `GEMINI_LIVE_MODEL`.
- **Google Cloud** — backend hosted on Cloud Run; Firestore/Datastore for job index and quota; Cloud Build for container images; Google OAuth for auth.

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
export GEMINI_LIVE_MODEL=gemini-2.5-flash-native-audio-preview-12-2025
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

For the frontend, create `web/.env.local` (optional):

```bash
NEXT_PUBLIC_API_URL=http://localhost:8090
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

Open [http://localhost:3000](http://localhost:3000). Enter a task (e.g. "A React counter with increment and decrement"), choose language, click Generate.

### 3. Run with Docker Compose

From the repo root:

```bash
docker-compose up --build
```

- API: [http://localhost:8090](http://localhost:8090) (mapped `8090:8080`)
- Web UI: [http://localhost:3010](http://localhost:3010) (mapped `3010:3000`)

Set `GEMINI_API_KEY` in the environment or in a `.env` file in the repo root.

### 4. Deploy both to Cloud Run (staging — fast path)

The scripts in `scripts/` are a **fast solution for staging deployments**: they use `gcloud run deploy --source` (no pre-built images), load env from `.env.prod`, and wire API ↔ web URLs and CORS. For production, prefer the **declarative configs** in `deploy/` with images built by CI (see [GitHub Actions](#github-actions-build--push-images) and [deploy/](#deploy-configs)).

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
- **CORS:** Set the API's `ALLOWED_ORIGINS` to include your frontend's Cloud Run URL.

### Deploy configs (production-style)

The **`deploy/`** directory holds Cloud Run–oriented configs for production-style deployments:

- **`deploy/env.prod.yaml`** — Stub env vars and Secret Manager secret links (no real values). Use with `gcloud run deploy --env-vars-file` (after substituting stubs) and `--set-secrets` for values from [Google Secret Manager](https://cloud.google.com/secret-manager). See `deploy/README.md` for usage.
- **`deploy/secrets-to-create.md`** — List of secrets to create in Secret Manager; create these once per project, then reference them in `env.prod.yaml` or in Cloud Run service configs.
- Any YAML in `deploy/` that contains actual secret values is **encrypted with SOPS**; decrypt before use.

Images for these configs are built by the GitHub Actions workflow below and stored in Google Artifact Registry.

### GitHub Actions (build, push, deploy)

A GitHub Actions workflow builds the API and web Docker images, pushes them to **Google Artifact Registry**, and **deploys** both services to Cloud Run (mirroring `scripts/deploy-cloudrun-api.sh` and `scripts/deploy-cloudrun-web.sh`).

- **Workflow:** [`.github/workflows/build-push-images.yaml`](.github/workflows/build-push-images.yaml)
- **Triggers:** Push to `main`, or manual `workflow_dispatch`.
- **Secrets (repository):** `GCP_PROJECT_ID`, `GCP_SA_KEY` (service account with Artifact Registry Writer, Cloud Run Admin, Service Account User, and Secret Manager Secret Accessor for API secrets). Alternatively use [Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation) and set `GCP_WORKLOAD_IDENTITY_PROVIDER` / `GCP_SERVICE_ACCOUNT`.
- **Deploy:** API is deployed with `GEMINI_API_KEY` from Secret Manager (`code-commenter-gemini-api-key`); optional env (S3, Firestore, OAuth) via repository **variables** (`S3_BUCKET`, `S3_REGION`, `S3_ENDPOINT`, `AWS_REGION`, `FIRESTORE_PROJECT_ID`, `GOOGLE_CLIENT_ID`, `EXTRA_ALLOWED_ORIGINS`, etc.). Create secrets per [deploy/secrets-to-create.md](deploy/secrets-to-create.md).
- **Output:** Images in `REGION-docker.pkg.dev/PROJECT_ID/code-commenter/code-commenter-api:latest` and `code-commenter-web:latest`; both services deployed to Cloud Run in `REGION`.

## Repo layout

- **`api/`** — Go backend: agentic orchestrator, WebSocket `GET /task/stream` (streaming spec/CSS/code/audio + stage events), WebSocket `GET /live` (Live API proxy).
- **`web/`** — Next.js frontend: task input, generation progress (stage labels + %), code view with typing effect, dynamic CSS, voice playback, embed player.
- **`scripts/`** — Fast staging deploy scripts (Cloud Run from source).
- **`deploy/`** — Cloud Run configs: env stubs and Secret Manager links (`env.prod.yaml`), list of [secrets to create](deploy/secrets-to-create.md). Secrets YAML in this directory is encrypted with SOPS.
- **`doc/architecture.md`** — Architecture with Mermaid diagram.
- **`doc/architecture.svg`** — Architecture diagram (visual).
- **`doc/testing.md`** — Reviewer guide with sample prompts and checklist.

## Architecture diagram

See [`doc/architecture.svg`](doc/architecture.svg) for the full visual diagram, or the Mermaid source in [`doc/architecture.md`](doc/architecture.md).

## API summary

| Method | Path | Description |
|--------|------|-------------|
| GET    | `/task/stream` | WebSocket: agentic pipeline stream — emits `stage`, `spec`, `css`, `segment`, `audio`, `story`, `visuals`, `code_done`, `error`. Requires auth when OAuth is configured. |
| GET    | `/live` | WebSocket: proxy to Gemini Live API for real-time bidirectional voice I/O. |
| GET    | `/auth/start` | Redirect to Google OAuth. Query: `redirect` (URL to return to after login). |
| GET    | `/auth/callback` | OAuth callback; sets session cookie and redirects. |
| GET    | `/auth/logout` | Clears session cookie and redirects. Query: `redirect`. |
| GET    | `/me` | Returns `{ sub, email, quotaRemaining }` when signed in; 401 otherwise. |
| GET    | `/jobs/mine` | Returns list of current user's jobs `[{ id, title, createdAt }]`. Requires auth. Query: `limit` (default 50). |
| GET    | `/jobs/recent` | Returns recently created jobs across all users (newest first). Query: `limit` (default 20). |
| GET    | `/jobs/{id}` | Returns job by ID (public; used for permalinks and embed). |

## Auth and jobs

When `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `AUTH_CALLBACK_URL`, and `SESSION_SECRET` are all set, the API requires sign-in for generation:

- Set `AUTH_CALLBACK_URL` to the **frontend** OAuth callback (e.g. `http://localhost:3010/auth/callback` or `https://your-app.com/auth/callback`). Google redirects users there after sign-in; the frontend then sends the auth code to the API to complete login. This way the URL shown on the Google sign-in screen is your app's domain.
- Unauthenticated requests to `GET /task/stream` receive 401.
- The frontend shows "Sign in with Google"; after sign-in, the session cookie is sent with requests (use `credentials: 'include'` and ensure `ALLOWED_ORIGINS` matches the web origin so CORS allows credentials).
- To disable auth even when OAuth vars are set, use `DISABLE_AUTH=yes`.

With a job index configured, job metadata (owner, title, createdAt) is written on each upload. The "My jobs" sidebar calls `GET /jobs/mine` to list the current user's jobs.

- **Firestore (Native):** set `FIRESTORE_PROJECT_ID` (and optionally `FIRESTORE_DATABASE_ID`). Create a composite index: collection `jobs`, fields `ownerSub` (Ascending) and `createdAt` (Descending). The Firebase console will prompt with a link when the first query runs.
- **Datastore / Firestore in Datastore mode:** set `DATASTORE_PROJECT_ID` when your database is in Datastore mode (the Cloud Firestore API is not available for that database). Set `DATASTORE_DATABASE_ID` to your named database (e.g. `code-commenter`); omit for the default database. Create a composite index: kind `Job`, properties `ownerSub` (Ascending) and `createdAt` (Descending). Use `gcloud datastore indexes create api/index.yaml --project=YOUR_PROJECT_ID` (add `--database=YOUR_DATABASE_ID` for a named database) or the link from the first-query error in the console.

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

## Gemini models used

| Model | Default ID | Purpose |
|-------|-----------|---------|
| **Gemini 3 Flash** | `gemini-3-flash-preview` | Task spec, CSS, code segments, narration, story, title (`GEMINI_MODEL`) |
| **Gemini 3.1 Flash Image** | `gemini-3.1-flash-image-preview` | Preview thumbnail + illustration image generation |
| **Gemini Live API** | `gemini-2.5-flash-native-audio-preview-12-2025` | Real-time bidirectional voice I/O (`GEMINI_LIVE_MODEL`) |
| **Gemini TTS** | `gemini-2.5-flash-preview-tts` | Batch text-to-speech for voiceover (`GEMINI_TTS_MODEL`) |
| **Gemini 2.5 Flash** | `gemini-2.5-flash` | Audio timestamp detection for segment alignment (`TIMESTAMP_MODEL`) |

## Tech stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.25, `google.golang.org/genai` v1.48.0, Gorilla WebSocket, Chroma syntax highlighter |
| **Frontend** | Next.js 14, React 18, Tailwind CSS, DOMPurify |
| **Cloud** | Google Cloud Run, Firestore / Datastore, Cloud Build, Google OAuth 2.0 |
| **Storage** | S3-compatible (AWS S3 or MinIO) for job persistence |
| **Testing** | Vitest (frontend) |

For how to run and verify the app locally and in CI, see [doc/testing.md](doc/testing.md).

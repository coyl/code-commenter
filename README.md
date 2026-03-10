# Code Commenter Live Agent

A hackathon-compliant web app: describe a coding task (text or live voice), get dynamically generated CSS and code with a typing effect and Gemini Live API voiceover.

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

For the frontend, create `web/.env.local`:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

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

### 4. Deploy backend to Google Cloud Run

```bash
cd api
gcloud run deploy code-commenter-api \
  --source . \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars GEMINI_API_KEY=your-key
```

Then set `NEXT_PUBLIC_API_URL` to your Cloud Run URL when building/serving the frontend.

## Repo layout

- **`api/`** — Go backend: POST `/task`, WebSocket `GET /live` (Live API proxy).
- **`web/`** — Next.js frontend: task input, code view with typing effect, dynamic CSS.
- **`doc/architecture.md`** — Architecture and data flow.

## Architecture diagram

See [doc/architecture.md](doc/architecture.md) for the Mermaid diagram (Browser ↔ Backend ↔ Gemini 3.1 + Gemini Live API).

## API summary

| Method | Path | Description |
|--------|------|-------------|
| POST   | `/task` | Submit task (text), get `id`, `css`, `code`, `spec`, `narration`. |
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

## Hackathon checklist

- [x] Use a Gemini model (Gemini 3.1 for generation).
- [x] Use at least one Google Cloud service (deploy backend on Cloud Run).
- [x] Gemini Live API used for real-time voice (WebSocket proxy).
- [ ] Public repo, README with spin-up instructions (this file).
- [ ] Architecture diagram (see `doc/architecture.md`).
- [ ] &lt;4 min demo video for submission.

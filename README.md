# Code Commenter Live Agent

A hackathon-compliant web app: describe a coding task (text or live voice), get dynamically generated CSS and code with a typing effect and Gemini Live API voiceover, then request changes via text or voice.

## Requirements

- **Gemini 3.1** for all generation (task spec, CSS, code, diff).
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

Open [http://localhost:3000](http://localhost:3000). Enter a task (e.g. “A React counter with increment and decrement”), choose language, click Generate. Use “Request a change” to get updated CSS and code.

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

- **`api/`** — Go backend: POST `/task`, POST `/task/:id/change`, WebSocket `GET /live` (Live API proxy).
- **`web/`** — Next.js frontend: task input, code view with typing effect, dynamic CSS, change loop.
- **`doc/architecture.md`** — Architecture and data flow.

## Architecture diagram

See [doc/architecture.md](doc/architecture.md) for the Mermaid diagram (Browser ↔ Backend ↔ Gemini 3.1 + Gemini Live API).

## API summary

| Method | Path | Description |
|--------|------|-------------|
| POST   | `/task` | Submit task (text), get `id`, `css`, `code`, `spec`, `narration`. |
| POST   | `/task/:id/change` | Send change message, get updated `css`, `code`, `unifiedDiff`. |
| GET    | `/live` | WebSocket: proxy to Gemini Live API for voice in/out. |

## Hackathon checklist

- [x] Use a Gemini model (Gemini 3.1 for generation).
- [x] Use at least one Google Cloud service (deploy backend on Cloud Run).
- [x] Gemini Live API used for real-time voice (WebSocket proxy).
- [ ] Public repo, README with spin-up instructions (this file).
- [ ] Architecture diagram (see `doc/architecture.md`).
- [ ] &lt;4 min demo video for submission.

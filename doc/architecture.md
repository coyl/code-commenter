# Code Commenter Live Agent — Architecture

## Overview

Code Commenter is a hackathon-compliant web app where users describe a coding task (text or live voice), receive dynamically generated CSS and code with a typing effect and optional Gemini Live API voiceover, then request changes via text or voice to get updated CSS and code diffs.

## Mandatory tech

- **Gemini 3 Flash** (`gemini-3-flash-preview` by default) for all text/code/CSS generation (task spec, CSS, code, diff). Override with `GEMINI_MODEL`.
- **Gemini Live API** (WebSocket) for real-time voice: live voice task input and/or voiceover output.
- **Google Cloud** for hosting the backend (e.g. Cloud Run).

## High-level architecture

```mermaid
flowchart LR
  subgraph client [Web Frontend - Next.js]
    UI[Task input]
    View[Code + CSS view]
    VoiceIn[Voice input]
    VoiceOut[Voiceover playback]
  end

  subgraph backend [Backend - Go on Google Cloud]
    API[API / WebSocket]
    Live[Gemini Live API - required]
    Gen[Gemini 3.1]
    Jobs[Session state]
  end

  UI -->|text task| API
  VoiceIn -->|live audio| Live
  API --> Gen
  API --> Live
  Gen -->|CSS + code + script| API
  Live -->|voiceover stream| VoiceOut
  API -->|JSON| View
  Jobs --> API
```

## Data flow

### First run

1. **Task input:** User enters text (or uses live voice via WebSocket to backend → Gemini Live API for transcript).
2. **Backend:** POST `/task` with `{ task, language }`. Backend calls Gemini 3.1 for:
   - Task → structured spec + optional narration script
   - Spec → CSS block
   - Spec + language → full code
3. **Voiceover (Live API):** Optional. Frontend can connect to `GET /live` (WebSocket proxy to Gemini Live API) and send narration text to receive audio stream for playback.
4. **Response:** API returns `{ id, css, code, spec, narration }`. Frontend injects CSS into `#dynamic-theme`, renders code with a typing effect, and can play voiceover via Live WebSocket.

### Change loop

1. **Input:** User types (or speaks via Live) a change request, e.g. “make the button blue”.
2. **Backend:** POST `/task/:id/change` with `{ message }`. Backend sends current CSS, code, and message to Gemini 3.1; receives updated CSS, full new code, and unified diff.
3. **Frontend:** Replaces dynamic CSS and animates code update (typing effect). Optional: request a short “what changed” voiceover via Live API.

## Components

| Component        | Role                                                                 |
|-----------------|----------------------------------------------------------------------|
| **Frontend**    | Next.js app: task form, code view with typing effect, dynamic CSS, Live WebSocket for voice. |
| **Backend**     | Go HTTP server: `POST /task`, `POST /task/:id/change`, `GET /live` (WebSocket proxy to Live API). |
| **Gemini 3.1**  | All generation: spec, CSS, code, change (new CSS + new code + diff). |
| **Live API**    | Real-time voice in/out over WebSocket (mandatory); proxied by backend so API key stays server-side. |
| **Session store** | In-memory store keyed by task `id` (MVP); can be replaced by Firestore/Cloud SQL later. |

## API

- `POST /task` — Body: `{ "task": string, "language": string }`. Returns `{ id, css, code, spec, narration }`.
- `POST /task/:id/change` — Body: `{ "message": string }`. Returns `{ css, code, unifiedDiff }`.
- `GET /live` — WebSocket upgrade. Server proxies to Gemini Live API; client sends/receives Live API message format (setup, realtimeInput, server content).

## Environment

- **Backend:** `GEMINI_API_KEY` or `GOOGLE_API_KEY`, optional `PORT`, `GEMINI_MODEL`, `GEMINI_LIVE_MODEL`, `ALLOWED_ORIGINS`.
- **Frontend:** `NEXT_PUBLIC_API_URL` (backend URL for API and WebSocket).

## Deployment

- Backend: containerize with Dockerfile, deploy to Cloud Run (or GKE).
- Frontend: build `next build`, static export or run on Node/Cloud Run; set `NEXT_PUBLIC_API_URL` to backend URL.

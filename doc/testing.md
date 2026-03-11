# How to test the deployed app (for reviewers)

Use this guide to verify the live deployment: sample prompts and a feature-by-feature checklist.

**Deployed app URL:** *(replace with your frontend URL, e.g. `https://code-commenter-web-xxxxx.run.app`)*

---

## Sample prompts (task mode)

Try these in **Describe a task** with **Generate** to see streaming, progress, and voiceover:

| Prompt | What to check |
|--------|----------------|
| **A React counter with increment and decrement buttons** | Short task; code streams quickly; progress bar moves through stages; voiceover reads the code. |
| **A Python function that takes a list of numbers and returns the sum** | Different language (select Python); narration language can be changed (e.g. German or Spanish). |
| **A TypeScript React component that displays a todo list with add and delete** | Slightly longer output; multiple segments; playback highlights each segment. |
| **A Go HTTP handler that returns JSON with current time** | Backend language (Go); confirms multi-language support. |

**Suggested flow:** Enter the first prompt, choose **JavaScript** and **English**, click **Generate**. Watch the progress stages (e.g. “Generating task spec” → “Generating CSS” → “Generating code segments” → “Generating voiceover” → “Finalizing”), then the code typing effect and voiceover playback.

---

## Features to verify

### 1. Two input modes

- **Describe a task:** Text description + **Language** (JavaScript, TypeScript, Python, Go, PHP, Ruby) + **Narration language** (English, German, Spanish, Italian, Chinese). Click **Generate** for streaming code and voiceover.
- **Your code:** Paste existing code (up to 5,000 characters). Only **Narration language** applies. Click **Generate** to get segmented narration over your code (no new code generation).

**Test:** Switch to **Your code**, paste a few lines of JavaScript, pick a narration language, click **Generate**. You should see “Preparing your code” then segments and voiceover for the pasted code.

### 2. Streaming and progress

- After clicking **Generate**, a **progress bar** and **stage label** appear (e.g. “Generating task spec (25%)”, “Generating code segments (50%)”, “Generating voiceover (75%)”, “Finalizing (100%)”).
- **Code** appears with a **typing effect** as it streams.
- **CSS** and **narration** update as they arrive.

**Test:** Use any sample prompt above; confirm the progress bar advances and the stage text updates before the full code is shown.

### 3. Voiceover (Gemini Live API)

- When streaming finishes, **voiceover** plays in sync with **code segments**: each segment can be played; the UI highlights the active segment.
- **Narration language** controls the voice language (English, German, Spanish, Italian, Chinese).

**Test:** After a stream completes, use play/pause and segment clicks in the code player. Try a second run with a different narration language (e.g. Spanish) and confirm the voice language changes.

### 4. Embeddable player

- After a job completes, the session has a **job ID**. The same run can be viewed in **embed** form:
  - **In-app:** Open `/embed/[jobId]` (job ID is in the URL or session state).
  - **External:** On any page that can load your app’s script: include `embed-player.js` with `data-job-id="YOUR_JOB_ID"` (see README).

**Test:** Complete a **Generate** run, copy the job ID from the URL or embed link, open `/embed/[that-id]` in a new tab. The same code and playback should load.

### 5. Error handling

- Empty task or empty pasted code: **Generate** should not start or should show an error.
- If the backend is unreachable or returns an error, the UI should show an error message instead of hanging.

**Test:** Click **Generate** with an empty task (or empty **Your code**); confirm you get validation or an error. *(Optional: if you have a way to simulate backend failure, confirm the UI shows a connection/error state.)*

---

## Quick checklist (judges)

- [ ] **Task mode:** Sample prompt → **Generate** → progress stages → code typing effect → voiceover plays.
- [ ] **Language:** Change **Language** (e.g. Python) and **Narration language** (e.g. Spanish); confirm output and voice match.
- [ ] **Your code:** Paste code → **Generate** → “Preparing your code” → segments + voiceover for pasted code.
- [ ] **Embed:** After a run, open `/embed/[jobId]` and confirm same code and playback.
- [ ] **Errors:** Empty input or backend issue shows a clear error, no silent failure.

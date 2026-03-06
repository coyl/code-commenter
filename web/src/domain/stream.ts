/**
 * Typed stream events from the API (GET /task/stream WebSocket).
 * Single source of truth for event shapes; adapters parse into these.
 */
export type StreamEvent =
  | { type: "job_started"; id: string }
  | { type: "spec"; spec?: string; narration?: string }
  | { type: "css"; css?: string }
  | { type: "segment"; index: number; code: string; codePlain: string; narration: string }
  | { type: "audio"; data: string }
  | { type: "code_done"; code: string; codePlain: string; rawJson?: string }
  | { type: "session"; id: string }
  | { type: "error"; error: string };

export type StreamEventType = StreamEvent["type"];

/** Segment with index for playback (domain shape used by CodePlayer and job replay). */
export type Segment = {
  index: number;
  code: string;
  codePlain: string;
  narration: string;
  audioChunks: string[];
};

export function isStreamEvent(msg: unknown): msg is StreamEvent {
  if (msg === null || typeof msg !== "object" || !("type" in msg)) return false;
  const t = (msg as { type: string }).type;
  return [
    "job_started",
    "spec",
    "css",
    "segment",
    "audio",
    "code_done",
    "session",
    "error",
  ].includes(t);
}

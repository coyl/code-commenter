import { getWsBase } from "@/config";
import type { StreamPort, StreamConnection } from "@/ports/stream";
import type { StreamEvent } from "@/domain/stream";
import { isStreamEvent } from "@/domain/stream";

export function parseMessage(data: string): StreamEvent | null {
  try {
    const msg = JSON.parse(data) as unknown;
    if (!isStreamEvent(msg)) return null;
    switch (msg.type) {
      case "segment":
        return {
          type: "segment",
          index: msg.index ?? 0,
          code: msg.code ?? "",
          codePlain: msg.codePlain ?? "",
          narration: msg.narration ?? "",
        };
      case "audio":
        return { type: "audio", data: msg.data ?? "" };
      case "code_done":
        return {
          type: "code_done",
          code: msg.code ?? "",
          codePlain: msg.codePlain ?? "",
          rawJson: typeof msg.rawJson === "string" ? msg.rawJson : undefined,
        };
      case "job_started":
        return { type: "job_started", id: msg.id ?? "" };
      case "spec":
        return { type: "spec", spec: msg.spec, narration: msg.narration };
      case "css":
        return { type: "css", css: msg.css };
      case "session":
        return { type: "session", id: msg.id ?? "" };
      case "error":
        return { type: "error", error: msg.error ?? "Stream error" };
      default:
        return null;
    }
  } catch {
    return null;
  }
}

function createConnection(ws: WebSocket): StreamConnection {
  const messageHandlers: Array<(event: StreamEvent) => void> = [];
  const closeHandlers: Array<() => void> = [];
  const errorHandlers: Array<(err: Event) => void> = [];
  const sendQueue: string[] = [];

  ws.onopen = () => {
    for (const msg of sendQueue.splice(0)) ws.send(msg);
  };
  ws.onmessage = (ev) => {
    const raw = typeof ev.data === "string" ? ev.data : "";
    const event = parseMessage(raw);
    if (event) messageHandlers.forEach((h) => h(event));
  };
  ws.onclose = () => closeHandlers.forEach((h) => h());
  ws.onerror = (e) => errorHandlers.forEach((h) => h(e));

  return {
    send(data: unknown) {
      const msg = JSON.stringify(data);
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(msg);
      } else if (ws.readyState === WebSocket.CONNECTING) {
        sendQueue.push(msg);
      }
    },
    onMessage(handler: (event: StreamEvent) => void) {
      messageHandlers.push(handler);
    },
    onClose(handler: () => void) {
      closeHandlers.push(handler);
    },
    onError(handler: (err: Event) => void) {
      errorHandlers.push(handler);
    },
    close() {
      ws.close();
    },
  };
}

export const websocketStreamAdapter: StreamPort = {
  open(path: string): StreamConnection {
    const wsBase = getWsBase();
    if (!wsBase) throw new Error("Cannot determine WebSocket URL");
    const ws = new WebSocket(`${wsBase}${path}`);
    return createConnection(ws);
  },
};

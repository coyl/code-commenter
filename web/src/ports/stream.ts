import type { StreamEvent } from "@/domain/stream";

export type StreamPort = {
  open(path: string): StreamConnection;
};

export type StreamConnection = {
  send(data: unknown): void;
  onMessage(handler: (event: StreamEvent) => void): void;
  onClose(handler: () => void): void;
  onError(handler: (err: Event) => void): void;
  close(): void;
};

export type StreamRequest = {
  task: string;
  language: string;
};

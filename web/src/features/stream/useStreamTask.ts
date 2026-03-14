"use client";

import { useCallback, useRef } from "react";
import { websocketStreamAdapter } from "@/adapters/stream";
import type { Segment } from "@/domain/stream";

export type StreamTaskCallbacks = {
  onCss: (css: string) => void;
  onCode: (code: string) => void;
  onSegments: (segments: Segment[]) => void;
  onSessionId: (id: string | null) => void;
  onNarration: (narration: string) => void;
  onRawJson: (raw: string) => void;
  onStoryHtml: (html: string) => void;
  onError: (err: string | null) => void;
  onLoading: (loading: boolean) => void;
  onStage: (stage: string) => void;
  onStreamEnded: (ended: boolean) => void;
  onNewSegmentIndex: (index: number | null) => void;
  stopAudio: () => void;
  unlockAudio: () => void;
};

export function useStreamTask(callbacks: StreamTaskCallbacks) {
  const callbacksRef = useRef(callbacks);
  callbacksRef.current = callbacks;

  const segmentsRef = useRef<Segment[]>([]);
  const pendingRef = useRef<{ code: string; codePlain: string; narration: string } | null>(null);
  const pendingChunksRef = useRef<string[]>([]);

  const runStream = useCallback(async (task: string, language: string, narrationLanguage: string, userCode?: string) => {
    const {
        onCss,
        onCode,
        onSegments,
        onSessionId,
        onNarration,
        onRawJson,
        onStoryHtml,
        onError,
        onLoading,
        onStage,
        onStreamEnded,
        onNewSegmentIndex,
        stopAudio,
        unlockAudio,
      } = callbacksRef.current;

      onError(null);
      onLoading(true);
      onStage("");
      onCss("");
      onCode("");
      onNarration("");
      onStoryHtml("");
      onSegments([]);
      onRawJson("");
      onStreamEnded(false);
      onNewSegmentIndex(null);
      segmentsRef.current = [];
      pendingRef.current = null;
      pendingChunksRef.current = [];
      stopAudio();
      await Promise.resolve(unlockAudio());

      let conn: Awaited<ReturnType<typeof websocketStreamAdapter.open>> | null = null;
      try {
        conn = await websocketStreamAdapter.open("/task/stream");
      } catch (e) {
        onError(e instanceof Error ? e.message : "Cannot determine WebSocket URL");
        onLoading(false);
        return;
      }

      const flushPending = () => {
        const pending = pendingRef.current;
        const chunks = pendingChunksRef.current;
        if (!pending) return;
        const newIndex = segmentsRef.current.length;
        const newSeg: Segment = {
          index: newIndex,
          code: pending.code,
          codePlain: pending.codePlain,
          narration: pending.narration,
          audioChunks: [...chunks],
        };
        segmentsRef.current = [...segmentsRef.current, newSeg];
        onSegments(segmentsRef.current);
        onNewSegmentIndex(newIndex);
        pendingRef.current = null;
        pendingChunksRef.current = [];
      };

      conn.onMessage((event) => {
        switch (event.type) {
          case "stage":
            onStage("stage" in event && typeof event.stage === "string" ? event.stage : "");
            break;
          case "spec":
            onNarration(event.narration ?? "");
            break;
          case "css":
            onCss(event.css ?? "");
            break;
          case "segment": {
            flushPending();
            pendingRef.current = {
              code: event.code ?? "",
              codePlain: event.codePlain ?? "",
              narration: event.narration ?? "",
            };
            pendingChunksRef.current = [];
            break;
          }
          case "audio":
            if (event.data) pendingChunksRef.current.push(event.data);
            break;
          case "code_done": {
            flushPending();
            pendingRef.current = null;
            pendingChunksRef.current = [];
            const full = (event.code ?? "").trim();
            onCode(full);
            if (typeof event.rawJson === "string") onRawJson(event.rawJson);
            break;
          }
          case "story":
            onStoryHtml(event.storyHtml ?? "");
            onLoading(false);
            break;
          case "session":
            onStreamEnded(true);
            onSessionId(event.id || null);
            break;
          case "error":
            onError(event.error ?? "Stream error");
            onLoading(false);
            conn?.close();
            break;
          default:
            break;
        }
      });

      conn.onClose(() => {
        callbacksRef.current.onLoading(false);
      });
      conn.onError(() => {
        onError("WebSocket error");
        onLoading(false);
      });

      const payload: { task?: string; language: string; code?: string; narration_language?: string } = {
        language,
        narration_language: narrationLanguage || "english",
      };
      if (userCode !== undefined && userCode.trim() !== "") {
        payload.code = userCode.trim();
      } else {
        payload.task = task.trim();
      }
      conn.send(payload);
    },
    []
  );

  return { runStream };
}

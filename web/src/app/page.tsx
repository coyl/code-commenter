"use client";

import React, { useState, useCallback, useRef, useEffect } from "react";
import CodePlayer, { type CodePlayerRef, type Segment as CodePlayerSegment } from "@/components/CodePlayer";
import { usePCMPlayer } from "@/lib/audio";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

function getWsBase(): string {
  if (typeof window === "undefined") return "";
  const u = new URL(API_BASE);
  u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
  return u.origin;
}

type TaskResponse = {
  id: string;
  css: string;
  code: string;
  spec?: string;
  narration?: string;
  voiceoverUrl?: string;
};

type ChangeResponse = {
  css: string;
  code: string;
  unifiedDiff: string;
};

type Segment = CodePlayerSegment;

export default function Home() {
  const [task, setTask] = useState("");
  const [language, setLanguage] = useState("javascript");
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [css, setCss] = useState("");
  const [code, setCode] = useState("");
  const [displayedCode, setDisplayedCode] = useState("");
  const [narration, setNarration] = useState("");
  const [changeMessage, setChangeMessage] = useState("");
  const [loading, setLoading] = useState(false);
  const [changing, setChanging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const styleElRef = useRef<HTMLStyleElement | null>(null);
  const streamCodeBufferRef = useRef("");
  const pendingSegmentRef = useRef<{ code: string; codePlain: string; narration: string } | null>(null);
  const pendingAudioChunksRef = useRef<string[]>([]);
  const newSegmentIndexRef = useRef<number | null>(null);
  const streamEndedRef = useRef(false);
  const codePlayerRef = useRef<CodePlayerRef | null>(null);
  const [segments, setSegments] = useState<Segment[]>([]);
  const [showRawDebug, setShowRawDebug] = useState(false);
  const [rawJsonOutput, setRawJsonOutput] = useState("");
  const { stop: stopAudio, unlock: unlockAudio } = usePCMPlayer();

  // Inject dynamic CSS into the page
  useEffect(() => {
    if (!css) return;
    let el = document.getElementById("dynamic-theme") as HTMLStyleElement | null;
    if (!el) {
      el = document.createElement("style");
      el.id = "dynamic-theme";
      document.head.appendChild(el);
      styleElRef.current = el;
    }
    el.textContent = css;
  }, [css]);

  // When a new segment is added during streaming, tell CodePlayer to play it if it was waiting.
  useEffect(() => {
    const idx = newSegmentIndexRef.current;
    if (idx === null) return;
    newSegmentIndexRef.current = null;
    if (segments.length > idx) codePlayerRef.current?.playSegment(idx);
  }, [segments]);

  // Streaming generate: just-in-time code + voice
  const submitTaskStream = useCallback(() => {
    if (!task.trim()) return;
    setError(null);
    setLoading(true);
    setCss("");
    setCode("");
    setDisplayedCode("");
    setNarration("");
    setSegments([]);
    setRawJsonOutput("");
    streamEndedRef.current = false;
    newSegmentIndexRef.current = null;
    pendingSegmentRef.current = null;
    pendingAudioChunksRef.current = [];
    streamCodeBufferRef.current = "";
    stopAudio();
    unlockAudio();

    const wsBase = getWsBase();
    if (!wsBase) {
      setError("Cannot determine WebSocket URL");
      setLoading(false);
      return;
    }
    const ws = new WebSocket(`${wsBase}/task/stream`);
    ws.onopen = () => {
      ws.send(JSON.stringify({ task: task.trim(), language }));
    };
    ws.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data as string);
        switch (msg.type) {
          case "job_started":
            break;
          case "spec":
            setNarration(msg.narration || "");
            break;
          case "css":
            setCss(msg.css || "");
            break;
          case "segment": {
            const segCode = msg.code ?? "";
            const segPlain = msg.codePlain ?? "";
            const segNarration = msg.narration ?? "";
            const pending = pendingSegmentRef.current;
            const pendingChunks = pendingAudioChunksRef.current;
            if (pending) {
              const completedSeg: Segment = {
                index: 0,
                code: pending.code,
                codePlain: pending.codePlain,
                narration: pending.narration,
                audioChunks: [...pendingChunks],
              };
              setSegments((prev) => {
                const newIndex = prev.length;
                newSegmentIndexRef.current = newIndex;
                const newSeg = { ...completedSeg, index: newIndex };
                return [...prev, newSeg];
              });
            }
            pendingSegmentRef.current = { code: segCode, codePlain: segPlain, narration: segNarration };
            pendingAudioChunksRef.current = [];
            break;
          }
          case "audio":
            if (msg.data) pendingAudioChunksRef.current.push(msg.data);
            break;
          case "code_chunk":
            streamCodeBufferRef.current += msg.chunk || "";
            setDisplayedCode(streamCodeBufferRef.current);
            setCode(streamCodeBufferRef.current);
            break;
          case "code_done": {
            const pending = pendingSegmentRef.current;
            const pendingChunks = pendingAudioChunksRef.current;
            if (pending) {
              const lastSeg: Segment = {
                index: 0,
                code: pending.code,
                codePlain: pending.codePlain,
                narration: pending.narration,
                audioChunks: [...pendingChunks],
              };
              setSegments((prev) => {
                const newIndex = prev.length;
                newSegmentIndexRef.current = newIndex;
                const newSeg = { ...lastSeg, index: newIndex };
                return [...prev, newSeg];
              });
            }
            pendingSegmentRef.current = null;
            pendingAudioChunksRef.current = [];
            const full = (msg.code || "").trim();
            setCode(full);
            setDisplayedCode(full);
            streamCodeBufferRef.current = full;
            if (typeof msg.rawJson === "string") setRawJsonOutput(msg.rawJson);
            break;
          }
          case "session":
            streamEndedRef.current = true;
            setSessionId(msg.id || null);
            setLoading(false);
            ws.close();
            break;
          case "error":
            setError(msg.error || "Stream error");
            setLoading(false);
            ws.close();
            break;
          default:
            break;
        }
      } catch {
        // ignore non-JSON
      }
    };
    ws.onclose = () => {
      setLoading((prev) => (prev ? false : prev));
    };
    ws.onerror = () => {
      setError("WebSocket error");
      setLoading(false);
    };
  }, [task, language, stopAudio, unlockAudio]);

  // Fallback: non-streaming POST /task (no voice)
  const submitTask = async () => {
    if (!task.trim()) return;
    setError(null);
    setLoading(true);
    try {
      const res = await fetch(`${API_BASE}/task`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ task: task.trim(), language }),
      });
      if (!res.ok) {
        const t = await res.text();
        throw new Error(t || res.statusText);
      }
      const data: TaskResponse = await res.json();
      setSessionId(data.id);
      setCss(data.css);
      setCode(data.code);
      setDisplayedCode(data.code);
      setNarration(data.narration || "");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Request failed");
    } finally {
      setLoading(false);
    }
  };

  const submitChange = async () => {
    if (!sessionId || !changeMessage.trim()) return;
    setError(null);
    setChanging(true);
    try {
      const res = await fetch(`${API_BASE}/task/${sessionId}/change`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message: changeMessage.trim() }),
      });
      if (!res.ok) {
        const t = await res.text();
        throw new Error(t || res.statusText);
      }
      const data: ChangeResponse = await res.json();
      setCss(data.css);
      setCode(data.code);
      setDisplayedCode(data.code);
      setChangeMessage("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Change failed");
    } finally {
      setChanging(false);
    }
  };

  return (
    <main className="min-h-screen p-6 max-w-5xl mx-auto">
      <header className="mb-8">
        <h1 className="text-2xl font-bold text-cyan-400">Code Commenter Live Agent</h1>
        <p className="text-zinc-400 text-sm mt-1">Describe a task → get CSS + code with just-in-time streaming and voiceover.</p>
      </header>

      <section className="mb-6 p-4 rounded-lg bg-zinc-900/80 border border-zinc-700">
        <label className="block text-sm font-medium text-zinc-300 mb-2">Task</label>
        <textarea
          className="w-full h-24 px-3 py-2 rounded bg-zinc-800 border border-zinc-600 text-zinc-100 placeholder-zinc-500 focus:ring-2 focus:ring-cyan-500 focus:border-transparent resize-none"
          placeholder="e.g. A React counter component with increment and decrement buttons"
          value={task}
          onChange={(e) => setTask(e.target.value)}
        />
        <div className="flex flex-wrap items-center gap-3 mt-3">
          <select
            className="rounded bg-zinc-800 border border-zinc-600 text-zinc-200 px-3 py-1.5 text-sm"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
          >
            <option value="javascript">JavaScript</option>
            <option value="typescript">TypeScript</option>
            <option value="python">Python</option>
            <option value="go">Go</option>
          </select>
          <button
            onClick={submitTaskStream}
            disabled={loading}
            className="px-4 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-medium text-sm"
          >
            {loading ? "Generating…" : "Generate (stream + voice)"}
          </button>
          <button
            onClick={submitTask}
            disabled={loading}
            className="px-4 py-2 rounded-lg bg-zinc-600 hover:bg-zinc-500 disabled:opacity-50 text-white font-medium text-sm"
          >
            Generate (no voice)
          </button>
        </div>
      </section>

      {error && (
        <div className="mb-4 p-3 rounded bg-red-900/30 border border-red-700 text-red-200 text-sm">
          {error}
        </div>
      )}

      {(css || code) && (
        <>
          <CodePlayer
            ref={codePlayerRef}
            segments={segments}
            displayedCode={displayedCode}
            onDisplayedCodeChange={setDisplayedCode}
            sessionId={sessionId}
            loading={loading}
            streamEndedRef={streamEndedRef}
          />
          {/* Foldable debug: raw segments JSON from LLM */}
          {(code || segments.length > 0) && (
            <div className="mt-4 border border-zinc-700 rounded-lg overflow-hidden bg-zinc-900/50">
              <button
                type="button"
                onClick={() => setShowRawDebug((v) => !v)}
                className="w-full flex items-center justify-between px-3 py-2 text-left text-sm font-medium text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50 transition-colors"
              >
                <span>Raw JSON output (debug)</span>
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                  className={`transition-transform ${showRawDebug ? "rotate-180" : ""}`}
                >
                  <path d="M7 10l5 5 5-5z" />
                </svg>
              </button>
              {showRawDebug && (
                <pre className="p-3 text-xs font-mono whitespace-pre-wrap break-all text-zinc-500 overflow-auto max-h-64 border-t border-zinc-700">
                  {rawJsonOutput || (segments.length > 0 ? segments.map((s) => s.codePlain).join("") : code)}
                </pre>
              )}
            </div>
          )}
        </>
      )}

      {sessionId && (
        <section className="mb-6 p-4 rounded-lg bg-zinc-900/80 border border-zinc-700">
          <label className="block text-sm font-medium text-zinc-300 mb-2">Request a change</label>
          <div className="flex gap-2">
            <input
              type="text"
              className="flex-1 px-3 py-2 rounded bg-zinc-800 border border-zinc-600 text-zinc-100 placeholder-zinc-500 focus:ring-2 focus:ring-cyan-500"
              placeholder="e.g. make the button blue"
              value={changeMessage}
              onChange={(e) => setChangeMessage(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && submitChange()}
            />
            <button
              onClick={submitChange}
              disabled={changing}
              className="px-4 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-medium text-sm"
            >
              {changing ? "Applying…" : "Apply"}
            </button>
          </div>
        </section>
      )}
    </main>
  );
}

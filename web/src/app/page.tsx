"use client";

import React, { useState, useRef, useEffect, useMemo } from "react";
import CodePlayer, { type CodePlayerRef } from "@/components/CodePlayer";
import { usePCMPlayer } from "@/lib/audio";
import type { Segment } from "@/domain/stream";
import { useStreamTask } from "@/features/stream/useStreamTask";
import { useTask } from "@/features/task/useTask";
import { useChange } from "@/features/change/useChange";

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
  const [error, setError] = useState<string | null>(null);
  const [segments, setSegments] = useState<Segment[]>([]);
  const [showRawDebug, setShowRawDebug] = useState(false);
  const [rawJsonOutput, setRawJsonOutput] = useState("");
  const styleElRef = useRef<HTMLStyleElement | null>(null);
  const streamEndedRef = useRef(false);
  const newSegmentIndexRef = useRef<number | null>(null);
  const codePlayerRef = useRef<CodePlayerRef | null>(null);
  const { stop: stopAudio, unlock: unlockAudio } = usePCMPlayer();

  const streamCallbacks = useMemo(
    () => ({
      onCss: setCss,
      onCode: setCode,
      onSegments: setSegments,
      onSessionId: setSessionId,
      onNarration: setNarration,
      onRawJson: setRawJsonOutput,
      onError: setError,
      onLoading: setLoading,
      onStreamEnded: (ended: boolean) => {
        streamEndedRef.current = ended;
      },
      onNewSegmentIndex: (idx: number | null) => {
        newSegmentIndexRef.current = idx;
      },
      stopAudio,
      unlockAudio,
    }),
    [stopAudio, unlockAudio]
  );
  const { runStream } = useStreamTask(streamCallbacks);
  const { runTask, error: taskError } = useTask();
  const { applyChange, changing, error: changeError } = useChange();

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

  useEffect(() => {
    const idx = newSegmentIndexRef.current;
    if (idx === null) return;
    newSegmentIndexRef.current = null;
    if (segments.length > idx) codePlayerRef.current?.playSegment(idx);
  }, [segments]);

  const submitTaskStream = () => {
    if (!task.trim()) return;
    runStream(task.trim(), language);
  };

  const submitTask = async () => {
    if (!task.trim()) return;
    setError(null);
    setLoading(true);
    try {
      const data = await runTask(task.trim(), language || "javascript");
      setSessionId(data.id);
      setCss(data.css);
      setCode(data.code);
      setDisplayedCode(data.code);
      setNarration(data.narration ?? "");
    } catch {
      // error already set by useTask
    } finally {
      setLoading(false);
    }
  };

  const submitChange = async () => {
    if (!sessionId || !changeMessage.trim()) return;
    setError(null);
    const data = await applyChange(sessionId, changeMessage.trim());
    if (data) {
      setCss(data.css);
      setCode(data.code);
      setDisplayedCode(data.code);
      setChangeMessage("");
    }
  };

  const displayError = error ?? changeError ?? taskError ?? null;

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

      {displayError && (
        <div className="mb-4 p-3 rounded bg-red-900/30 border border-red-700 text-red-200 text-sm">
          {displayError}
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

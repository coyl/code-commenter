"use client";

import React, { useState, useRef, useEffect, useMemo } from "react";
import CodePlayer, { type CodePlayerRef } from "@/components/CodePlayer";
import GenerationProgress from "@/components/GenerationProgress";
import { usePCMPlayer } from "@/lib/audio";
import type { Segment } from "@/domain/stream";
import { useStreamTask } from "@/features/stream/useStreamTask";
import { useTask } from "@/features/task/useTask";

type InputTab = "task" | "code";

const NARRATION_LANGUAGES = [
  { value: "english", label: "English" },
  { value: "german", label: "German" },
  { value: "spanish", label: "Spanish" },
  { value: "italian", label: "Italian" },
  { value: "chinese", label: "Chinese (Simplified)" },
] as const;

/** Max characters for the "Your code" paste input (enforced on client; backend truncates segment summary for wrapping narration). */
const MAX_USER_CODE_LENGTH = 5_000;

export default function Home() {
  const [inputTab, setInputTab] = useState<InputTab>("task");
  const [task, setTask] = useState("");
  const [userCode, setUserCode] = useState("");
  const [language, setLanguage] = useState("javascript");
  const [narrationLanguage, setNarrationLanguage] = useState("english");
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [css, setCss] = useState("");
  const [code, setCode] = useState("");
  const [displayedCode, setDisplayedCode] = useState("");
  const [narration, setNarration] = useState("");
  const [loading, setLoading] = useState(false);
  const [stage, setStage] = useState("");
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
      onStage: setStage,
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
  const { runTask, error: taskError, clearError: clearTaskError } = useTask();

  const clearAllErrors = () => {
    setError(null);
    clearTaskError();
  };

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
    if (inputTab === "task" && !task.trim()) return;
    if (inputTab === "code" && !userCode.trim()) return;
    clearAllErrors();
    setDisplayedCode("");
    if (inputTab === "code") {
      runStream("", "", narrationLanguage, userCode.trim());
    } else {
      runStream(task.trim(), language, narrationLanguage);
    }
  };

  const submitTask = async () => {
    if (inputTab === "code") return; // "Generate (no voice)" only for task
    if (!task.trim()) return;
    clearAllErrors();
    setLoading(true);
    try {
      const data = await runTask(task.trim(), language || "javascript", narrationLanguage);
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

  const displayError = error ?? taskError ?? null;

  return (
    <main className="min-h-screen p-6 max-w-5xl mx-auto">
      <header className="mb-8">
        <h1 className="text-2xl font-bold text-cyan-400">Code Commenter Live Agent</h1>
        <p className="text-zinc-400 text-sm mt-1">Describe a task → get code with just-in-time streaming and voiceover.</p>
      </header>

      <section className="mb-6 p-4 rounded-lg bg-zinc-900/80 border border-zinc-700">
        <div className="flex gap-2 mb-2">
          <button
            type="button"
            onClick={() => setInputTab("task")}
            className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
              inputTab === "task"
                ? "bg-cyan-600 text-white"
                : "bg-zinc-800 text-zinc-400 hover:text-zinc-200"
            }`}
          >
            Describe a task
          </button>
          <button
            type="button"
            onClick={() => setInputTab("code")}
            className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
              inputTab === "code"
                ? "bg-cyan-600 text-white"
                : "bg-zinc-800 text-zinc-400 hover:text-zinc-200"
            }`}
          >
            Your code
          </button>
        </div>
        <label className="block text-sm font-medium text-zinc-300 mb-2">
          {inputTab === "task" ? "Task" : "Paste your code"}
        </label>
        {inputTab === "task" ? (
          <textarea
            className="w-full h-24 px-3 py-2 rounded bg-zinc-800 border border-zinc-600 text-zinc-100 placeholder-zinc-500 focus:ring-2 focus:ring-cyan-500 focus:border-transparent resize-none"
            placeholder="e.g. A React counter component with increment and decrement buttons"
            value={task}
            onChange={(e) => setTask(e.target.value)}
          />
        ) : (
          <div>
            <textarea
              className="w-full h-40 px-3 py-2 rounded bg-zinc-800 border border-zinc-600 text-zinc-100 placeholder-zinc-500 focus:ring-2 focus:ring-cyan-500 focus:border-transparent resize-none font-mono text-sm"
              placeholder="Paste your code here. It will be formatted (indentation/newlines only) and split into segments for interactive narration."
              value={userCode}
              maxLength={MAX_USER_CODE_LENGTH}
              onChange={(e) => setUserCode(e.target.value.slice(0, MAX_USER_CODE_LENGTH))}
            />
            <p
              className={`mt-1.5 text-right text-xs ${
                userCode.length >= MAX_USER_CODE_LENGTH
                  ? "text-amber-400"
                  : "text-zinc-500"
              }`}
              aria-live="polite"
            >
              {userCode.length.toLocaleString()} / {MAX_USER_CODE_LENGTH.toLocaleString()} characters
            </p>
          </div>
        )}
        <div className="flex flex-wrap items-center gap-3 mt-3">
          {inputTab === "task" && (
            <select
              className="rounded bg-zinc-800 border border-zinc-600 text-zinc-200 px-3 py-1.5 text-sm"
              value={language}
              onChange={(e) => setLanguage(e.target.value)}
            >
              <option value="javascript">JavaScript</option>
              <option value="typescript">TypeScript</option>
              <option value="python">Python</option>
              <option value="go">Go</option>
              <option value="php">PHP</option>
              <option value="ruby">Ruby</option>
            </select>
          )}
          <select
            className="rounded bg-zinc-800 border border-zinc-600 text-zinc-200 px-3 py-1.5 text-sm"
            value={narrationLanguage}
            onChange={(e) => setNarrationLanguage(e.target.value)}
          >
            {NARRATION_LANGUAGES.map(({ value, label }) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>
          <button
            onClick={submitTaskStream}
            disabled={loading}
            className="px-4 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-medium text-sm"
          >
            {loading ? "Generating…" : "Generate (stream + voice)"}
          </button>
          {inputTab === "task" && (
            <button
              onClick={submitTask}
              disabled={loading}
              className="px-4 py-2 rounded-lg bg-zinc-600 hover:bg-zinc-500 disabled:opacity-50 text-white font-medium text-sm"
            >
              Generate (no voice)
            </button>
          )}
        </div>
      </section>

      {loading && <GenerationProgress stage={stage} />}

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
    </main>
  );
}

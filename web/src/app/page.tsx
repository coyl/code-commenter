"use client";

import React, { useState, useRef, useEffect, useMemo } from "react";
import Link from "next/link";
import CodePlayer, { type CodePlayerRef } from "@/components/CodePlayer";
import GenerationProgress from "@/components/GenerationProgress";
import { usePCMPlayer } from "@/lib/audio";
import { clearSessionToken } from "@/lib/session-token";
import type { Segment } from "@/domain/stream";
import { useStreamTask } from "@/features/stream/useStreamTask";
import { useAuth } from "@/features/auth/useAuth";
import JobsSidebar from "@/components/JobsSidebar";
import GoogleSignInButton from "@/components/GoogleSignInButton";

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
  const [showStory, setShowStory] = useState(false);
  const [storyHtml, setStoryHtml] = useState("");
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [jobsRefreshKey, setJobsRefreshKey] = useState(0);
  const styleElRef = useRef<HTMLStyleElement | null>(null);
  const streamEndedRef = useRef(false);
  const newSegmentIndexRef = useRef<number | null>(null);
  const codePlayerRef = useRef<CodePlayerRef | null>(null);
  const { playChunk, stop: stopAudio, unlock: unlockAudio, remainingMs } = usePCMPlayer();
  const { user, loading: authLoading, authConfigured, signInUrl, signOutUrl, quotaRemaining, refetch: refetchAuth } = useAuth();

  useEffect(() => {
    if (storyHtml.trim()) setShowStory(true);
  }, [storyHtml]);

  const streamCallbacks = useMemo(
    () => ({
      onCss: setCss,
      onCode: setCode,
      onSegments: setSegments,
      onSessionId: setSessionId,
      onNarration: setNarration,
      onRawJson: () => {},
      onStoryHtml: setStoryHtml,
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

  const clearAllErrors = () => setError(null);

  useEffect(() => {
    let unlocked = false;
    const primeAudio = () => {
      if (unlocked) return;
      unlocked = true;
      Promise.resolve(unlockAudio()).catch(() => {
        unlocked = false;
      });
    };
    window.addEventListener("pointerdown", primeAudio, { passive: true });
    window.addEventListener("touchstart", primeAudio, { passive: true });
    window.addEventListener("keydown", primeAudio);
    return () => {
      window.removeEventListener("pointerdown", primeAudio);
      window.removeEventListener("touchstart", primeAudio);
      window.removeEventListener("keydown", primeAudio);
    };
  }, [unlockAudio]);

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

  // After successful generation, session event fires before backend finishes writing to job index; refetch jobs list and quota after a delay.
  useEffect(() => {
    if (!sessionId || !authConfigured) return;
    const t = setTimeout(() => {
      setJobsRefreshKey((k) => k + 1);
      refetchAuth();
    }, 3000);
    return () => clearTimeout(t);
  }, [sessionId, authConfigured, refetchAuth]);

  const quotaExhausted = authConfigured && user && quotaRemaining !== undefined && quotaRemaining <= 0;

  const submitTaskStream = () => {
    if (authConfigured && !user) return;
    if (quotaExhausted) return;
    if (inputTab === "task" && !task.trim()) return;
    if (inputTab === "code" && !userCode.trim()) return;
    clearAllErrors();
    setDisplayedCode("");
    setStoryHtml("");
    if (inputTab === "code") {
      runStream("", "", narrationLanguage, userCode.trim());
    } else {
      runStream(task.trim(), language, narrationLanguage);
    }
  };

  const displayError = error;

  const showAuthOverlay = !authLoading && authConfigured && !user && !!signInUrl;

  return (
    <div className="flex min-h-screen">
      {authConfigured && (
        <JobsSidebar
          open={sidebarOpen}
          onToggle={() => setSidebarOpen((o) => !o)}
          signedIn={!!user}
          refreshTrigger={sessionId ? `${sessionId}-${jobsRefreshKey}` : undefined}
        />
      )}
      <main className="flex-1 min-w-0 p-6 max-w-5xl mx-auto relative">
      <header className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-cyan-400">Anee Explainee</h1>
          <p className="text-zinc-400 text-sm mt-1">Describe a task → get code with just-in-time streaming and voiceover.</p>
        </div>
        <div className="flex items-center gap-3">
          {authLoading ? (
            <span className="text-zinc-500 text-sm">Checking sign-in…</span>
          ) : user ? (
            <>
              <span className="text-zinc-400 text-sm truncate max-w-[200px]" title={user.email}>
                {user.email}
              </span>
              {signOutUrl && (
                <button
                  type="button"
                  onClick={() => { clearSessionToken(); window.location.href = signOutUrl; }}
                  className="text-sm text-zinc-400 hover:text-zinc-200 underline"
                >
                  Sign out
                </button>
              )}
            </>
          ) : signInUrl ? (
            <a
              href={signInUrl}
              className="px-3 py-1.5 rounded bg-cyan-600 hover:bg-cyan-500 text-white text-sm font-medium"
            >
              Sign in with Google
            </a>
          ) : null}
        </div>
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
            disabled={loading || (authConfigured && !user) || !!quotaExhausted}
            className={`px-4 py-2 rounded-lg text-white font-medium text-sm ${
              quotaExhausted
                ? "bg-zinc-600 cursor-not-allowed"
                : "bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50"
            }`}
          >
            {loading ? "Generating…" : "Generate"}
          </button>
        </div>
        {quotaExhausted && (
          <p className="mt-2 text-amber-400 text-sm">
            Daily limit reached — you can generate up to 3 times per 24 hours.
          </p>
        )}
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
            jobId={sessionId}
            loading={loading}
            streamEndedRef={streamEndedRef}
            audio={{ playChunk, stop: stopAudio, unlock: unlockAudio, remainingMs }}
          />
          {sessionId && storyHtml && (
            <div className="mt-4 flex items-center gap-3">
              <Link
                href={`/story/${sessionId}`}
                className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg bg-zinc-800 border border-zinc-600 text-zinc-200 hover:text-white hover:border-zinc-400 text-sm font-medium transition-colors"
              >
                View story
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                  <polyline points="15 3 21 3 21 9" />
                  <line x1="10" y1="14" x2="21" y2="3" />
                </svg>
              </Link>
            </div>
          )}
          {(code || segments.length > 0) && (
            <div className="mt-4 border border-zinc-700 rounded-lg overflow-hidden bg-zinc-900/50">
              <button
                type="button"
                onClick={() => setShowStory((v) => !v)}
                className="w-full flex items-center justify-between px-3 py-2 text-left text-sm font-medium text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50 transition-colors"
              >
                <span>Story</span>
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                  className={`transition-transform ${showStory ? "rotate-180" : ""}`}
                >
                  <path d="M7 10l5 5 5-5z" />
                </svg>
              </button>
              {showStory && (
                <pre className="p-3 text-xs font-mono whitespace-pre-wrap break-all text-zinc-500 overflow-auto max-h-64 border-t border-zinc-700">
                  {storyHtml || "No story yet. It will appear here after generation completes."}
                </pre>
              )}
            </div>
          )}
        </>
      )}

      {showAuthOverlay && signInUrl && (
        <div
          className="absolute inset-0 z-10 flex items-center justify-center backdrop-blur-md bg-black/50 rounded-lg"
          aria-modal
          aria-label="Sign in required"
        >
          <div className="bg-zinc-900/95 border border-zinc-700 rounded-xl p-8 max-w-sm w-full mx-4 shadow-xl text-center">
            <p className="text-zinc-300 text-sm leading-relaxed mb-6">
              Sign in with Google to generate jobs. Generation is only available when you are signed in.
            </p>
            <GoogleSignInButton href={signInUrl} />
          </div>
        </div>
      )}
      </main>
    </div>
  );
}

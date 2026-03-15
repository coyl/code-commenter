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
import JobCarousel from "@/components/JobCarousel";

type InputTab = "task" | "code";

const NARRATION_LANGUAGES = [
  { value: "english", label: "English" },
  { value: "german", label: "German" },
  { value: "spanish", label: "Spanish" },
  { value: "italian", label: "Italian" },
  { value: "chinese", label: "Chinese (Simplified)" },
] as const;

const FEATURE_CHIPS = [
  "Typing animations",
  "AI voiceover",
  "Shareable player",
  "Multi-language",
];

/** Max characters for the "Your code" paste input. */
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
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [jobsRefreshKey, setJobsRefreshKey] = useState(0);
  const styleElRef = useRef<HTMLStyleElement | null>(null);
  const streamEndedRef = useRef(false);
  const newSegmentIndexRef = useRef<number | null>(null);
  const codePlayerRef = useRef<CodePlayerRef | null>(null);
  const { playChunk, stop: stopAudio, unlock: unlockAudio, remainingMs } = usePCMPlayer();
  const { user, loading: authLoading, authConfigured, signInUrl, signOutUrl, quotaRemaining, refetch: refetchAuth } = useAuth();

  useEffect(() => {
    if (typeof window !== "undefined" && window.innerWidth >= 768) {
      setSidebarOpen(true);
    }
  }, []);

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
      onStreamEnded: (ended: boolean) => { streamEndedRef.current = ended; },
      onNewSegmentIndex: (idx: number | null) => { newSegmentIndexRef.current = idx; },
      stopAudio,
      unlockAudio,
    }),
    [stopAudio, unlockAudio]
  );
  const { runStream } = useStreamTask(streamCallbacks);

  useEffect(() => {
    let unlocked = false;
    const primeAudio = () => {
      if (unlocked) return;
      unlocked = true;
      Promise.resolve(unlockAudio()).catch(() => { unlocked = false; });
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
    setError(null);
    setDisplayedCode("");
    setStoryHtml("");
    if (inputTab === "code") {
      runStream("", "", narrationLanguage, userCode.trim());
    } else {
      runStream(task.trim(), language, narrationLanguage);
    }
  };

  const showAuthOverlay = !authLoading && authConfigured && !user && !!signInUrl;

  return (
    <div className="flex flex-col min-h-screen">
      {/* ── Ambient background glows ─────────────────────────────── */}
      <div className="pointer-events-none fixed inset-0" aria-hidden>
        <div className="absolute -top-[20%] -right-[10%] w-[640px] h-[640px] rounded-full bg-cyan-500/[0.055] blur-[130px]" />
        <div className="absolute -bottom-[20%] -left-[10%] w-[520px] h-[520px] rounded-full bg-indigo-500/[0.04] blur-[110px]" />
      </div>

      {/* ── Sticky top header ─────────────────────────────────────── */}
      <header className="sticky top-0 z-40 flex-shrink-0 border-b border-zinc-800/60 bg-zinc-950/80 backdrop-blur-md">
        {/* Subtle gradient accent line at the very top */}
        <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-cyan-500/30 to-transparent" aria-hidden />
        <div className="flex h-14 items-center justify-between gap-4 px-4 md:px-6">
          {/* Left: mobile sidebar hamburger + logo */}
          <div className="flex items-center gap-3">
            {authConfigured && !authLoading && user && (
              <button
                type="button"
                onClick={() => setSidebarOpen((o) => !o)}
                className="md:hidden flex items-center justify-center w-9 h-9 rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
                aria-label={sidebarOpen ? "Close my jobs" : "Open my jobs"}
              >
                {sidebarOpen ? (
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M18 6 6 18" /><path d="m6 6 12 12" />
                  </svg>
                ) : (
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <line x1="3" y1="6" x2="21" y2="6" /><line x1="3" y1="12" x2="21" y2="12" /><line x1="3" y1="18" x2="21" y2="18" />
                  </svg>
                )}
              </button>
            )}
            <a href="/" className="flex items-center gap-2 select-none group">
              {/* Code brackets icon */}
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" className="text-cyan-400 group-hover:text-cyan-300 transition-colors" aria-hidden>
                <polyline points="16 18 22 12 16 6" />
                <polyline points="8 6 2 12 8 18" />
              </svg>
              <span className="text-base font-bold tracking-tight bg-gradient-to-r from-cyan-400 to-sky-300 bg-clip-text text-transparent">
                Anee Explainee
              </span>
            </a>
          </div>

          {/* Right: auth controls */}
          <div className="flex items-center gap-2">
            {authLoading ? (
              <span className="text-zinc-700 text-sm">…</span>
            ) : user ? (
              <>
                <span className="hidden sm:block text-zinc-500 text-sm truncate max-w-[180px]" title={user.email}>
                  {user.email}
                </span>
                {quotaRemaining !== undefined && (
                  <span className="hidden sm:block text-xs text-zinc-600 tabular-nums">
                    {quotaRemaining} left
                  </span>
                )}
                {signOutUrl && (
                  <button
                    type="button"
                    onClick={() => { clearSessionToken(); window.location.href = signOutUrl; }}
                    className="px-2.5 py-1 rounded-md text-zinc-500 hover:text-zinc-200 text-sm hover:bg-zinc-800 transition-colors"
                  >
                    Sign out
                  </button>
                )}
              </>
            ) : signInUrl ? (
              <a
                href={signInUrl}
                className="px-3 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 active:bg-cyan-700 text-white text-sm font-medium transition-colors"
              >
                Sign in
              </a>
            ) : null}
          </div>
        </div>
      </header>

      {/* ── Body: sidebar + main ───────────────────────────────────── */}
      <div className="flex flex-col flex-1 md:flex-row overflow-hidden">
        {authConfigured && (
          <JobsSidebar
            open={sidebarOpen}
            onToggle={() => setSidebarOpen((o) => !o)}
            signedIn={!!user}
            refreshTrigger={sessionId ? `${sessionId}-${jobsRefreshKey}` : undefined}
          />
        )}

        <main className="flex-1 min-w-0 overflow-y-auto relative">
          <div className="max-w-4xl mx-auto px-4 py-6 md:px-6 md:py-10">

            {/* ── Hero ───────────────────────────────────────────── */}
            {!css && !code && !loading && (
              <div className="mb-9">
                {/* Eyebrow badge */}
                <div className="anim-in inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-cyan-500/10 border border-cyan-500/20 text-cyan-400 text-xs font-semibold tracking-wide mb-5">
                  <span className="w-1.5 h-1.5 rounded-full bg-cyan-400 animate-pulse" aria-hidden />
                  AI-powered code walkthroughs
                </div>

                {/* Headline */}
                <h1 className="anim-in-d1 text-3xl sm:text-4xl font-bold mb-4 leading-[1.18] tracking-tight">
                  <span className="text-zinc-100">Turn any coding task into</span>
                  <br className="hidden sm:block" />
                  {" "}
                  <span className="bg-gradient-to-r from-cyan-400 via-sky-300 to-blue-400 bg-clip-text text-transparent">
                    a live walkthrough
                  </span>
                </h1>

                {/* Sub-headline */}
                <p className="anim-in-d2 text-zinc-400 text-base leading-relaxed mb-7 max-w-lg">
                  Describe a task and get an interactive player with step-by-step typing animations and AI voiceover narration.
                </p>

                {/* Feature chips */}
                <div className="anim-in-d2 flex flex-wrap gap-2 mb-8">
                  {FEATURE_CHIPS.map((label) => (
                    <span
                      key={label}
                      className="inline-flex items-center px-3 py-1 rounded-full bg-zinc-800/60 border border-zinc-700/50 text-zinc-400 text-xs"
                    >
                      {label}
                    </span>
                  ))}
                </div>

                {/* Recent walkthroughs carousel */}
                <div className="anim-in-d3">
                  <p className="text-xs font-semibold uppercase tracking-widest text-zinc-600 mb-2.5">
                    Recent walkthroughs
                  </p>
                  <JobCarousel />
                </div>
              </div>
            )}

            {/* ── Input form ──────────────────────────────────────── */}
            <section className="anim-in-d4 mb-6 relative rounded-xl border border-zinc-800/80 bg-zinc-900/60 shadow-[0_0_0_1px_rgba(6,182,212,0.06),0_8px_32px_rgba(0,0,0,0.35)] backdrop-blur-sm">
              {/* Gradient top-edge glow */}
              <div className="absolute inset-x-0 top-0 h-px rounded-t-xl bg-gradient-to-r from-transparent via-cyan-500/30 to-transparent" aria-hidden />

              {/* Tab row */}
              <div className="flex gap-1 p-3 border-b border-zinc-800/60">
                <button
                  type="button"
                  onClick={() => setInputTab("task")}
                  className={`px-3.5 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                    inputTab === "task"
                      ? "bg-zinc-800 text-zinc-100 shadow-sm"
                      : "text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50"
                  }`}
                >
                  Describe a task
                </button>
                <button
                  type="button"
                  onClick={() => setInputTab("code")}
                  className={`px-3.5 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                    inputTab === "code"
                      ? "bg-zinc-800 text-zinc-100 shadow-sm"
                      : "text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50"
                  }`}
                >
                  Your code
                </button>
              </div>

              <div className="p-4">
                <label className="block text-xs font-semibold uppercase tracking-wider text-zinc-500 mb-2">
                  {inputTab === "task" ? "Task description" : "Paste your code"}
                </label>

                {inputTab === "task" ? (
                  <textarea
                    className="w-full h-24 px-3 py-2.5 rounded-lg bg-zinc-800/70 border border-zinc-700/50 text-zinc-100 placeholder-zinc-600 text-sm focus:outline-none focus:ring-2 focus:ring-cyan-500/50 focus:border-cyan-500/30 resize-none transition-colors"
                    placeholder="e.g. A React counter component with increment and decrement buttons"
                    value={task}
                    onChange={(e) => setTask(e.target.value)}
                  />
                ) : (
                  <div>
                    <textarea
                      className="w-full h-44 px-3 py-2.5 rounded-lg bg-zinc-800/70 border border-zinc-700/50 text-zinc-100 placeholder-zinc-600 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-cyan-500/50 focus:border-cyan-500/30 resize-none transition-colors"
                      placeholder="Paste your code here. It will be formatted and split into segments for interactive narration."
                      value={userCode}
                      maxLength={MAX_USER_CODE_LENGTH}
                      onChange={(e) => setUserCode(e.target.value.slice(0, MAX_USER_CODE_LENGTH))}
                    />
                    <p
                      className={`mt-1.5 text-right text-xs tabular-nums ${
                        userCode.length >= MAX_USER_CODE_LENGTH ? "text-amber-400" : "text-zinc-600"
                      }`}
                      aria-live="polite"
                    >
                      {userCode.length.toLocaleString()} / {MAX_USER_CODE_LENGTH.toLocaleString()}
                    </p>
                  </div>
                )}

                {/* Controls — stacks on mobile, inline on sm+ */}
                <div className="flex flex-col gap-2 mt-4 sm:flex-row sm:flex-wrap sm:items-center sm:gap-3">
                  {inputTab === "task" && (
                    <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-2">
                      <label className="text-xs text-zinc-500 sm:hidden">Language</label>
                      <select
                        className="w-full sm:w-auto rounded-lg bg-zinc-800/70 border border-zinc-700/50 text-zinc-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-cyan-500/50 transition-colors"
                        value={language}
                        onChange={(e) => setLanguage(e.target.value)}
                        aria-label="Programming language"
                      >
                        <option value="javascript">JavaScript</option>
                        <option value="typescript">TypeScript</option>
                        <option value="python">Python</option>
                        <option value="go">Go</option>
                        <option value="php">PHP</option>
                        <option value="ruby">Ruby</option>
                      </select>
                    </div>
                  )}

                  <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-2">
                    <label className="text-xs text-zinc-500 sm:hidden">Narration language</label>
                    <select
                      className="w-full sm:w-auto rounded-lg bg-zinc-800/70 border border-zinc-700/50 text-zinc-200 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-cyan-500/50 transition-colors"
                      value={narrationLanguage}
                      onChange={(e) => setNarrationLanguage(e.target.value)}
                      aria-label="Narration language"
                    >
                      {NARRATION_LANGUAGES.map(({ value, label }) => (
                        <option key={value} value={value}>{label}</option>
                      ))}
                    </select>
                  </div>

                  <button
                    onClick={submitTaskStream}
                    disabled={loading || (authConfigured && !user) || !!quotaExhausted}
                    className={`w-full sm:w-auto mt-1 sm:mt-0 sm:ml-auto px-6 py-2 rounded-lg text-white text-sm font-semibold ${
                      quotaExhausted
                        ? "bg-zinc-700 text-zinc-500 cursor-not-allowed"
                        : "btn-shimmer"
                    }`}
                  >
                    {loading ? "Generating…" : "Generate"}
                  </button>
                </div>

                {quotaExhausted && (
                  <p className="mt-3 text-amber-400/90 text-xs leading-relaxed">
                    Daily limit reached — you can generate up to 3 times per 24 hours.
                  </p>
                )}
              </div>
            </section>

            {/* ── Progress ────────────────────────────────────────── */}
            {loading && <GenerationProgress stage={stage} />}

            {/* ── Error ───────────────────────────────────────────── */}
            {error && (
              <div className="mb-5 px-4 py-3 rounded-xl bg-red-950/40 border border-red-800/50 text-red-300 text-sm leading-relaxed">
                {error}
              </div>
            )}

            {/* ── Output ──────────────────────────────────────────── */}
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
                  <div className="mt-4">
                    <Link
                      href={`/story/${sessionId}`}
                      className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg bg-zinc-800/70 border border-zinc-700/60 text-zinc-200 hover:text-white hover:border-zinc-500 text-sm font-medium transition-colors"
                    >
                      View story
                      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                        <polyline points="15 3 21 3 21 9" />
                        <line x1="10" y1="14" x2="21" y2="3" />
                      </svg>
                    </Link>
                  </div>
                )}

                {(code || segments.length > 0) && (
                  <div className="mt-3 border border-zinc-800/70 rounded-xl overflow-hidden bg-zinc-900/50">
                    <button
                      type="button"
                      onClick={() => setShowStory((v) => !v)}
                      className="w-full flex items-center justify-between px-4 py-2.5 text-left text-xs font-semibold uppercase tracking-wider text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/40 transition-colors"
                    >
                      <span>Story HTML</span>
                      <svg
                        width="15" height="15" viewBox="0 0 24 24" fill="currentColor"
                        className={`transition-transform duration-200 ${showStory ? "rotate-180" : ""}`}
                      >
                        <path d="M7 10l5 5 5-5z" />
                      </svg>
                    </button>
                    {showStory && (
                      <pre className="p-4 text-xs font-mono whitespace-pre-wrap break-all text-zinc-500 overflow-auto max-h-56 border-t border-zinc-800/70">
                        {storyHtml || "No story yet. It will appear here after generation completes."}
                      </pre>
                    )}
                  </div>
                )}
              </>
            )}
          </div>

          {/* ── Auth overlay ───────────────────────────────────────── */}
          {showAuthOverlay && signInUrl && (
            <div
              className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4 sm:p-6 backdrop-blur-md bg-black/60"
              aria-modal
              aria-label="Sign in required"
            >
              <div className="relative bg-zinc-900/95 border border-zinc-700/80 rounded-2xl p-7 max-w-sm w-full shadow-2xl shadow-black/70 text-center overflow-hidden">
                {/* Top glow */}
                <div className="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-cyan-500/40 to-transparent" aria-hidden />
                <div className="mb-4">
                  <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-zinc-800 mb-4 shadow-inner">
                    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className="text-cyan-400">
                      <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" />
                    </svg>
                  </div>
                  <h2 className="text-base font-semibold text-zinc-100 mb-2">Sign in to generate</h2>
                  <p className="text-zinc-400 text-sm leading-relaxed">
                    Generation is only available when you are signed in.
                  </p>
                </div>
                <GoogleSignInButton href={signInUrl} className="w-full" />
              </div>
            </div>
          )}
        </main>
      </div>
    </div>
  );
}

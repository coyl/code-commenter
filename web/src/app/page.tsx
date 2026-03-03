"use client";

import { useState, useCallback, useRef, useEffect } from "react";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const LIVE_SAMPLE_RATE = 24000;

// PCM 24kHz 16-bit LE: duration in seconds from base64 chunks
function audioDurationSeconds(chunks: string[]): number {
  let totalSamples = 0;
  for (const b64 of chunks) {
    if (!b64) continue;
    const binary = atob(b64);
    totalSamples += Math.floor(binary.length / 2);
  }
  return totalSamples / LIVE_SAMPLE_RATE;
}

// Typing speed so text finishes by 80% of audio length. Returns ms per character.
function typingSpeedFor80Percent(codeLength: number, audioChunks: string[]): number {
  if (codeLength <= 0) return 20;
  const durationSec = audioDurationSeconds(audioChunks);
  if (durationSec <= 0) return 20;
  const targetMs = 0.8 * durationSec * 1000;
  return Math.max(5, Math.min(80, Math.round(targetMs / codeLength)));
}

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

type Segment = {
  index: number;
  code: string;
  narration: string;
  audioChunks: string[];
};

// Play PCM 24kHz 16-bit LE mono chunks via Web Audio API.
// AudioContext must be created/resumed on user gesture (e.g. button click) or playback will be blocked.
// stop() stops all scheduled/playing sources so new playback doesn't stack.
function usePCMPlayer() {
  const ctxRef = useRef<AudioContext | null>(null);
  const nextStartRef = useRef(0);
  const activeSourcesRef = useRef<Set<AudioBufferSourceNode>>(new Set());

  const playChunk = useCallback((base64PCM: string) => {
    try {
      const binary = atob(base64PCM);
      const byteLen = binary.length;
      const bytes = new Uint8Array(byteLen);
      for (let i = 0; i < byteLen; i++) bytes[i] = binary.charCodeAt(i);
      const numSamples = Math.floor(byteLen / 2);
      const int16 = new Int16Array(bytes.buffer, 0, numSamples);
      const float32 = new Float32Array(numSamples);
      for (let i = 0; i < numSamples; i++) float32[i] = int16[i] / 32768;

      let ctx = ctxRef.current;
      if (!ctx) {
        const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
        ctx = new Ctx();
        ctxRef.current = ctx;
        nextStartRef.current = ctx.currentTime;
      }
      if (ctx.state === "suspended") ctx.resume();
      const buffer = ctx.createBuffer(1, float32.length, LIVE_SAMPLE_RATE);
      buffer.copyToChannel(float32, 0);
      const source = ctx.createBufferSource();
      source.buffer = buffer;
      source.connect(ctx.destination);
      const start = Math.max(ctx.currentTime, nextStartRef.current);
      source.start(start);
      nextStartRef.current = start + buffer.duration;
      activeSourcesRef.current.add(source);
      source.onended = () => activeSourcesRef.current.delete(source);
    } catch (e) {
      console.warn("PCM play failed:", e);
    }
  }, []);

  const stop = useCallback(() => {
    nextStartRef.current = 0;
    activeSourcesRef.current.forEach((s) => {
      try {
        s.stop();
      } catch {
        // already stopped
      }
    });
    activeSourcesRef.current.clear();
  }, []);

  // Unlock AudioContext on user gesture (required by browsers). Call this when user clicks Generate.
  const unlock = useCallback(() => {
    if (ctxRef.current) {
      if (ctxRef.current.state === "suspended") ctxRef.current.resume();
      return;
    }
    const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    const ctx = new Ctx();
    ctxRef.current = ctx;
    nextStartRef.current = ctx.currentTime;
    if (ctx.state === "suspended") ctx.resume();
  }, []);

  return { playChunk, stop, unlock };
}

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
  const codeContainerRef = useRef<HTMLPreElement>(null);
  const styleElRef = useRef<HTMLStyleElement | null>(null);
  const streamCodeBufferRef = useRef("");
  const displayedLenRef = useRef(0);
  const typingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const pendingSegmentRef = useRef<{ code: string; narration: string } | null>(null);
  const pendingAudioChunksRef = useRef<string[]>([]);
  const playNextTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isPlayingRef = useRef(false);
  const [currentNarration, setCurrentNarration] = useState("");
  const [segments, setSegments] = useState<Segment[]>([]);
  const [currentSegmentIndex, setCurrentSegmentIndex] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const { playChunk: playAudioChunk, stop: stopAudio, unlock: unlockAudio } = usePCMPlayer();

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

  useEffect(() => {
    isPlayingRef.current = isPlaying;
  }, [isPlaying]);

  useEffect(() => {
    return () => {
      stopAudio();
      if (typingTimerRef.current) clearInterval(typingTimerRef.current);
    };
  }, [stopAudio]);

  // Scroll code area to follow the printed content
  useEffect(() => {
    const el = codeContainerRef.current;
    if (el) el.scrollTop = el.scrollHeight - el.clientHeight;
  }, [displayedCode]);

  const typeSegment = useCallback(
    (
      segmentCode: string,
      speedMs = 15,
      options?: { updateCode?: boolean; onComplete?: () => void }
    ) => {
      const updateCode = options?.updateCode !== false;
      const onComplete = options?.onComplete;
      streamCodeBufferRef.current += segmentCode;
      const targetLen = streamCodeBufferRef.current.length;
      const startLen = displayedLenRef.current;
      if (typingTimerRef.current) clearInterval(typingTimerRef.current);
      let len = startLen;
      typingTimerRef.current = setInterval(() => {
        len += 1;
        displayedLenRef.current = len;
        setDisplayedCode(streamCodeBufferRef.current.slice(0, len));
        if (updateCode) setCode(streamCodeBufferRef.current);
        if (len >= targetLen && typingTimerRef.current) {
          clearInterval(typingTimerRef.current);
          typingTimerRef.current = null;
          onComplete?.();
        }
      }, speedMs);
    },
    []
  );

  // Play one segment with typing paced to 80% of audio length. Used for initial stream (buffer-then-play).
  const playSegmentNow = useCallback(
    (
      seg: { code: string; narration: string; audioChunks: string[] },
      codeSoFar: string,
      updateCode: boolean,
      segmentIndex: number
    ) => {
      setCurrentSegmentIndex(segmentIndex);
      streamCodeBufferRef.current = codeSoFar;
      displayedLenRef.current = codeSoFar.length;
      setDisplayedCode(codeSoFar);
      setCurrentNarration(seg.narration);
      const speedMs = typingSpeedFor80Percent(seg.code.length, seg.audioChunks);
      typeSegment(seg.code, speedMs, { updateCode });
      seg.audioChunks.forEach(playAudioChunk);
    },
    [typeSegment, playAudioChunk]
  );

  const replaySegment = useCallback(
    (i: number) => {
      if (i < 0 || i >= segments.length) return;
      if (playNextTimeoutRef.current) {
        clearTimeout(playNextTimeoutRef.current);
        playNextTimeoutRef.current = null;
      }
      stopAudio();
      if (typingTimerRef.current) {
        clearInterval(typingTimerRef.current);
        typingTimerRef.current = null;
      }
      setIsPlaying(true);
      const codeSoFar = segments
        .slice(0, i)
        .map((s) => s.code)
        .join("");
      streamCodeBufferRef.current = codeSoFar;
      displayedLenRef.current = codeSoFar.length;
      setDisplayedCode(codeSoFar);
      setCurrentSegmentIndex(i);
      setCurrentNarration(segments[i].narration);
      const speedMs = typingSpeedFor80Percent(segments[i].code.length, segments[i].audioChunks);
      const onComplete = () => {
        const remainingMs = Math.max(200, 0.2 * audioDurationSeconds(segments[i].audioChunks) * 1000);
        playNextTimeoutRef.current = setTimeout(() => {
          playNextTimeoutRef.current = null;
          if (!isPlayingRef.current) return;
          if (i < segments.length - 1) replaySegment(i + 1);
          else setIsPlaying(false);
        }, remainingMs);
      };
      typeSegment(segments[i].code, speedMs, { updateCode: false, onComplete });
      segments[i].audioChunks.forEach(playAudioChunk);
    },
    [segments, typeSegment, stopAudio, playAudioChunk]
  );

  const goPrevBlock = useCallback(() => {
    const prev = Math.max(0, currentSegmentIndex - 1);
    setCurrentSegmentIndex(prev);
    replaySegment(prev);
  }, [currentSegmentIndex, replaySegment]);

  const goNextBlock = useCallback(() => {
    const next = Math.min(segments.length - 1, currentSegmentIndex + 1);
    setCurrentSegmentIndex(next);
    replaySegment(next);
  }, [currentSegmentIndex, segments.length, replaySegment]);

  // Single place to stop any current playback (typing + audio). Keeps playback from stacking.
  const stopCurrentPlayback = useCallback(() => {
    if (playNextTimeoutRef.current) {
      clearTimeout(playNextTimeoutRef.current);
      playNextTimeoutRef.current = null;
    }
    stopAudio();
    if (typingTimerRef.current) {
      clearInterval(typingTimerRef.current);
      typingTimerRef.current = null;
    }
    setIsPlaying(false);
  }, [stopAudio]);

  const togglePlayPause = useCallback(() => {
    if (isPlaying) {
      stopCurrentPlayback();
    } else {
      stopCurrentPlayback(); // ensure nothing is playing before we start
      setIsPlaying(true);
      replaySegment(currentSegmentIndex);
    }
  }, [isPlaying, currentSegmentIndex, replaySegment, stopCurrentPlayback]);

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
    setCurrentSegmentIndex(0);
    setIsPlaying(false);
    if (playNextTimeoutRef.current) clearTimeout(playNextTimeoutRef.current);
    playNextTimeoutRef.current = null;
    pendingSegmentRef.current = null;
    pendingAudioChunksRef.current = [];
    streamCodeBufferRef.current = "";
    displayedLenRef.current = 0;
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
          case "spec":
            setNarration(msg.narration || "");
            break;
          case "css":
            setCss(msg.css || "");
            break;
          case "segment": {
            const segCode = msg.code ?? "";
            const segNarration = msg.narration ?? "";
            const pending = pendingSegmentRef.current;
            const pendingChunks = pendingAudioChunksRef.current;
            if (pending) {
              const completedSeg = {
                index: 0,
                code: pending.code,
                narration: pending.narration,
                audioChunks: [...pendingChunks],
              };
              setSegments((prev) => {
                const newSeg = { ...completedSeg, index: prev.length };
                const codeSoFar = prev.map((s) => s.code).join("");
                setTimeout(() => playSegmentNow(newSeg, codeSoFar, true, prev.length), 0);
                return [...prev, newSeg];
              });
            }
            pendingSegmentRef.current = { code: segCode, narration: segNarration };
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
            const full = (msg.code || streamCodeBufferRef.current || "").trim();
            setCode(full);
            const pending = pendingSegmentRef.current;
            const pendingChunks = pendingAudioChunksRef.current;
            if (pending) {
              const lastSeg = {
                index: 0,
                code: pending.code,
                narration: pending.narration,
                audioChunks: [...pendingChunks],
              };
              setSegments((prev) => {
                const newSeg = { ...lastSeg, index: prev.length };
                const codeSoFar = prev.map((s) => s.code).join("");
                setTimeout(() => playSegmentNow(newSeg, codeSoFar, true, prev.length), 0);
                return [...prev, newSeg];
              });
            } else {
              setDisplayedCode(full);
              streamCodeBufferRef.current = full;
              displayedLenRef.current = full.length;
            }
            pendingSegmentRef.current = null;
            pendingAudioChunksRef.current = [];
            if (typingTimerRef.current) {
              clearInterval(typingTimerRef.current);
              typingTimerRef.current = null;
            }
            setCurrentNarration("");
            setCurrentSegmentIndex(0);
            break;
          }
          case "session":
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
  }, [task, language, playAudioChunk, stopAudio, unlockAudio, typeSegment, playSegmentNow]);

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
        <section className="mb-6">
          <h2 className="text-sm font-medium text-zinc-400 mb-2">Code view</h2>
          <div
            id="code-view"
            className="rounded-lg overflow-hidden border border-zinc-700 bg-zinc-900 h-[400px] flex flex-col"
          >
            <pre
              ref={codeContainerRef}
              className="p-4 text-sm overflow-auto font-mono whitespace-pre text-zinc-100 flex-1 min-h-0 scrollbar-hide"
            >
              {segments.length > 0 ? (
                (() => {
                  const cumLengths: number[] = [0];
                  for (const s of segments) cumLengths.push(cumLengths[cumLengths.length - 1] + s.code.length);
                  return (
                    <>
                      {segments.map((_, i) => {
                        const start = cumLengths[i];
                        const end = Math.min(cumLengths[i + 1], displayedCode.length);
                        if (start >= displayedCode.length) return null;
                        const text = displayedCode.slice(start, end);
                        const isCurrent = i === currentSegmentIndex;
                        return (
                          <span
                            key={i}
                            className={isCurrent ? "bg-zinc-800/70 rounded-sm" : undefined}
                          >
                            {text}
                          </span>
                        );
                      })}
                      {loading && displayedCode.length > 0 && <span className="animate-pulse">|</span>}
                    </>
                  );
                })()
              ) : (
                <>
                  {displayedCode}
                  {loading && displayedCode.length > 0 && <span className="animate-pulse">|</span>}
                </>
              )}
            </pre>
          </div>
          {(narration || currentNarration) && (
            <p className="mt-2 text-zinc-500 text-sm italic">
              {currentNarration ? `Narrating: ${currentNarration}` : `Narration: ${narration}`}
            </p>
          )}
          {segments.length > 0 && (
            <>
              <div className="mt-3 flex items-center gap-2 flex-wrap">
                <span className="text-xs text-zinc-500 mr-1">Playback:</span>
                <button
                  type="button"
                  onClick={togglePlayPause}
                  className="px-3 py-1.5 rounded bg-zinc-700 hover:bg-zinc-600 text-zinc-200 text-sm font-medium"
                  title={isPlaying ? "Pause" : "Play current block"}
                >
                  {isPlaying ? "Pause" : "Play"}
                </button>
                <button
                  type="button"
                  onClick={goPrevBlock}
                  disabled={currentSegmentIndex <= 0}
                  className="px-3 py-1.5 rounded bg-zinc-700 hover:bg-zinc-600 disabled:opacity-50 disabled:pointer-events-none text-zinc-200 text-sm font-medium"
                  title="Previous block"
                >
                  ← Prev
                </button>
                <button
                  type="button"
                  onClick={goNextBlock}
                  disabled={currentSegmentIndex >= segments.length - 1}
                  className="px-3 py-1.5 rounded bg-zinc-700 hover:bg-zinc-600 disabled:opacity-50 disabled:pointer-events-none text-zinc-200 text-sm font-medium"
                  title="Next block"
                >
                  Next →
                </button>
                <span className="text-xs text-zinc-500 ml-1">
                  Block {currentSegmentIndex + 1} / {segments.length}
                </span>
              </div>
              <div className="mt-2 flex flex-wrap gap-1" role="tablist" aria-label="Timeline segments">
                {segments.map((seg, i) => (
                  <button
                    key={seg.index}
                    type="button"
                    onClick={() => replaySegment(i)}
                    className={`px-2 py-1 rounded text-xs font-medium transition-colors ${
                      i === currentSegmentIndex
                        ? "bg-cyan-600 text-white"
                        : "bg-zinc-700 text-zinc-300 hover:bg-zinc-600"
                    }`}
                    title={seg.narration || `Block ${i + 1}`}
                  >
                    {i + 1}
                  </button>
                ))}
              </div>
            </>
          )}
        </section>
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

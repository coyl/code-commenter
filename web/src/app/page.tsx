"use client";

import React, { useState, useCallback, useRef, useEffect } from "react";
import { findTokenEnd, stripOrphanClosers } from "@/lib/syntaxHighlight";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// Short tag (1–2 chars) -> CSS class. Backend uses k,s,c,n,f,o,p,v to save tokens.
const TOKEN_TAG_MAP: Record<string, string> = {
  k: "keyword",
  s: "string",
  c: "comment",
  n: "number",
  f: "function",
  o: "operator",
  p: "punctuation",
  v: "variable",
  keyword: "keyword",
  string: "string",
  comment: "comment",
  number: "number",
  function: "function",
  operator: "operator",
  punctuation: "punctuation",
  variable: "variable",
};

function tokenClass(type: string): string {
  const t = type.trim().toLowerCase();
  return TOKEN_TAG_MAP[t] ? `token-${TOKEN_TAG_MAP[t]}` : "";
}

// Splits segment code into chunks: each chunk is either a run of plain text or one full [[x]]...[[/x]] token.
function getChunks(s: string): string[] {
  const chunks: string[] = [];
  let i = 0;
  while (i < s.length) {
    const open = s.indexOf("[[", i);
    if (open === -1) {
      if (i < s.length) chunks.push(s.slice(i));
      break;
    }
    if (open > i) chunks.push(s.slice(i, open));
    const typeEnd = s.indexOf("]]", open + 2);
    if (typeEnd === -1) {
      chunks.push(s.slice(open));
      break;
    }
    const type = s.slice(open + 2, typeEnd);
    const contentStart = typeEnd + 2;
    const { end: contentEnd, skipLen } = findTokenEnd(s, contentStart, type);
    const fullToken =
      contentEnd === -1 ? s.slice(open) : s.slice(open, contentEnd + skipLen);
    chunks.push(fullToken);
    i = contentEnd === -1 ? s.length : contentEnd + skipLen;
  }
  return chunks;
}

// Parses code that may contain [[x]]content[[/x]] (short or long type) and returns React nodes for syntax highlighting.
// Strips orphan malformed closers like )[/p]] so they are not shown as literal text.
function parseSyntaxHighlight(code: string): React.ReactNode {
  const out: React.ReactNode[] = [];
  let i = 0;
  while (i < code.length) {
    const open = code.indexOf("[[", i);
    if (open === -1) {
      if (i < code.length) out.push(stripOrphanClosers(code.slice(i)));
      break;
    }
    if (open > i) out.push(stripOrphanClosers(code.slice(i, open)));
    const typeEnd = code.indexOf("]]", open + 2);
    if (typeEnd === -1) {
      out.push(code.slice(open));
      break;
    }
    const type = code.slice(open + 2, typeEnd).trim().toLowerCase();
    const contentStart = typeEnd + 2;
    const { end: contentEnd, skipLen } = findTokenEnd(code, contentStart, type);
    const content = contentEnd === -1 ? code.slice(contentStart) : code.slice(contentStart, contentEnd);
    const className = tokenClass(type);
    if (className) {
      out.push(React.createElement("span", { key: open, className }, content));
    } else {
      out.push(content);
    }
    i = contentEnd === -1 ? code.length : contentEnd + skipLen;
  }
  return React.createElement(React.Fragment, null, ...out);
}
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
  const [showRawDebug, setShowRawDebug] = useState(false);
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
      if (typingTimerRef.current) clearInterval(typingTimerRef.current);

      const prefixLen = streamCodeBufferRef.current.length;
      streamCodeBufferRef.current += segmentCode;
      const hasTags = segmentCode.includes("[[");
      const chunks = getChunks(segmentCode);

      if (hasTags && chunks.length > 0) {
        // Reveal chunk-by-chunk so each [[type]]...[[/type]] appears as a whole, already highlighted
        const msPerChunk = Math.max(20, Math.round((segmentCode.length * speedMs) / chunks.length));
        let chunkIndex = 0;
        typingTimerRef.current = setInterval(() => {
          chunkIndex += 1;
          const displayed = streamCodeBufferRef.current.slice(0, prefixLen) + chunks.slice(0, chunkIndex).join("");
          displayedLenRef.current = displayed.length;
          setDisplayedCode(displayed);
          if (chunkIndex >= chunks.length) {
            if (typingTimerRef.current) {
              clearInterval(typingTimerRef.current);
              typingTimerRef.current = null;
            }
            if (updateCode) setCode(streamCodeBufferRef.current);
            onComplete?.();
          }
        }, msPerChunk);
      } else {
        // No tags: character-by-character as before
        const targetLen = streamCodeBufferRef.current.length;
        let len = displayedLenRef.current;
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
      }
    },
    []
  );

  // Play one segment with typing paced to 80% of audio length. Narration-only segments (empty code) just play audio.
  const playSegmentNow = useCallback(
    (
      seg: { code: string; narration: string; audioChunks: string[] },
      codeSoFar: string,
      updateCode: boolean,
      segmentIndex: number
    ) => {
      setCurrentSegmentIndex(segmentIndex);
      setCurrentNarration(seg.narration);
      if (seg.code.length > 0) {
        streamCodeBufferRef.current = codeSoFar;
        displayedLenRef.current = codeSoFar.length;
        setDisplayedCode(codeSoFar);
        const speedMs = typingSpeedFor80Percent(seg.code.length, seg.audioChunks);
        typeSegment(seg.code, speedMs, { updateCode });
      }
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
      const seg = segments[i];
      const codeSoFar = segments
        .slice(0, i)
        .map((s) => s.code)
        .join("");
      streamCodeBufferRef.current = codeSoFar;
      displayedLenRef.current = codeSoFar.length;
      setDisplayedCode(codeSoFar);
      setCurrentSegmentIndex(i);
      setCurrentNarration(seg.narration);
      const onComplete = () => {
        const remainingMs = Math.max(200, 0.2 * audioDurationSeconds(seg.audioChunks) * 1000);
        playNextTimeoutRef.current = setTimeout(() => {
          playNextTimeoutRef.current = null;
          if (!isPlayingRef.current) return;
          if (i < segments.length - 1) replaySegment(i + 1);
          else setIsPlaying(false);
        }, remainingMs);
      };
      if (seg.code.length > 0) {
        const speedMs = typingSpeedFor80Percent(seg.code.length, seg.audioChunks);
        typeSegment(seg.code, speedMs, { updateCode: false, onComplete });
      } else {
        // Narration-only segment (e.g. wrapping): just play audio, then run onComplete after duration
        const durationMs = Math.max(200, audioDurationSeconds(seg.audioChunks) * 1000);
        playNextTimeoutRef.current = setTimeout(() => {
          playNextTimeoutRef.current = null;
          onComplete();
        }, durationMs);
      }
      seg.audioChunks.forEach(playAudioChunk);
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
              const completedSeg: Segment = {
                index: 0,
                code: pending.code,
                narration: pending.narration,
                audioChunks: [...pendingChunks],
              };
              setSegments((prev) => {
                const newSeg = { ...completedSeg, index: prev.length };
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
            const pending = pendingSegmentRef.current;
            const pendingChunks = pendingAudioChunksRef.current;
            if (pending) {
              const lastSeg: Segment = {
                index: 0,
                code: pending.code,
                narration: pending.narration,
                audioChunks: [...pendingChunks],
              };
              setSegments((prev) => {
                const newSeg = { ...lastSeg, index: prev.length };
                return [...prev, newSeg];
              });
            }
            pendingSegmentRef.current = null;
            pendingAudioChunksRef.current = [];
            // Show full code immediately (no typing animation during generation)
            const full = (msg.code || "").trim();
            setCode(full);
            setDisplayedCode(full);
            streamCodeBufferRef.current = full;
            displayedLenRef.current = full.length;
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
            className={`overflow-hidden border border-zinc-700 bg-zinc-900 h-[400px] flex flex-col ${
              segments.length > 0 ? "rounded-t-lg" : "rounded-lg"
            }`}
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
                        if (start > displayedCode.length) return null;
                        const text = displayedCode.slice(start, end);
                        const isCurrent = i === currentSegmentIndex;
                        return (
                          <span
                            key={i}
                            className={isCurrent ? "bg-zinc-800/70 rounded-sm" : undefined}
                          >
                            {parseSyntaxHighlight(text)}
                          </span>
                        );
                      })}
                      {loading && displayedCode.length > 0 && <span className="animate-pulse">|</span>}
                    </>
                  );
                })()
              ) : (
                <>
                  {parseSyntaxHighlight(displayedCode)}
                  {loading && displayedCode.length > 0 && <span className="animate-pulse">|</span>}
                </>
              )}
            </pre>
          </div>
          {segments.length > 0 && (
            <div className="mt-0 rounded-b-lg bg-zinc-800/90 border-t border-zinc-700 px-3 py-2">
              {/* Controls row */}
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  onClick={goPrevBlock}
                  disabled={currentSegmentIndex <= 0}
                  className="w-8 h-8 flex items-center justify-center rounded-full hover:bg-zinc-700 disabled:opacity-30 disabled:pointer-events-none text-zinc-200 transition-colors"
                  title="Previous segment"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M6 6h2v12H6zm3.5 6 8.5 6V6z"/></svg>
                </button>
                <button
                  type="button"
                  onClick={togglePlayPause}
                  className="w-10 h-10 flex items-center justify-center rounded-full hover:bg-zinc-700 text-zinc-100 transition-colors"
                  title={isPlaying ? "Pause" : "Play"}
                >
                  {isPlaying ? (
                    <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
                  ) : (
                    <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
                  )}
                </button>
                <button
                  type="button"
                  onClick={goNextBlock}
                  disabled={currentSegmentIndex >= segments.length - 1}
                  className="w-8 h-8 flex items-center justify-center rounded-full hover:bg-zinc-700 disabled:opacity-30 disabled:pointer-events-none text-zinc-200 transition-colors"
                  title="Next segment"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/></svg>
                </button>
                <span className="text-xs text-zinc-500 ml-2 select-none">
                  {currentSegmentIndex + 1} / {segments.length}
                </span>
              </div>
              {/* Progress bar */}
              <div
                className="mt-1.5 flex h-1.5 w-full rounded-full overflow-hidden bg-zinc-700/60 cursor-pointer"
                role="progressbar"
                aria-label="Segment timeline"
              >
                {segments.map((seg, i) => {
                  const isActive = i === currentSegmentIndex;
                  const isPast = i < currentSegmentIndex;
                  return (
                    <button
                      key={seg.index}
                      type="button"
                      onClick={() => replaySegment(i)}
                      className={`h-full transition-colors border-r border-zinc-900/40 last:border-r-0 ${
                        isActive
                          ? "bg-cyan-500"
                          : isPast
                          ? "bg-cyan-800"
                          : "bg-zinc-600 hover:bg-zinc-500"
                      }`}
                      style={{ flex: seg.code.length || 1 }}
                      title={seg.narration || `Segment ${i + 1}`}
                    />
                  );
                })}
              </div>
            </div>
          )}
          {/* Foldable debug: raw LLM output with tags */}
          {(code || segments.length > 0) && (
            <div className="mt-4 border border-zinc-700 rounded-lg overflow-hidden bg-zinc-900/50">
              <button
                type="button"
                onClick={() => setShowRawDebug((v) => !v)}
                className="w-full flex items-center justify-between px-3 py-2 text-left text-sm font-medium text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50 transition-colors"
              >
                <span>Raw LLM output (debug)</span>
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
                  {segments.length > 0 ? segments.map((s) => s.code).join("") : code}
                </pre>
              )}
            </div>
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

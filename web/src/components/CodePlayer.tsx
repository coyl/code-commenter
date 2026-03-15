"use client";

import React, {
  useState,
  useCallback,
  useRef,
  useEffect,
  forwardRef,
  useImperativeHandle,
} from "react";
import { audioDurationSeconds, pcmChunksToWavBlob, usePCMPlayer } from "@/lib/audio";
import { getHTMLChunks, typingSpeedFor80Percent } from "@/lib/codePlayer";
import type { Segment } from "@/domain/stream";

export type { Segment };

export type CodePlayerRef = {
  playSegment: (i: number) => void;
};

export type CodePlayerAudio = {
  playChunk: (base64PCM: string) => void;
  stop: () => void;
  unlock: () => void | Promise<void>;
  remainingMs: () => number;
  getDebugState?: () => { hasContext: boolean; contextState: string };
};

export type CodePlayerDebugSample = {
  contextState: string;
  hasContext: boolean;
  isPlaying: boolean;
  queuedMs: number;
  playedChunks: number;
  totalChunks: number;
};

export type CodePlayerProps = {
  segments: Segment[];
  displayedCode: string;
  onDisplayedCodeChange: (html: string) => void;
  sessionId?: string | null;
  /** When set, show a button that opens the job view (e.g. /jobs/{jobId}) on the current origin (iframe-friendly). */
  jobId?: string | null;
  loading?: boolean;
  /** When provided (e.g. main page streaming), last segment will wait for next segment instead of stopping. */
  streamEndedRef?: React.MutableRefObject<boolean>;
  /** When true (e.g. embed with ?autoplay=1), start playback from segment 0 once segments are ready. */
  autoplay?: boolean;
  /** Shared audio functions. When provided, CodePlayer uses these instead of its own AudioContext.
   *  This ensures the AudioContext unlocked in a user gesture (e.g. clicking Generate) is the same one used for playback — critical on iOS. */
  audio?: CodePlayerAudio;
  /** Optional debug stream for diagnostics (used by jobs page, not embed). */
  onDebugSample?: (sample: CodePlayerDebugSample) => void;
  /** Base64-encoded PNG (640x480) shown as a poster overlay before the first play action. Hidden once playback starts. */
  previewImageBase64?: string | null;
};

const CodePlayer = forwardRef<CodePlayerRef, CodePlayerProps>(function CodePlayer(
  {
    segments,
    displayedCode,
    onDisplayedCodeChange,
    sessionId = null,
    jobId = null,
    loading = false,
    streamEndedRef,
    autoplay = false,
    audio: externalAudio,
    onDebugSample,
    previewImageBase64 = null,
  },
  ref
) {
  const [currentSegmentIndex, setCurrentSegmentIndex] = useState(0);
  const [currentNarration, setCurrentNarration] = useState("");
  const [narrationDurationMs, setNarrationDurationMs] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const [hasStartedPlayback, setHasStartedPlayback] = useState(false);
  const autoplayTriggeredRef = useRef(false);
  const [copyJustDone, setCopyJustDone] = useState(false);
  const [narrationReplayKey, setNarrationReplayKey] = useState(0);
  const [embedPopupOpen, setEmbedPopupOpen] = useState(false);
  const [embedCopied, setEmbedCopied] = useState(false);
  const embedResetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const codeContainerRef = useRef<HTMLPreElement>(null);
  const currentSegmentRef = useRef<HTMLSpanElement | null>(null);
  const streamCodeBufferRef = useRef("");
  const typingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const playNextTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isPlayingRef = useRef(false);
  const segmentsRef = useRef<Segment[]>([]);
  const waitingForSegmentRef = useRef<number | null>(null);
  /** Incremented on each replaySegment start; used to ignore stale playSegmentAudio promise callbacks (e.g. on iOS when user taps another segment before el.play() resolves). */
  const playbackGenRef = useRef(0);
  const narrationContainerRef = useRef<HTMLDivElement>(null);
  const narrationInnerRef = useRef<HTMLDivElement>(null);
  const [narrationScrollPx, setNarrationScrollPx] = useState(0);

  const ownAudio = usePCMPlayer();
  const playAudioChunk = externalAudio?.playChunk ?? ownAudio.playChunk;
  const stopAudio = externalAudio?.stop ?? ownAudio.stop;
  const unlockAudio = externalAudio?.unlock ?? ownAudio.unlock;
  const audioRemainingMs = externalAudio?.remainingMs ?? ownAudio.remainingMs;
  const getAudioDebugState = externalAudio?.getDebugState ?? ownAudio.getDebugState;
  const playedChunksRef = useRef(0);
  const iosAudioRef = useRef<HTMLAudioElement | null>(null);
  const iosObjectUrlRef = useRef<string | null>(null);
  const mediaEndsAtRef = useRef(0);
  const isIOSRef = useRef(false);

  useEffect(() => {
    if (typeof navigator === "undefined") return;
    const ua = navigator.userAgent || "";
    const platform = navigator.platform || "";
    const maxTouch = (navigator as Navigator & { maxTouchPoints?: number }).maxTouchPoints ?? 0;
    isIOSRef.current =
      /iPad|iPhone|iPod/.test(ua) ||
      (platform === "MacIntel" && maxTouch > 1);
  }, []);

  useEffect(() => {
    segmentsRef.current = segments;
    playedChunksRef.current = 0;
  }, [segments]);

  useEffect(() => {
    isPlayingRef.current = isPlaying;
  }, [isPlaying]);

  useEffect(() => {
    return () => {
      stopAudio();
      if (iosAudioRef.current) {
        iosAudioRef.current.pause();
        iosAudioRef.current.removeAttribute("src");
      }
      if (iosObjectUrlRef.current) {
        URL.revokeObjectURL(iosObjectUrlRef.current);
        iosObjectUrlRef.current = null;
      }
      if (typingTimerRef.current) clearInterval(typingTimerRef.current);
    };
  }, [stopAudio]);

  // Keep the current segment (cursor) in view when content overflows
  useEffect(() => {
    const segmentEl = currentSegmentRef.current;
    if (segmentEl) {
      segmentEl.scrollIntoView({ block: "nearest", behavior: "auto" });
      return;
    }
    const el = codeContainerRef.current;
    if (el) el.scrollTop = el.scrollHeight - el.clientHeight;
  }, [displayedCode, currentSegmentIndex]);

  // Measure narration line vs container; only scroll when text is wider than container
  useEffect(() => {
    if (!currentNarration) {
      setNarrationScrollPx(0);
      return;
    }
    const id = requestAnimationFrame(() => {
      const container = narrationContainerRef.current;
      const inner = narrationInnerRef.current;
      if (!container || !inner) {
        setNarrationScrollPx(0);
        return;
      }
      const overflow = inner.scrollWidth - container.clientWidth;
      setNarrationScrollPx(overflow > 0 ? overflow : 0);
    });
    return () => cancelAnimationFrame(id);
  }, [currentNarration, currentSegmentIndex]);

  const typeSegment = useCallback(
    (
      segmentCode: string,
      msPerChunk: number,
      prefix: string,
      onComplete?: () => void
    ) => {
      if (typingTimerRef.current) clearInterval(typingTimerRef.current);
      streamCodeBufferRef.current = prefix + segmentCode;
      const chunks = getHTMLChunks(segmentCode);
      if (chunks.length === 0) {
        onComplete?.();
        return;
      }
      // Use the 80%-based interval directly so typing finishes at 80% of audio (min 1ms to avoid runaway)
      const intervalMs = Math.max(1, msPerChunk);
      let chunkIdx = 0;
      typingTimerRef.current = setInterval(() => {
        chunkIdx += 1;
        const displayed = prefix + chunks.slice(0, chunkIdx).join("");
        onDisplayedCodeChange(displayed);
        if (chunkIdx >= chunks.length) {
          if (typingTimerRef.current) {
            clearInterval(typingTimerRef.current);
            typingTimerRef.current = null;
          }
          onComplete?.();
        }
      }, intervalMs);
    },
    [onDisplayedCodeChange]
  );

  const stopIOSAudio = useCallback(() => {
    mediaEndsAtRef.current = 0;
    const el = iosAudioRef.current;
    if (!el) return;
    try {
      el.pause();
      el.currentTime = 0;
    } catch {
      // ignore
    }
    if (iosObjectUrlRef.current) {
      URL.revokeObjectURL(iosObjectUrlRef.current);
      iosObjectUrlRef.current = null;
    }
    el.removeAttribute("src");
    el.load();
  }, []);

  const playSegmentAudio = useCallback(
    async (audioChunks: string[]): Promise<number> => {
      if (audioChunks.length === 0) return 0;
      const durationMs = Math.round(audioDurationSeconds(audioChunks) * 1000);
      if (!isIOSRef.current || !iosAudioRef.current) {
        audioChunks.forEach(playAudioChunk);
        playedChunksRef.current += audioChunks.length;
        return durationMs;
      }

      const el = iosAudioRef.current;
      stopIOSAudio();
      const wavBlob = pcmChunksToWavBlob(audioChunks);
      const url = URL.createObjectURL(wavBlob);
      iosObjectUrlRef.current = url;
      el.src = url;
      el.preload = "auto";
      el.setAttribute("playsinline", "true");
      try {
        await el.play();
        mediaEndsAtRef.current = Date.now() + durationMs;
      } catch {
        mediaEndsAtRef.current = 0;
      }
      playedChunksRef.current += audioChunks.length;
      return durationMs;
    },
    [playAudioChunk, stopIOSAudio]
  );

  const playbackRemainingMs = useCallback((): number => {
    if (isIOSRef.current) {
      return Math.max(0, mediaEndsAtRef.current - Date.now());
    }
    return audioRemainingMs();
  }, [audioRemainingMs]);

  const replaySegment = useCallback(
    (i: number) => {
      const segs = segmentsRef.current;
      if (i < 0 || i >= segs.length) return;
      setNarrationReplayKey((k) => k + 1);
      setHasStartedPlayback(true);
      if (playNextTimeoutRef.current) {
        clearTimeout(playNextTimeoutRef.current);
        playNextTimeoutRef.current = null;
      }
      stopAudio();
      stopIOSAudio();
      if (typingTimerRef.current) {
        clearInterval(typingTimerRef.current);
        typingTimerRef.current = null;
      }
      setIsPlaying(true);
      const seg = segs[i];
      const hasAudio = seg.audioChunks && seg.audioChunks.length > 0;

      if (!hasAudio) {
        const codeSoFar = segs
          .slice(0, i + 1)
          .map((s) => s.code)
          .join("");
        onDisplayedCodeChange(codeSoFar);
        streamCodeBufferRef.current = codeSoFar;
        const nextIndex = i + 1;
        setCurrentSegmentIndex(Math.min(nextIndex, segs.length - 1));
        setCurrentNarration("");
        setNarrationDurationMs(0);
        playNextTimeoutRef.current = setTimeout(() => {
          playNextTimeoutRef.current = null;
          if (!isPlayingRef.current) return;
          const currentLen = segmentsRef.current.length;
          if (nextIndex < currentLen) replaySegment(nextIndex);
          else if (streamEndedRef && !streamEndedRef.current) {
            waitingForSegmentRef.current = nextIndex;
          } else setIsPlaying(false);
        }, 50);
        return;
      }

      const codeSoFar = segs
        .slice(0, i)
        .map((s) => s.code)
        .join("");
      streamCodeBufferRef.current = codeSoFar;
      onDisplayedCodeChange(codeSoFar);
      const gen = ++playbackGenRef.current;
      const startPlaybackAndContinue = (durationMs: number) => {
        if (playbackGenRef.current !== gen) return;
        setCurrentSegmentIndex(i);
        setCurrentNarration(seg.narration ?? "");
        setNarrationDurationMs(durationMs);
        const onComplete = () => {
          if (playbackGenRef.current !== gen) return;
          const remaining = Math.max(200, playbackRemainingMs());
          playNextTimeoutRef.current = setTimeout(() => {
            playNextTimeoutRef.current = null;
            if (!isPlayingRef.current) return;
            if (playbackGenRef.current !== gen) return;
            const currentLen = segmentsRef.current.length;
            if (i < currentLen - 1) replaySegment(i + 1);
            else if (i === currentLen - 1 && streamEndedRef && !streamEndedRef.current) {
              waitingForSegmentRef.current = i + 1;
            } else setIsPlaying(false);
          }, remaining);
        };
        if (seg.code?.length > 0) {
          const chunks = getHTMLChunks(seg.code);
          const speedMs = typingSpeedFor80Percent(chunks.length, seg.audioChunks);
          typeSegment(seg.code, speedMs, codeSoFar, onComplete);
        } else {
          const waitMs = Math.max(200, playbackRemainingMs());
          playNextTimeoutRef.current = setTimeout(() => {
            playNextTimeoutRef.current = null;
            if (!isPlayingRef.current) return;
            if (playbackGenRef.current !== gen) return;
            const currentLen = segmentsRef.current.length;
            if (i < currentLen - 1) replaySegment(i + 1);
            else if (i === currentLen - 1 && streamEndedRef && !streamEndedRef.current) {
              waitingForSegmentRef.current = i + 1;
            } else setIsPlaying(false);
          }, waitMs);
        }
      };
      Promise.resolve(playSegmentAudio(seg.audioChunks))
        .then((durationMs) => {
          if (playbackGenRef.current !== gen) return;
          startPlaybackAndContinue(durationMs);
        })
        .catch(() => {
          if (playbackGenRef.current !== gen) return;
          startPlaybackAndContinue(Math.round(audioDurationSeconds(seg.audioChunks) * 1000));
        });
    },
    [typeSegment, stopAudio, stopIOSAudio, playbackRemainingMs, streamEndedRef, playSegmentAudio, onDisplayedCodeChange]
  );

  useEffect(() => {
    if (!onDebugSample) return;
    const send = () => {
      const dbg = getAudioDebugState?.() ?? { hasContext: false, contextState: "unknown" };
      const totalChunks = segmentsRef.current.reduce(
        (sum, s) => sum + (s.audioChunks?.length ?? 0),
        0
      );
      const contextState = isIOSRef.current
        ? `html-audio:${iosAudioRef.current?.paused ? "paused" : "playing"}`
        : dbg.contextState;
      onDebugSample({
        contextState,
        hasContext: isIOSRef.current ? Boolean(iosAudioRef.current) : dbg.hasContext,
        isPlaying: isPlayingRef.current,
        queuedMs: Math.round(playbackRemainingMs()),
        playedChunks: playedChunksRef.current,
        totalChunks,
      });
    };
    send();
    const id = setInterval(send, 500);
    return () => clearInterval(id);
  }, [onDebugSample, getAudioDebugState, playbackRemainingMs]);

  useImperativeHandle(
    ref,
    () => ({
      playSegment(i: number) {
        if (
          isPlayingRef.current &&
          waitingForSegmentRef.current === i &&
          segmentsRef.current.length > i
        ) {
          waitingForSegmentRef.current = null;
          replaySegment(i);
        }
      },
    }),
    [replaySegment]
  );

  const stopCurrentPlayback = useCallback(() => {
    if (playNextTimeoutRef.current) {
      clearTimeout(playNextTimeoutRef.current);
      playNextTimeoutRef.current = null;
    }
    waitingForSegmentRef.current = null;
    stopAudio();
    stopIOSAudio();
    if (typingTimerRef.current) {
      clearInterval(typingTimerRef.current);
      typingTimerRef.current = null;
    }
    setIsPlaying(false);
  }, [stopAudio, stopIOSAudio]);

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

  const togglePlayPause = useCallback(async () => {
    if (isPlaying) {
      stopCurrentPlayback();
    } else {
      stopCurrentPlayback();
      await Promise.resolve(unlockAudio());
      setIsPlaying(true);
      replaySegment(currentSegmentIndex);
    }
  }, [isPlaying, currentSegmentIndex, replaySegment, stopCurrentPlayback, unlockAudio]);

  // Embed autoplay: when ?autoplay=1 and segments are ready, start playback from segment 0 once.
  useEffect(() => {
    if (!autoplay || segments.length === 0 || autoplayTriggeredRef.current) return;
    autoplayTriggeredRef.current = true;
    Promise.resolve(unlockAudio()).then(() => {
      setCurrentSegmentIndex(0);
      setIsPlaying(true);
      replaySegment(0);
    });
  }, [autoplay, segments.length, unlockAudio, replaySegment]);

  const fullCodePlain = segments.map((s) => s.codePlain ?? s.code).join("");
  const copyResetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const handleCopyFullCode = useCallback(() => {
    if (!fullCodePlain) return;
    navigator.clipboard.writeText(fullCodePlain).then(() => {
      if (copyResetTimerRef.current) clearTimeout(copyResetTimerRef.current);
      setCopyJustDone(true);
      copyResetTimerRef.current = setTimeout(() => {
        setCopyJustDone(false);
        copyResetTimerRef.current = null;
      }, 2000);
    }).catch(() => {});
  }, [fullCodePlain]);

  useEffect(() => () => {
    if (copyResetTimerRef.current) clearTimeout(copyResetTimerRef.current);
    if (embedResetTimerRef.current) clearTimeout(embedResetTimerRef.current);
  }, []);

  const embedSnippet =
    typeof window !== "undefined" && jobId
      ? `<div id="my-player"></div>
<script
  src="${window.location.origin}/embed-player.js"
  data-code-commenter-embed
  data-job-id="${jobId.replace(/"/g, "&quot;")}"
  data-target="#my-player"
  data-width="100%"
  data-height="640"
></script>`
      : "";

  const handleCopyEmbed = useCallback(() => {
    if (!embedSnippet) return;
    navigator.clipboard.writeText(embedSnippet).then(() => {
      if (embedResetTimerRef.current) clearTimeout(embedResetTimerRef.current);
      setEmbedCopied(true);
      embedResetTimerRef.current = setTimeout(() => {
        setEmbedCopied(false);
        embedResetTimerRef.current = null;
      }, 2000);
    }).catch(() => {});
  }, [embedSnippet]);

  const cumLengths: number[] = [0];
  for (const s of segments) cumLengths.push(cumLengths[cumLengths.length - 1] + s.code.length);

  const totalLength = segments.reduce((sum, s) => sum + (s.codePlain?.length || 1), 0);
  const minLastFlex = Math.max(totalLength * 0.05, 1);

  return (
    <>
    <audio ref={iosAudioRef} style={{ display: "none" }} playsInline />
    <section className="mb-6">
      <style>{`@keyframes codePlayerNarrationScroll { from { transform: translateX(0); } to { transform: translateX(var(--narration-scroll-end, 0)); } }`}</style>
      <div
        id="code-view"
        className={`relative overflow-hidden border border-zinc-800/80 bg-zinc-900/95 h-[380px] md:h-[420px] flex flex-col shadow-lg shadow-black/30 ${
          segments.length > 0 ? "rounded-t-xl" : "rounded-xl"
        }`}
      >
        {previewImageBase64 && !hasStartedPlayback && (
          /* eslint-disable-next-line @next/next/no-img-element */
          <img
            src={`data:image/png;base64,${previewImageBase64}`}
            alt="Preview"
            className="absolute inset-0 w-full h-full object-cover z-[5] pointer-events-none"
            aria-hidden
          />
        )}
        {segments.length > 0 && !isPlaying && (
          <button
            type="button"
            onClick={togglePlayPause}
            className="absolute inset-0 z-10 flex items-center justify-center bg-black/40 transition-opacity hover:bg-black/30 focus:outline-none focus:ring-2 focus:ring-cyan-500/50 focus:ring-inset"
            aria-label="Play"
          >
            <span className="flex h-16 items-center justify-center rounded-2xl px-8 py-5 bg-cyan-600 hover:bg-cyan-500 shadow-xl shadow-cyan-900/50 transition-transform hover:scale-105 active:scale-95">
              <svg className="-ml-[3px] h-9 w-9 text-white" viewBox="0 0 24 24" fill="currentColor" aria-hidden>
                <path d="M8 5v14l11-7z" />
              </svg>
            </span>
          </button>
        )}
        <pre
          ref={codeContainerRef}
          className="p-4 text-sm overflow-y-auto overflow-x-hidden font-mono whitespace-pre-wrap break-words text-zinc-100 flex-1 min-h-0 scrollbar-hide"
        >
          {segments.length > 0 ? (
            <>
              {!hasStartedPlayback ? null : segments.map((_, i) => {
                const start = cumLengths[i];
                const end = Math.min(cumLengths[i + 1], displayedCode.length);
                if (start > displayedCode.length) return null;
                const text = displayedCode.slice(start, end);
                const isCurrent = i === currentSegmentIndex;
                return (
                  <span
                    key={i}
                    ref={isCurrent ? currentSegmentRef : undefined}
                    className={isCurrent ? "block bg-zinc-800/70 rounded-sm py-0.5 -my-0.5" : undefined}
                  >
                    <span dangerouslySetInnerHTML={{ __html: text }} />
                    {isCurrent && hasStartedPlayback && (
                      <span
                        className="inline-block w-0.5 h-4 align-middle bg-zinc-100 animate-pulse ml-0.5"
                        aria-hidden
                      />
                    )}
                  </span>
                );
              })}
            </>
          ) : (
            <>
              <span dangerouslySetInnerHTML={{ __html: displayedCode }} />
              {displayedCode.length > 0 && (
                <span
                  className="inline-block w-0.5 h-4 align-middle bg-zinc-100 animate-pulse ml-0.5"
                  aria-hidden
                />
              )}
            </>
          )}
        </pre>
      </div>
      {segments.length > 0 && (
        <div className="rounded-b-xl bg-zinc-900/95 border-t border-zinc-800/60 px-3 py-2.5">
          <div className="flex items-center gap-0.5">
            <button
              type="button"
              onClick={goPrevBlock}
              disabled={currentSegmentIndex <= 0}
              className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-zinc-800 disabled:opacity-25 disabled:pointer-events-none text-zinc-400 hover:text-zinc-200 transition-colors"
              title="Previous segment"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M6 6h2v12H6zm3.5 6 8.5 6V6z"/></svg>
            </button>
            <button
              type="button"
              onClick={togglePlayPause}
              className="w-9 h-9 flex items-center justify-center rounded-lg hover:bg-zinc-800 text-zinc-200 hover:text-white transition-colors"
              title={isPlaying ? "Pause" : "Play"}
            >
              {isPlaying ? (
                <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg>
              ) : (
                <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg>
              )}
            </button>
            <button
              type="button"
              onClick={goNextBlock}
              disabled={currentSegmentIndex >= segments.length - 1}
              className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-zinc-800 disabled:opacity-25 disabled:pointer-events-none text-zinc-400 hover:text-zinc-200 transition-colors"
              title="Next segment"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor"><path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/></svg>
            </button>
            <span className="text-xs text-zinc-600 ml-2 select-none tabular-nums">
              {currentSegmentIndex + 1} / {segments.length}
            </span>
            <div className="ml-auto flex items-center gap-0.5">
              {jobId && (
                <a
                  href={`/jobs/${encodeURIComponent(jobId)}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition-colors"
                  title="Open job in new tab"
                >
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" x2="21" y1="14" y2="3"/></svg>
                </a>
              )}
              {jobId && (
                <button
                  type="button"
                  onClick={() => setEmbedPopupOpen(true)}
                  className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition-colors"
                  title="Embed"
                >
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10 8 6 12l4 4M14 8l4 4-4 4"/></svg>
                </button>
              )}
              <button
                type="button"
                onClick={handleCopyFullCode}
                disabled={!fullCodePlain}
                className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-zinc-800 disabled:opacity-25 disabled:pointer-events-none text-zinc-400 hover:text-zinc-200 transition-colors"
                title="Copy full code"
              >
                {copyJustDone ? (
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-green-400"><polyline points="20 6 9 17 4 12"/></svg>
                ) : (
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
                )}
              </button>
            </div>
          </div>
          <div
            className="mt-2 flex h-1 w-full rounded-full overflow-hidden bg-zinc-800 cursor-pointer"
            role="progressbar"
            aria-label="Segment timeline"
          >
            {segments.map((seg, i) => {
              const isActive = i === currentSegmentIndex;
              const isPast = i < currentSegmentIndex;
              const isLast = i === segments.length - 1;
              const flex = isLast ? Math.max(seg.codePlain?.length || 1, minLastFlex) : (seg.codePlain?.length || 1);
              return (
                <button
                  key={seg.index}
                  type="button"
                  onClick={() => replaySegment(i)}
                  className={`h-full transition-colors border-r border-zinc-900/60 last:border-r-0 ${
                    isActive
                      ? "bg-cyan-500"
                      : isPast
                      ? "bg-cyan-800/80"
                      : "bg-zinc-700 hover:bg-zinc-600"
                  }`}
                  style={{ flex }}
                  title={seg.narration || `Segment ${i + 1}`}
                />
              );
            })}
          </div>
          {currentNarration && narrationDurationMs > 0 && (
            <div
              ref={narrationContainerRef}
              className="mt-2 h-4 overflow-hidden text-xs text-zinc-500 italic"
              aria-live="polite"
            >
              <div
                ref={narrationInnerRef}
                key={`${currentSegmentIndex}-${narrationReplayKey}`}
                className="inline-block whitespace-nowrap"
                style={
                  narrationScrollPx > 0
                    ? {
                        animation: `codePlayerNarrationScroll ${narrationDurationMs / 1000}s linear forwards`,
                        ["--narration-scroll-end" as string]: `-${narrationScrollPx}px`,
                        animationPlayState: isPlaying ? "running" : "paused",
                      }
                    : undefined
                }
              >
                {currentNarration}
              </div>
            </div>
          )}
          {sessionId && (
            <p className="mt-2 text-xs text-zinc-600">
              Permalink:{" "}
              <a href={`/jobs/${sessionId}`} className="text-cyan-500/80 hover:text-cyan-400 hover:underline transition-colors">
                /jobs/{sessionId}
              </a>
            </p>
          )}
        </div>
      )}
    </section>

    {embedPopupOpen && jobId && (
      <div
        className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4 sm:p-6 bg-black/60 backdrop-blur-sm"
        onClick={() => setEmbedPopupOpen(false)}
        role="dialog"
        aria-modal="true"
        aria-labelledby="embed-dialog-title"
      >
        <div
          className="bg-zinc-900 border border-zinc-700/80 rounded-2xl shadow-2xl shadow-black/60 max-w-lg w-full max-h-[85vh] flex flex-col"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-center justify-between px-4 py-3.5 border-b border-zinc-800/70">
            <h2 id="embed-dialog-title" className="text-sm font-semibold text-zinc-200">
              Embed player
            </h2>
            <button
              type="button"
              onClick={() => setEmbedPopupOpen(false)}
              className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-zinc-800 text-zinc-500 hover:text-zinc-200 transition-colors"
              aria-label="Close"
            >
              <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
            </button>
          </div>
          <p className="px-4 pt-3.5 text-xs text-zinc-500 leading-relaxed">
            Copy and paste this snippet into your page to embed the interactive player.
          </p>
          <textarea
            readOnly
            value={embedSnippet}
            className="mx-4 mt-2 p-3 rounded-lg bg-zinc-800/60 border border-zinc-700/60 text-zinc-300 font-mono text-xs resize-none w-[calc(100%-2rem)] h-32 focus:outline-none"
            spellCheck={false}
          />
          <div className="flex items-center justify-end gap-2 px-4 py-3.5 border-t border-zinc-800/70">
            <button
              type="button"
              onClick={() => setEmbedPopupOpen(false)}
              className="px-3 py-1.5 text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded-lg transition-colors"
            >
              Close
            </button>
            <button
              type="button"
              onClick={handleCopyEmbed}
              className="px-4 py-1.5 text-sm bg-cyan-600 hover:bg-cyan-500 active:bg-cyan-700 text-white rounded-lg font-medium transition-colors flex items-center gap-2"
            >
              {embedCopied ? (
                <>
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-green-300" aria-hidden><polyline points="20 6 9 17 4 12"/></svg>
                  Copied!
                </>
              ) : (
                "Copy snippet"
              )}
            </button>
          </div>
        </div>
      </div>
    )}
    </>
  );
});

export default CodePlayer;

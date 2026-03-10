"use client";

import React, {
  useState,
  useCallback,
  useRef,
  useEffect,
  forwardRef,
  useImperativeHandle,
} from "react";
import { audioDurationSeconds, usePCMPlayer } from "@/lib/audio";
import { getHTMLChunks, typingSpeedFor80Percent } from "@/lib/codePlayer";
import type { Segment } from "@/domain/stream";

export type { Segment };

export type CodePlayerRef = {
  playSegment: (i: number) => void;
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
  const streamCodeBufferRef = useRef("");
  const typingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const playNextTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isPlayingRef = useRef(false);
  const segmentsRef = useRef<Segment[]>([]);
  const waitingForSegmentRef = useRef<number | null>(null);
  const narrationContainerRef = useRef<HTMLDivElement>(null);
  const narrationInnerRef = useRef<HTMLDivElement>(null);
  const [narrationScrollPx, setNarrationScrollPx] = useState(0);

  const { playChunk: playAudioChunk, stop: stopAudio, unlock: unlockAudio, remainingMs: audioRemainingMs } = usePCMPlayer();

  useEffect(() => {
    segmentsRef.current = segments;
  }, [segments]);

  useEffect(() => {
    isPlayingRef.current = isPlaying;
  }, [isPlaying]);

  useEffect(() => {
    return () => {
      stopAudio();
      if (typingTimerRef.current) clearInterval(typingTimerRef.current);
    };
  }, [stopAudio]);

  // Scroll to follow the typing (keep end of content in view)
  useEffect(() => {
    const el = codeContainerRef.current;
    if (el) el.scrollTop = el.scrollHeight - el.clientHeight;
  }, [displayedCode]);

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

      const durationMs = Math.round(audioDurationSeconds(seg.audioChunks) * 1000);
      setCurrentSegmentIndex(i);
      setCurrentNarration(seg.narration ?? "");
      setNarrationDurationMs(durationMs);

      const codeSoFar = segs
        .slice(0, i)
        .map((s) => s.code)
        .join("");
      streamCodeBufferRef.current = codeSoFar;
      onDisplayedCodeChange(codeSoFar);
      const onComplete = () => {
        const remaining = Math.max(200, audioRemainingMs());
        playNextTimeoutRef.current = setTimeout(() => {
          playNextTimeoutRef.current = null;
          if (!isPlayingRef.current) return;
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
        seg.audioChunks.forEach(playAudioChunk);
        const waitMs = Math.max(200, audioRemainingMs());
        playNextTimeoutRef.current = setTimeout(() => {
          playNextTimeoutRef.current = null;
          if (!isPlayingRef.current) return;
          const currentLen = segmentsRef.current.length;
          if (i < currentLen - 1) replaySegment(i + 1);
          else if (i === currentLen - 1 && streamEndedRef && !streamEndedRef.current) {
            waitingForSegmentRef.current = i + 1;
          } else setIsPlaying(false);
        }, waitMs);
        return;
      }
      seg.audioChunks.forEach(playAudioChunk);
    },
    [typeSegment, stopAudio, playAudioChunk, audioRemainingMs, streamEndedRef]
  );

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
    if (typingTimerRef.current) {
      clearInterval(typingTimerRef.current);
      typingTimerRef.current = null;
    }
    setIsPlaying(false);
  }, [stopAudio]);

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

  const togglePlayPause = useCallback(() => {
    if (isPlaying) {
      stopCurrentPlayback();
    } else {
      stopCurrentPlayback();
      unlockAudio();
      setIsPlaying(true);
      replaySegment(currentSegmentIndex);
    }
  }, [isPlaying, currentSegmentIndex, replaySegment, stopCurrentPlayback, unlockAudio]);

  // Embed autoplay: when ?autoplay=1 and segments are ready, start playback from segment 0 once.
  useEffect(() => {
    if (!autoplay || segments.length === 0 || autoplayTriggeredRef.current) return;
    autoplayTriggeredRef.current = true;
    unlockAudio();
    setCurrentSegmentIndex(0);
    setIsPlaying(true);
    replaySegment(0);
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
    <section className="mb-6">
      <style>{`@keyframes codePlayerNarrationScroll { from { transform: translateX(0); } to { transform: translateX(var(--narration-scroll-end, 0)); } }`}</style>
      <div
        id="code-view"
        className={`overflow-hidden border border-zinc-700 bg-zinc-900 h-[400px] flex flex-col ${
          segments.length > 0 ? "rounded-t-lg" : "rounded-lg"
        }`}
      >
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
        <div className="mt-0 rounded-b-lg bg-zinc-800/90 border-t border-zinc-700 px-3 py-2">
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
            <div className="ml-auto flex items-center gap-1">
              {jobId && (
                <a
                  href={`/jobs/${encodeURIComponent(jobId)}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="w-8 h-8 flex items-center justify-center rounded-full hover:bg-zinc-700 text-zinc-200 transition-colors"
                  title="Open job in new tab"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" x2="21" y1="14" y2="3"/></svg>
                </a>
              )}
              {jobId && (
                <button
                  type="button"
                  onClick={() => setEmbedPopupOpen(true)}
                  className="w-8 h-8 flex items-center justify-center rounded-full hover:bg-zinc-700 text-zinc-200 transition-colors"
                  title="Embed"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10 8 6 12l4 4M14 8l4 4-4 4"/></svg>
                </button>
              )}
              <button
                type="button"
                onClick={handleCopyFullCode}
                disabled={!fullCodePlain}
                className="w-8 h-8 flex items-center justify-center rounded-full hover:bg-zinc-700 disabled:opacity-30 disabled:pointer-events-none text-zinc-200 transition-colors"
                title="Copy full code"
              >
                {copyJustDone ? (
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-green-500"><polyline points="20 6 9 17 4 12"/></svg>
                ) : (
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>
                )}
              </button>
            </div>
          </div>
          <div
            className="mt-1.5 flex h-1.5 w-full rounded-full overflow-hidden bg-zinc-700/60 cursor-pointer"
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
                  className={`h-full transition-colors border-r border-zinc-900/40 last:border-r-0 ${
                    isActive
                      ? "bg-cyan-500"
                      : isPast
                      ? "bg-cyan-800"
                      : "bg-zinc-600 hover:bg-zinc-500"
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
              className="mt-2 h-5 overflow-hidden text-sm text-zinc-400 italic"
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
            <p className="mt-2 text-xs text-zinc-500">
              Permalink:{" "}
              <a href={`/jobs/${sessionId}`} className="text-cyan-400 hover:underline">
                /jobs/{sessionId}
              </a>
            </p>
          )}
        </div>
      )}
    </section>

    {embedPopupOpen && jobId && (
      <div
        className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60"
        onClick={() => setEmbedPopupOpen(false)}
        role="dialog"
        aria-modal="true"
        aria-labelledby="embed-dialog-title"
      >
        <div
          className="bg-zinc-800 border border-zinc-600 rounded-lg shadow-xl max-w-lg w-full max-h-[85vh] flex flex-col"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-600">
            <h2 id="embed-dialog-title" className="text-sm font-medium text-zinc-200">
              Embed
            </h2>
            <button
              type="button"
              onClick={() => setEmbedPopupOpen(false)}
              className="w-8 h-8 flex items-center justify-center rounded-full hover:bg-zinc-700 text-zinc-400 transition-colors"
              aria-label="Close"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M18 6 6 18"/><path d="m6 6 12 12"/></svg>
            </button>
          </div>
          <p className="px-4 pt-3 text-xs text-zinc-400">
            Copy and paste this code into your page to embed the player.
          </p>
          <textarea
            readOnly
            value={embedSnippet}
            className="mx-4 mt-2 p-3 rounded bg-zinc-900 border border-zinc-600 text-zinc-200 font-mono text-xs resize-none w-[calc(100%-2rem)] h-32"
            spellCheck={false}
          />
          <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-zinc-600">
            <button
              type="button"
              onClick={() => setEmbedPopupOpen(false)}
              className="px-3 py-1.5 text-sm text-zinc-300 hover:text-zinc-100 hover:bg-zinc-700 rounded transition-colors"
            >
              Close
            </button>
            <button
              type="button"
              onClick={handleCopyEmbed}
              className="px-3 py-1.5 text-sm bg-cyan-600 hover:bg-cyan-500 text-white rounded transition-colors flex items-center gap-2"
            >
              {embedCopied ? (
                <>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="text-green-300" aria-hidden><polyline points="20 6 9 17 4 12"/></svg>
                  Copied!
                </>
              ) : (
                "Copy"
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

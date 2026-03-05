"use client";

import React, {
  useState,
  useCallback,
  useRef,
  useEffect,
  forwardRef,
  useImperativeHandle,
} from "react";
import { usePCMPlayer } from "@/lib/audio";
import { getHTMLChunks, typingSpeedFor80Percent } from "@/lib/codePlayer";

export type Segment = {
  index: number;
  code: string;
  codePlain: string;
  narration: string;
  audioChunks: string[];
};

export type CodePlayerRef = {
  playSegment: (i: number) => void;
};

export type CodePlayerProps = {
  segments: Segment[];
  displayedCode: string;
  onDisplayedCodeChange: (html: string) => void;
  sessionId?: string | null;
  loading?: boolean;
  /** When provided (e.g. main page streaming), last segment will wait for next segment instead of stopping. */
  streamEndedRef?: React.MutableRefObject<boolean>;
};

const CodePlayer = forwardRef<CodePlayerRef, CodePlayerProps>(function CodePlayer(
  {
    segments,
    displayedCode,
    onDisplayedCodeChange,
    sessionId = null,
    loading = false,
    streamEndedRef,
  },
  ref
) {
  const [currentSegmentIndex, setCurrentSegmentIndex] = useState(0);
  const [currentNarration, setCurrentNarration] = useState("");
  const [isPlaying, setIsPlaying] = useState(false);

  const streamCodeBufferRef = useRef("");
  const typingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const playNextTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const isPlayingRef = useRef(false);
  const segmentsRef = useRef<Segment[]>([]);
  const waitingForSegmentRef = useRef<number | null>(null);

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

  const typeSegment = useCallback(
    (
      segmentCode: string,
      speedMs: number,
      prefix: string,
      onComplete?: () => void
    ) => {
      if (typingTimerRef.current) clearInterval(typingTimerRef.current);
      streamCodeBufferRef.current = prefix + segmentCode;
      const chunks = getHTMLChunks(segmentCode);
      const visChars = chunks.reduce((n, c) => n + (c.startsWith("<") ? 1 : 1), 0);
      const msPerChunk = Math.max(5, Math.round(speedMs * Math.max(1, visChars / chunks.length)));
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
      }, msPerChunk);
    },
    [onDisplayedCodeChange]
  );

  const replaySegment = useCallback(
    (i: number) => {
      const segs = segmentsRef.current;
      if (i < 0 || i >= segs.length) return;
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
      const codeSoFar = segs
        .slice(0, i)
        .map((s) => s.code)
        .join("");
      streamCodeBufferRef.current = codeSoFar;
      onDisplayedCodeChange(codeSoFar);
      setCurrentSegmentIndex(i);
      setCurrentNarration(seg.narration ?? "");
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

  const cumLengths: number[] = [0];
  for (const s of segments) cumLengths.push(cumLengths[cumLengths.length - 1] + s.code.length);

  const totalLength = segments.reduce((sum, s) => sum + (s.codePlain?.length || 1), 0);
  const minLastFlex = Math.max(totalLength * 0.05, 1);

  return (
    <section className="mb-6">
      <h2 className="text-sm font-medium text-zinc-400 mb-2">Code view</h2>
      <div
        id="code-view"
        className={`overflow-hidden border border-zinc-700 bg-zinc-900 h-[400px] flex flex-col ${
          segments.length > 0 ? "rounded-t-lg" : "rounded-lg"
        }`}
      >
        <pre className="p-4 text-sm overflow-hidden font-mono whitespace-pre text-zinc-100 flex-1 min-h-0">
          {segments.length > 0 ? (
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
                    dangerouslySetInnerHTML={{ __html: text }}
                  />
                );
              })}
              {loading && displayedCode.length > 0 && <span className="animate-pulse">|</span>}
            </>
          ) : (
            <>
              <span dangerouslySetInnerHTML={{ __html: displayedCode }} />
              {loading && displayedCode.length > 0 && <span className="animate-pulse">|</span>}
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
          {currentNarration && (
            <p className="mt-2 text-sm text-zinc-400 italic">{currentNarration}</p>
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
  );
});

export default CodePlayer;

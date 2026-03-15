"use client";

import React, { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import CodePlayer from "@/components/CodePlayer";
import type { Segment } from "@/domain/stream";
import { useJob } from "@/features/job/useJob";

function isAutoplayRequested(searchParams: URLSearchParams | null): boolean {
  if (!searchParams) return false;
  const v = searchParams.get("autoplay")?.toLowerCase();
  return v === "1" || v === "true" || v === "yes";
}

const NARRATION_LANG_LABELS: Record<string, string> = {
  english: "English",
  german: "German",
  spanish: "Spanish",
  italian: "Italian",
  chinese: "Chinese (Simplified)",
};

const OVERLAY_HIDE_DELAY_MS = 2500;

export default function EmbedJobPage() {
  const params = useParams();
  const searchParams = useSearchParams();
  const id = typeof params.id === "string" ? params.id : null;
  const autoplay = isAutoplayRequested(searchParams);
  const { job, loading, error } = useJob(id);
  const [displayedCode, setDisplayedCode] = useState("");
  const [overlayVisible, setOverlayVisible] = useState(false);
  const overlayTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    if (!job) return;

    if (job.css) {
      let el = document.getElementById("dynamic-theme") as HTMLStyleElement | null;
      if (!el) {
        el = document.createElement("style");
        el.id = "dynamic-theme";
        document.head.appendChild(el);
      }
      el.textContent = job.css;
    }

    if (job.fullCode) {
      setDisplayedCode(job.fullCode);
    }
  }, [job]);

  const scheduleOverlayHide = useCallback(() => {
    if (overlayTimeoutRef.current) clearTimeout(overlayTimeoutRef.current);
    overlayTimeoutRef.current = setTimeout(() => {
      setOverlayVisible(false);
      overlayTimeoutRef.current = null;
    }, OVERLAY_HIDE_DELAY_MS);
  }, []);

  const handleMouseMove = useCallback(() => {
    setOverlayVisible(true);
    scheduleOverlayHide();
  }, [scheduleOverlayHide]);

  const handleMouseLeave = useCallback(() => {
    if (overlayTimeoutRef.current) {
      clearTimeout(overlayTimeoutRef.current);
      overlayTimeoutRef.current = null;
    }
    setOverlayVisible(false);
  }, []);

  useEffect(() => {
    return () => {
      if (overlayTimeoutRef.current) clearTimeout(overlayTimeoutRef.current);
    };
  }, []);

  const segments: Segment[] = (job?.segments ?? []).map((s, i) => ({
    ...s,
    index: i,
  }));

  if (loading) {
    return (
      <main className="min-h-[320px] flex items-center justify-center bg-zinc-900">
        <div className="flex items-center gap-3 text-zinc-500 text-sm">
          <span className="w-4 h-4 rounded-full border-2 border-zinc-700 border-t-cyan-500 animate-spin" />
          Loading…
        </div>
      </main>
    );
  }

  if (error || !job) {
    return (
      <main className="min-h-[320px] p-4 flex items-center justify-center bg-zinc-900">
        <div className="px-4 py-3 rounded-xl bg-red-950/40 border border-red-800/50 text-red-300 text-sm">
          {error || "Job not found"}
        </div>
      </main>
    );
  }

  const title = job.title || job.prompt;
  const narrationLabel = job.narrationLang
    ? NARRATION_LANG_LABELS[job.narrationLang.toLowerCase()] || job.narrationLang
    : null;

  return (
    <main
      className="relative w-full bg-zinc-950 rounded-xl overflow-hidden"
      onMouseMove={handleMouseMove}
      onMouseLeave={handleMouseLeave}
    >
      <div className="p-2.5">
        <CodePlayer
          segments={segments}
          displayedCode={displayedCode}
          onDisplayedCodeChange={setDisplayedCode}
          jobId={id}
          autoplay={autoplay}
          previewImageBase64={job.previewImageBase64 || null}
        />
      </div>

      {/* Title overlay: visible on mouse hover */}
      <div
        className="absolute inset-x-0 top-0 py-2.5 px-4 pointer-events-none bg-gradient-to-b from-black/70 to-transparent transition-opacity duration-300"
        style={{ opacity: overlayVisible ? 1 : 0 }}
        aria-hidden
      >
        <div className="flex items-center justify-between gap-2">
          <span className="text-sm font-medium text-white truncate" title={title || undefined}>
            {title || "Code walkthrough"}
          </span>
          {narrationLabel && (
            <span className="text-xs text-zinc-400 shrink-0 tabular-nums">{narrationLabel}</span>
          )}
        </div>
      </div>
    </main>
  );
}

"use client";

import React, { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import CodePlayer from "@/components/CodePlayer";
import type { Segment } from "@/domain/stream";
import { useJob } from "@/features/job/useJob";

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
  const id = typeof params.id === "string" ? params.id : null;
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
      <main className="min-h-[320px] p-4 flex items-center justify-center bg-zinc-900">
        <p className="text-zinc-400 text-sm">Loading player...</p>
      </main>
    );
  }

  if (error || !job) {
    return (
      <main className="min-h-[320px] p-4 flex items-center justify-center bg-zinc-900">
        <div className="p-3 rounded bg-red-900/30 border border-red-700 text-red-200 text-sm">
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
      className="relative w-full bg-zinc-900 rounded-lg overflow-hidden"
      onMouseMove={handleMouseMove}
      onMouseLeave={handleMouseLeave}
    >
      <div className="p-3">
        <CodePlayer
          segments={segments}
          displayedCode={displayedCode}
          onDisplayedCodeChange={setDisplayedCode}
          jobId={id}
        />
      </div>

      {/* YouTube-style overlay: title + narration language on mouse move (top bar, does not cover controls) */}
      <div
        className="absolute inset-x-0 top-0 py-2 px-3 pointer-events-none bg-gradient-to-b from-black/75 to-transparent transition-opacity duration-300"
        style={{ opacity: overlayVisible ? 1 : 0 }}
        aria-hidden
      >
        <div className="flex items-center justify-between gap-2">
          <span className="text-sm font-medium text-white truncate" title={title || undefined}>
            {title || "Code walkthrough"}
          </span>
          {narrationLabel && (
            <span className="text-xs text-zinc-300 shrink-0">Narration: {narrationLabel}</span>
          )}
        </div>
      </div>
    </main>
  );
}

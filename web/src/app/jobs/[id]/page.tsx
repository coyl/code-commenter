"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
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

export default function JobPage() {
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : null;
  const { job, loading, error } = useJob(id);
  const [displayedCode, setDisplayedCode] = useState("");
  const [showPrompt, setShowPrompt] = useState(false);

  // Apply job CSS and initial code when job loads
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

  const segments: Segment[] = (job?.segments ?? []).map((s, i) => ({
    ...s,
    index: i,
  }));

  if (loading) {
    return (
      <main className="min-h-screen p-6 max-w-5xl mx-auto">
        <p className="text-zinc-400">Loading job…</p>
      </main>
    );
  }
  if (error || !job) {
    return (
      <main className="min-h-screen p-6 max-w-5xl mx-auto">
        <div className="mb-4 p-3 rounded bg-red-900/30 border border-red-700 text-red-200 text-sm">
          {error || "Job not found"}
        </div>
        <Link href="/" className="text-cyan-400 hover:underline">
          ← Back to generator
        </Link>
      </main>
    );
  }

  const title = job.title || job.prompt;
  const narrationLabel = job.narrationLang
    ? NARRATION_LANG_LABELS[job.narrationLang.toLowerCase()] || job.narrationLang
    : null;

  return (
    <main className="min-h-screen p-6 max-w-5xl mx-auto">
      <div className="mb-4 flex items-center justify-between gap-4 flex-wrap">
        <Link href="/" className="text-cyan-400 hover:underline text-sm">
          ← New generation
        </Link>
        <div className="flex items-center gap-3">
          {job.storyHtml && id && (
            <Link
              href={`/story/${id}`}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-zinc-800 border border-zinc-600 text-zinc-200 hover:text-white hover:border-zinc-400 text-sm font-medium transition-colors"
            >
              View story
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                <polyline points="15 3 21 3 21 9" />
                <line x1="10" y1="14" x2="21" y2="3" />
              </svg>
            </Link>
          )}
          {narrationLabel && (
            <span className="text-xs text-zinc-500">Narration: {narrationLabel}</span>
          )}
        </div>
      </div>

      {title && (
        <h1 className="text-lg font-medium text-zinc-200 mb-4 truncate" title={title}>
          {title}
        </h1>
      )}

      <section className="mb-6 border border-zinc-700 rounded-lg overflow-hidden bg-zinc-900/50">
        <button
          type="button"
          onClick={() => setShowPrompt((v) => !v)}
          className="w-full flex items-center justify-between px-3 py-2 text-left text-sm font-medium text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50 transition-colors"
        >
          <span>Prompt</span>
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="currentColor"
            className={`transition-transform ${showPrompt ? "rotate-180" : ""}`}
          >
            <path d="M7 10l5 5 5-5z" />
          </svg>
        </button>
        {showPrompt && (
          <div className="p-3 text-zinc-200 whitespace-pre-wrap text-sm border-t border-zinc-700">
            {job.prompt}
          </div>
        )}
      </section>

      <CodePlayer
        segments={segments}
        displayedCode={displayedCode}
        onDisplayedCodeChange={setDisplayedCode}
        jobId={id}
      />
    </main>
  );
}

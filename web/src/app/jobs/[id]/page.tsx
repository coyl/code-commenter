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
      <main className="min-h-screen bg-zinc-950 px-4 py-10 max-w-4xl mx-auto">
        <div className="flex items-center gap-3 text-zinc-500 text-sm">
          <span className="w-4 h-4 rounded-full border-2 border-zinc-600 border-t-cyan-500 animate-spin" />
          Loading job…
        </div>
      </main>
    );
  }

  if (error || !job) {
    return (
      <main className="min-h-screen bg-zinc-950 px-4 py-10 max-w-4xl mx-auto">
        <div className="mb-5 px-4 py-3 rounded-xl bg-red-950/40 border border-red-800/50 text-red-300 text-sm">
          {error || "Job not found"}
        </div>
        <Link href="/" className="text-cyan-500/80 hover:text-cyan-400 text-sm transition-colors">
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
    <main className="min-h-screen bg-zinc-950 px-4 py-6 md:py-8 max-w-4xl mx-auto">
      {/* Top nav row */}
      <div className="mb-6 flex items-center justify-between gap-4 flex-wrap">
        <Link
          href="/"
          className="text-zinc-500 hover:text-zinc-300 text-sm transition-colors flex items-center gap-1.5"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
            <path d="M19 12H5"/><path d="m12 19-7-7 7-7"/>
          </svg>
          New generation
        </Link>
        <div className="flex items-center gap-2 flex-wrap">
          {narrationLabel && (
            <span className="px-2.5 py-1 rounded-full bg-zinc-800/70 border border-zinc-700/50 text-xs text-zinc-400">
              {narrationLabel}
            </span>
          )}
          {job.storyHtml && id && (
            <Link
              href={`/story/${id}`}
              className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-zinc-800/70 border border-zinc-700/60 text-zinc-300 hover:text-white hover:border-zinc-500 text-sm font-medium transition-colors"
            >
              View story
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                <polyline points="15 3 21 3 21 9" />
                <line x1="10" y1="14" x2="21" y2="3" />
              </svg>
            </Link>
          )}
        </div>
      </div>

      {/* Title */}
      {title && (
        <h1 className="text-xl font-semibold text-zinc-100 mb-5 leading-snug" title={title}>
          {title}
        </h1>
      )}

      {/* Prompt collapsible */}
      <section className="mb-5 border border-zinc-800/70 rounded-xl overflow-hidden bg-zinc-900/50">
        <button
          type="button"
          onClick={() => setShowPrompt((v) => !v)}
          className="w-full flex items-center justify-between px-4 py-2.5 text-left"
        >
          <span className="text-xs font-semibold uppercase tracking-wider text-zinc-500">Prompt</span>
          <svg
            width="15"
            height="15"
            viewBox="0 0 24 24"
            fill="currentColor"
            className={`text-zinc-600 transition-transform duration-200 ${showPrompt ? "rotate-180" : ""}`}
            aria-hidden
          >
            <path d="M7 10l5 5 5-5z" />
          </svg>
        </button>
        {showPrompt && (
          <div className="px-4 py-3 text-zinc-300 whitespace-pre-wrap text-sm leading-relaxed border-t border-zinc-800/70">
            {job.prompt}
          </div>
        )}
      </section>

      <CodePlayer
        segments={segments}
        displayedCode={displayedCode}
        onDisplayedCodeChange={setDisplayedCode}
        jobId={id}
        previewImageBase64={job.previewImageBase64 || null}
      />

      {(job.previewImageBase64 || job.illustrationImageBase64) && (
        <div className="mt-6 flex flex-col gap-4">
          <h2 className="text-xs font-semibold uppercase tracking-wider text-zinc-500">Generated visuals</h2>
          <div className="flex flex-wrap gap-4">
            {job.previewImageBase64 && (
              <div className="flex flex-col gap-1.5">
                <span className="text-xs text-zinc-600">Preview thumbnail</span>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={`data:image/png;base64,${job.previewImageBase64}`}
                  alt="Video preview thumbnail"
                  className="rounded-lg border border-zinc-800/70 w-full max-w-[320px]"
                  width={320}
                  height={240}
                />
              </div>
            )}
            {job.illustrationImageBase64 && (
              <div className="flex flex-col gap-1.5">
                <span className="text-xs text-zinc-600">Article illustration</span>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  src={`data:image/png;base64,${job.illustrationImageBase64}`}
                  alt="Article illustration"
                  className="rounded-lg border border-zinc-800/70 w-full max-w-[320px]"
                  width={320}
                  height={240}
                />
              </div>
            )}
          </div>
        </div>
      )}
    </main>
  );
}

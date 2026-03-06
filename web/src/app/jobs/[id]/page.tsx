"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import CodePlayer from "@/components/CodePlayer";
import type { Segment } from "@/domain/stream";
import { useJob } from "@/features/job/useJob";

export default function JobPage() {
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : null;
  const { job, loading, error } = useJob(id);
  const [displayedCode, setDisplayedCode] = useState("");
  const [showPrompt, setShowPrompt] = useState(false);

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

  return (
    <main className="min-h-screen p-6 max-w-5xl mx-auto">
      <div className="mb-4">
        <Link href="/" className="text-cyan-400 hover:underline text-sm">
          ← New generation
        </Link>
      </div>

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
      />
    </main>
  );
}

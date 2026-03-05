"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import CodePlayer, { type Segment } from "@/components/CodePlayer";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

type JobData = {
  prompt: string;
  rawJson: string;
  fullCode: string;
  segments: Array<{
    code: string;
    codePlain: string;
    narration: string;
    audioChunks: string[];
  }>;
};

export default function JobPage() {
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : "";
  const [job, setJob] = useState<JobData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [displayedCode, setDisplayedCode] = useState("");
  const [showPrompt, setShowPrompt] = useState(false);

  useEffect(() => {
    if (!id) {
      setLoading(false);
      setError("Missing job id");
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetch(`${API_BASE}/jobs/${id}`)
      .then((res) => {
        if (!res.ok) throw new Error(res.status === 404 ? "Job not found" : "Failed to load job");
        return res.json();
      })
      .then((data: JobData) => {
        if (!cancelled) {
          setJob(data);
          setDisplayedCode(data.fullCode ?? "");
        }
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : "Failed to load");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  const segments: Segment[] = (job?.segments ?? []).map((s, i) => ({ ...s, index: i }));

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

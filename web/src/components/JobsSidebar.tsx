"use client";

import React, { useState, useEffect, useCallback, useRef } from "react";
import Link from "next/link";
import { fetchApiAdapter } from "@/adapters/api";
import type { JobMeta } from "@/domain/api";

type JobsSidebarProps = {
  open: boolean;
  onToggle: () => void;
  /** When true, user is signed in and we can fetch /jobs/mine. */
  signedIn: boolean;
  /** Refresh list after a new job (e.g. sessionId just set). */
  refreshTrigger?: string | null;
  api?: { listMyJobs: (limit?: number) => Promise<JobMeta[]> };
};

function formatDate(ms: number): string {
  const d = new Date(ms);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  if (sameDay) return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  return d.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: d.getFullYear() !== now.getFullYear() ? "numeric" : undefined,
  });
}

export default function JobsSidebar({
  open,
  onToggle,
  signedIn,
  refreshTrigger,
  api = fetchApiAdapter,
}: JobsSidebarProps) {
  const [jobs, setJobs] = useState<JobMeta[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fetchIdRef = useRef(0);

  const fetchJobs = useCallback(async () => {
    if (!signedIn) return;
    fetchIdRef.current += 1;
    const thisFetchId = fetchIdRef.current;
    setLoading(true);
    setError(null);
    try {
      const list = await api.listMyJobs(50);
      if (thisFetchId === fetchIdRef.current) setJobs(list);
    } catch (e) {
      if (thisFetchId === fetchIdRef.current) {
        setError(e instanceof Error ? e.message : "Failed to load jobs");
        setJobs([]);
      }
    } finally {
      if (thisFetchId === fetchIdRef.current) setLoading(false);
    }
  }, [signedIn, api]);

  useEffect(() => {
    if (open && signedIn) fetchJobs();
  }, [open, signedIn, fetchJobs]);

  useEffect(() => {
    if (refreshTrigger && open && signedIn) fetchJobs();
  }, [refreshTrigger, open, signedIn, fetchJobs]);

  if (!signedIn) return null;

  return (
    <aside
      className={[
        // Fixed so opening/closing never shifts main content; main content stays centered in viewport
        "jobs-sidebar fixed left-0 top-14 z-30 flex flex-col bg-zinc-900/95 h-[calc(100vh-3.5rem)]",
        // Mobile: full-width drawer; desktop: narrow strip or expanded width
        open
          ? "w-full border-b border-zinc-800/70 md:w-[264px] md:border-b-0 md:border-r md:border-zinc-800/70"
          : "hidden md:flex md:w-12 md:border-r md:border-zinc-800/70",
      ].join(" ")}
    >
      {/* Header bar — always visible when the aside is shown */}
      <div className="flex items-center h-12 min-h-12 border-b border-zinc-800/60 flex-shrink-0 px-2 gap-1">
        {/* Desktop-only: collapse / expand chevron */}
        <button
          type="button"
          onClick={onToggle}
          className="hidden md:flex items-center justify-center w-8 h-8 rounded-lg text-zinc-500 hover:text-zinc-200 hover:bg-zinc-800/80 transition-colors flex-shrink-0"
          aria-label={open ? "Collapse sidebar" : "Expand sidebar"}
          title={open ? "Collapse" : "My jobs"}
        >
          {open ? (
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M15 18l-6-6 6-6" />
            </svg>
          ) : (
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M9 18l6-6-6-6" />
            </svg>
          )}
        </button>

        {/* Label: always visible on mobile (sidebar only mounts when open), only when open on desktop */}
        <span className={`text-sm font-semibold text-zinc-300 flex-1 truncate ml-1 ${!open ? "md:hidden" : ""}`}>
          My jobs
        </span>
      </div>

      {/* Job list — only rendered when the panel is open */}
      {open && (
        <div className="flex-1 overflow-y-auto min-h-0">
          {loading ? (
            <div className="px-4 py-4 text-zinc-500 text-xs">Loading…</div>
          ) : error ? (
            <div className="px-4 py-4 text-red-400 text-xs">{error}</div>
          ) : jobs.length === 0 ? (
            <div className="px-4 py-4 text-zinc-500 text-xs">No jobs yet.</div>
          ) : (
            <ul className="py-1">
              {jobs.map((job) => (
                <li key={job.id}>
                  <Link
                    href={`/jobs/${job.id}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="group flex flex-col px-3 py-2.5 border-l-2 border-transparent hover:border-cyan-500/60 hover:bg-zinc-800/50 transition-colors"
                  >
                    <span
                      className="text-sm font-medium text-zinc-300 group-hover:text-zinc-100 truncate transition-colors"
                      title={job.title}
                    >
                      {job.title || "Untitled"}
                    </span>
                    <span className="text-xs text-zinc-600 mt-0.5">
                      {formatDate(job.createdAt)}
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </aside>
  );
}

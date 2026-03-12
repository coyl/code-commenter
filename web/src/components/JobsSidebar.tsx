"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import { fetchApiAdapter } from "@/adapters/api";
import type { JobMeta } from "@/domain/api";

const SIDEBAR_WIDTH_OPEN = 280;
const SIDEBAR_WIDTH_COLLAPSED = 48;

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
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: d.getFullYear() !== now.getFullYear() ? "numeric" : undefined });
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

  const fetchJobs = useCallback(async () => {
    if (!signedIn) return;
    setLoading(true);
    setError(null);
    try {
      const list = await api.listMyJobs(50);
      setJobs(list);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load jobs");
      setJobs([]);
    } finally {
      setLoading(false);
    }
  }, [signedIn, api]);

  useEffect(() => {
    if (open && signedIn) fetchJobs();
  }, [open, signedIn, fetchJobs]);

  useEffect(() => {
    if (refreshTrigger && open && signedIn) fetchJobs();
  }, [refreshTrigger, open, signedIn, fetchJobs]);

  if (!signedIn) return null;

  const width = open ? SIDEBAR_WIDTH_OPEN : SIDEBAR_WIDTH_COLLAPSED;

  return (
    <aside
      className="flex-shrink-0 border-r border-zinc-700 bg-zinc-900/80 flex flex-col transition-[width] duration-200 ease-out"
      style={{ width }}
    >
      <div className="flex items-center h-12 min-h-12 border-b border-zinc-700 px-2 flex-shrink-0">
        <button
          type="button"
          onClick={onToggle}
          className="p-2 rounded text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
          aria-label={open ? "Collapse sidebar" : "Expand sidebar"}
          title={open ? "Collapse sidebar" : "My jobs"}
        >
          {open ? (
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M15 18l-6-6 6-6" />
            </svg>
          ) : (
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M9 18l6-6-6-6" />
            </svg>
          )}
        </button>
        {open && (
          <span className="ml-1 text-sm font-medium text-zinc-300 truncate">
            My jobs
          </span>
        )}
      </div>
      {open && (
        <div className="flex-1 overflow-auto min-h-0">
          {loading ? (
            <div className="p-3 text-zinc-500 text-sm">Loading…</div>
          ) : error ? (
            <div className="p-3 text-red-400 text-sm">{error}</div>
          ) : jobs.length === 0 ? (
            <div className="p-3 text-zinc-500 text-sm">No jobs yet.</div>
          ) : (
            <ul className="py-2">
              {jobs.map((job) => (
                <li key={job.id}>
                  <Link
                    href={`/jobs/${job.id}`}
                    className="block px-3 py-2 text-sm text-zinc-300 hover:bg-zinc-800 hover:text-zinc-100 border-l-2 border-transparent hover:border-cyan-500"
                  >
                    <span className="block truncate font-medium" title={job.title}>
                      {job.title || "Untitled"}
                    </span>
                    <span className="block text-xs text-zinc-500 mt-0.5">
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

"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { fetchApiAdapter } from "@/adapters/api";
import type { ApiPort } from "@/ports/api";
import type { JobMeta } from "@/domain/api";

interface Props {
  api?: Pick<ApiPort, "listRecentJobs">;
}

export default function JobCarousel({ api = fetchApiAdapter }: Props) {
  const [jobs, setJobs] = useState<JobMeta[]>([]);

  useEffect(() => {
    api
      .listRecentJobs()
      .then((j) => setJobs(j.filter((x) => x.title && x.title.trim())))
      .catch(() => setJobs([]));
  }, [api]);

  if (jobs.length < 3) return null;

  // Duplicate so the marquee loops seamlessly (-50% animation trick).
  // Multiply copies enough to guarantee the single-copy width overflows any viewport.
  const copies = Math.max(2, Math.ceil(10 / jobs.length) * 2);
  const items = Array.from({ length: copies * jobs.length }, (_, i) => jobs[i % jobs.length]);

  return (
    <div>
      <p className="text-xs font-semibold uppercase tracking-widest text-zinc-600 mb-2.5">
        Recent walkthroughs
      </p>
    <div className="relative overflow-hidden">
      {/* Left edge fade */}
      <div
        className="pointer-events-none absolute inset-y-0 left-0 w-16 z-10"
        style={{ background: "linear-gradient(to right, #09090b, transparent)" }}
        aria-hidden
      />
      {/* Right edge fade */}
      <div
        className="pointer-events-none absolute inset-y-0 right-0 w-16 z-10"
        style={{ background: "linear-gradient(to left, #09090b, transparent)" }}
        aria-hidden
      />

      <div className="marquee-track gap-2.5" aria-label="Recent walkthroughs">
        {items.map((job, i) => (
          <Link
            key={`${job.id}-${i}`}
            href={`/jobs/${job.id}`}
            target="_blank"
            rel="noopener noreferrer"
            className="group flex-shrink-0 flex items-center gap-2 px-3.5 py-2 rounded-lg bg-zinc-900/80 border border-zinc-800/70 hover:border-cyan-500/30 hover:bg-zinc-800/60 transition-colors max-w-[220px] shadow-sm"
            tabIndex={-1}
          >
            {/* Small code icon */}
            <svg
              width="11"
              height="11"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="text-zinc-600 group-hover:text-cyan-500/70 flex-shrink-0 transition-colors"
              aria-hidden
            >
              <polyline points="16 18 22 12 16 6" />
              <polyline points="8 6 2 12 8 18" />
            </svg>
            <span className="text-xs text-zinc-400 group-hover:text-zinc-200 truncate transition-colors leading-none">
              {job.title}
            </span>
          </Link>
        ))}
      </div>
    </div>
    </div>
  );
}

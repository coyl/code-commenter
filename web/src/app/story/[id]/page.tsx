"use client";

import React, { useMemo } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import DOMPurify from "dompurify";
import { useJob } from "@/features/job/useJob";

const EMBED_PLAYER_MARKER = "{{EMBED_PLAYER}}";

/** Tags the backend LLM is instructed to use; no script, no event handlers. */
const STORY_ALLOWED_TAGS = ["h1", "h2", "p", "ul", "li", "strong", "em", "code"];

/** Sanitize LLM-generated story HTML to prevent XSS (prompt injection → script/onerror etc.). */
function sanitizeStoryHtml(html: string): string {
  if (typeof window === "undefined") return html;
  return DOMPurify.sanitize(html, { ALLOWED_TAGS: STORY_ALLOWED_TAGS });
}

/** Returns the storyHtml with the marker replaced by an iframe pointing to /embed/{id}. */
function injectEmbed(storyHtml: string, id: string): string {
  const iframe = `<div class="story-embed-container"><iframe src="/embed/${encodeURIComponent(id)}" title="Interactive code player" allow="autoplay; clipboard-write" loading="lazy" style="width:100%;height:540px;border:0;border-radius:12px;display:block;"></iframe></div>`;
  return storyHtml.replace(EMBED_PLAYER_MARKER, iframe);
}

export default function StoryPage() {
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : null;
  const { job, loading, error } = useJob(id);

  const finalHtml = useMemo(() => {
    if (!job?.storyHtml || !id) return null;
    const sanitized = sanitizeStoryHtml(job.storyHtml);
    return injectEmbed(sanitized, id);
  }, [job?.storyHtml, id]);

  if (loading) {
    return (
      <main className="min-h-screen bg-zinc-950 px-4 py-10 max-w-3xl mx-auto">
        <div className="flex items-center gap-3 text-zinc-500 text-sm">
          <span className="w-4 h-4 rounded-full border-2 border-zinc-600 border-t-cyan-500 animate-spin" />
          Loading story…
        </div>
      </main>
    );
  }

  if (error || !job) {
    return (
      <main className="min-h-screen bg-zinc-950 px-4 py-10 max-w-3xl mx-auto">
        <div className="mb-5 px-4 py-3 rounded-xl bg-red-950/40 border border-red-800/50 text-red-300 text-sm">
          {error || "Story not found"}
        </div>
        <Link href="/" className="text-cyan-500/80 hover:text-cyan-400 text-sm transition-colors">
          ← Back to generator
        </Link>
      </main>
    );
  }

  if (!finalHtml) {
    return (
      <main className="min-h-screen bg-zinc-950 px-4 py-10 max-w-3xl mx-auto">
        <div className="mb-5 px-4 py-3 rounded-xl bg-zinc-900/60 border border-zinc-800 text-zinc-400 text-sm">
          No story available for this job yet.
        </div>
        {id && (
          <Link href={`/jobs/${id}`} className="text-cyan-500/80 hover:text-cyan-400 text-sm transition-colors">
            ← Back to job
          </Link>
        )}
      </main>
    );
  }

  const title = job.title || job.prompt || "Code story";

  return (
    <main className="min-h-screen bg-zinc-950 text-zinc-100">
      <div className="max-w-3xl mx-auto px-4 sm:px-6 py-10 md:py-14">
        {/* Breadcrumb nav */}
        <nav className="mb-8 flex items-center gap-2 text-sm" aria-label="Breadcrumb">
          <Link href="/" className="text-zinc-600 hover:text-zinc-300 transition-colors">
            Generator
          </Link>
          <span className="text-zinc-700" aria-hidden>/</span>
          {id && (
            <>
              <Link href={`/jobs/${id}`} className="text-zinc-600 hover:text-zinc-300 transition-colors">
                Job
              </Link>
              <span className="text-zinc-700" aria-hidden>/</span>
            </>
          )}
          <span className="text-zinc-400">Story</span>
        </nav>

        <h1 className="text-2xl md:text-3xl font-bold text-white mb-10 leading-snug tracking-tight">
          {title}
        </h1>

        {job.illustrationImageBase64 && (
          /* eslint-disable-next-line @next/next/no-img-element */
          <img
            src={`data:image/png;base64,${job.illustrationImageBase64}`}
            alt="Article illustration"
            className="rounded-xl border border-zinc-800/70 w-full mb-10"
            width={640}
            height={480}
          />
        )}

        <article
          className="prose-story"
          dangerouslySetInnerHTML={{ __html: finalHtml }}
        />
      </div>

      <style>{`
        .prose-story h1,
        .prose-story h2 {
          color: #f4f4f5;
          font-weight: 700;
          line-height: 1.3;
          letter-spacing: -0.01em;
          margin: 2.25rem 0 0.875rem;
        }
        .prose-story h1 { font-size: 1.5rem; }
        .prose-story h2 { font-size: 1.2rem; color: #e4e4e7; }
        .prose-story p {
          color: #a1a1aa;
          line-height: 1.8;
          margin: 0 0 1.375rem;
          font-size: 1rem;
        }
        .prose-story ul,
        .prose-story ol {
          color: #a1a1aa;
          line-height: 1.8;
          margin: 0 0 1.375rem;
          padding-left: 1.5rem;
        }
        .prose-story li { margin-bottom: 0.375rem; }
        .prose-story strong { color: #e4e4e7; font-weight: 600; }
        .prose-story em { color: #d4d4d8; }
        .prose-story code {
          background: #27272a;
          border: 1px solid #3f3f46;
          border-radius: 5px;
          padding: 0.15em 0.45em;
          font-size: 0.85em;
          color: #67e8f9;
          font-family: var(--font-mono, ui-monospace, monospace);
        }
        .story-embed-container {
          margin: 3rem 0;
          border-radius: 14px;
          overflow: hidden;
          border: 1px solid #3f3f46;
          box-shadow: 0 12px 48px rgba(0,0,0,0.5), 0 0 0 1px rgba(255,255,255,0.03);
        }
      `}</style>
    </main>
  );
}

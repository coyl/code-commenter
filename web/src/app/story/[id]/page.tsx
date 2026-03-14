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
  const iframe = `<div class="story-embed-container"><iframe src="/embed/${encodeURIComponent(id)}" title="Interactive code player" allow="autoplay; clipboard-write" loading="lazy" style="width:100%;height:560px;border:0;border-radius:8px;display:block;"></iframe></div>`;
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
      <main className="min-h-screen p-6 max-w-3xl mx-auto">
        <p className="text-zinc-400">Loading story…</p>
      </main>
    );
  }

  if (error || !job) {
    return (
      <main className="min-h-screen p-6 max-w-3xl mx-auto">
        <div className="mb-4 p-3 rounded bg-red-900/30 border border-red-700 text-red-200 text-sm">
          {error || "Story not found"}
        </div>
        <Link href="/" className="text-cyan-400 hover:underline text-sm">
          ← Back to generator
        </Link>
      </main>
    );
  }

  if (!finalHtml) {
    return (
      <main className="min-h-screen p-6 max-w-3xl mx-auto">
        <div className="mb-4 p-3 rounded bg-zinc-800 border border-zinc-700 text-zinc-300 text-sm">
          No story available for this job yet.
        </div>
        {id && (
          <Link href={`/jobs/${id}`} className="text-cyan-400 hover:underline text-sm">
            ← Back to job
          </Link>
        )}
      </main>
    );
  }

  const title = job.title || job.prompt || "Code story";

  return (
    <main className="min-h-screen bg-zinc-950 text-zinc-100">
      <div className="max-w-3xl mx-auto px-4 py-12">
        <nav className="mb-8 flex items-center gap-4 text-sm">
          <Link href="/" className="text-zinc-500 hover:text-zinc-300 transition-colors">
            ← Generator
          </Link>
          {id && (
            <>
              <span className="text-zinc-700">/</span>
              <Link href={`/jobs/${id}`} className="text-zinc-500 hover:text-zinc-300 transition-colors">
                Job
              </Link>
            </>
          )}
          <span className="text-zinc-700">/</span>
          <span className="text-zinc-400">Story</span>
        </nav>

        <h1 className="text-2xl font-bold text-white mb-8 leading-snug">{title}</h1>

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
          margin: 2rem 0 0.75rem;
        }
        .prose-story h1 { font-size: 1.5rem; }
        .prose-story h2 { font-size: 1.25rem; }
        .prose-story p {
          color: #a1a1aa;
          line-height: 1.75;
          margin: 0 0 1.25rem;
        }
        .prose-story ul,
        .prose-story ol {
          color: #a1a1aa;
          line-height: 1.75;
          margin: 0 0 1.25rem;
          padding-left: 1.5rem;
        }
        .prose-story li { margin-bottom: 0.25rem; }
        .prose-story strong { color: #e4e4e7; font-weight: 600; }
        .prose-story em { color: #d4d4d8; }
        .prose-story code {
          background: #27272a;
          border: 1px solid #3f3f46;
          border-radius: 4px;
          padding: 0.15em 0.4em;
          font-size: 0.875em;
          color: #67e8f9;
          font-family: ui-monospace, monospace;
        }
        .story-embed-container {
          margin: 2.5rem 0;
          border-radius: 12px;
          overflow: hidden;
          border: 1px solid #3f3f46;
          box-shadow: 0 8px 32px rgba(0,0,0,0.4);
        }
      `}</style>
    </main>
  );
}

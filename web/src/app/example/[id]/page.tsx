"use client";

import React from "react";
import { useParams } from "next/navigation";

/**
 * Example page: light-themed sample article with embedded player.
 * Iframe has no scrollbars (overflow hidden).
 */

const EMBED_HEIGHT_PX = 560;
const EMBED_MAX_WIDTH_PX = 896;

export default function ExamplePage() {
  const params = useParams();
  const id = typeof params.id === "string" ? params.id : null;

  if (!id) {
    return (
      <main className="min-h-screen flex items-center justify-center p-6 bg-stone-50">
        <p className="text-stone-600 text-sm">Missing job id. Use /example/[job-uuid].</p>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-stone-50 text-stone-800">
      <article className="max-w-3xl mx-auto px-4 py-10">
        <h1 className="text-2xl font-semibold text-stone-900 mb-3">
          Understanding the event loop
        </h1>
        <p className="text-stone-600 leading-relaxed mb-6">
          JavaScript runs on a single thread, but it can still handle async work thanks to the
          event loop. In this walkthrough we build a minimal example that schedules callbacks with{" "}
          <code className="px-1.5 py-0.5 rounded bg-stone-200 text-stone-800 text-sm font-mono">
            setTimeout
          </code>{" "}
          and processes them in order. The code below is narrated step by step—press play to hear
          the explanation.
        </p>

        <div
          className="w-full rounded-lg overflow-hidden border border-stone-200 bg-stone-100 shadow-sm mb-6"
          style={{ maxWidth: EMBED_MAX_WIDTH_PX, height: EMBED_HEIGHT_PX }}
        >
          <iframe
            src={`/embed/${encodeURIComponent(id)}`}
            title="Code Commenter player"
            className="w-full h-full border-0 block overflow-hidden"
            style={{ overflow: "hidden" }}
            allow="autoplay"
            scrolling="no"
          />
        </div>

        <p className="text-stone-600 leading-relaxed">
          Once you’ve seen how the event loop queues and runs callbacks, try modifying the delays
          or adding a <code className="px-1.5 py-0.5 rounded bg-stone-200 text-stone-800 text-sm font-mono">Promise</code>-
          based example. The same pattern applies to I/O and timers in Node.js and the browser.
        </p>
      </article>
    </main>
  );
}

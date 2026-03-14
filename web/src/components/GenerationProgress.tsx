"use client";

import React, { useMemo, useState, useEffect } from "react";

/** Stage title (normalized) -> progress percentage. CSS step is not shown. */
const STAGE_PERCENT: Record<string, number> = {
  "Preparing your code": 25,
  "Generating task spec": 25,
  "Generating code segments": 50,
  "Generating voiceover": 75,
  "Generating story": 90,
  "Finalizing": 100,
};

const CSS_STAGE = "Generating CSS";

function stageToPercent(stage: string): number {
  const normalized = stage.replace(/\s*…?\s*$/, "").trim();
  if (!normalized || normalized === CSS_STAGE) return -1;
  const percent = STAGE_PERCENT[normalized];
  if (percent !== undefined) return percent;
  const byLower = Object.entries(STAGE_PERCENT).find(
    ([key]) => key.toLowerCase() === normalized.toLowerCase()
  );
  return byLower ? byLower[1] : -1;
}

type Props = {
  stage: string;
};

export default function GenerationProgress({ stage }: Props) {
  const [displayed, setDisplayed] = useState({ label: "Generating", percent: 0 });

  const trimmed = useMemo(
    () => stage.replace(/\s*…?\s*$/, "").trim() || "Generating",
    [stage]
  );
  const pct = useMemo(() => stageToPercent(stage), [stage]);

  useEffect(() => {
    if (!stage.trim()) {
      setDisplayed({ label: "Generating", percent: 0 });
      return;
    }
    if (trimmed !== CSS_STAGE && pct >= 0) {
      setDisplayed({ label: trimmed, percent: Math.min(100, pct) });
    }
  }, [stage, trimmed, pct]);

  const { label, percent } = displayed;

  return (
    <div
      className="mb-6 rounded-lg border border-cyan-800/60 bg-cyan-950/30 px-4 py-3"
      role="status"
      aria-live="polite"
      aria-valuenow={percent}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={`${label} — ${percent}%`}
    >
      <div className="flex items-center gap-3">
        <div
          className="h-2 flex-1 min-w-0 rounded-full bg-zinc-800 overflow-hidden"
          aria-hidden
        >
          <div
            className="h-full rounded-full bg-cyan-500 transition-[width] duration-300 ease-out"
            style={{ width: `${percent}%` }}
          />
        </div>
        <span className="text-sm font-medium text-cyan-200/90 whitespace-nowrap tabular-nums">
          {label}
          <span className="text-zinc-500 font-normal ml-1">({percent}%)</span>
        </span>
      </div>
    </div>
  );
}

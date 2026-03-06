"use client";

import { useCallback, useState, useEffect } from "react";
import { fetchApiAdapter } from "@/adapters/api";
import type { JobResponse } from "@/domain/api";

export function useJob(jobId: string | null, api = fetchApiAdapter) {
  const [job, setJob] = useState<JobResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadJob = useCallback(
    async (id: string) => {
      setLoading(true);
      setError(null);
      try {
        const data = await api.getJob(id);
        setJob(data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
        setJob(null);
      } finally {
        setLoading(false);
      }
    },
    [api]
  );

  useEffect(() => {
    if (!jobId) {
      setLoading(false);
      setError("Missing job id");
      setJob(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    api
      .getJob(jobId)
      .then((data) => {
        if (!cancelled) setJob(data);
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
  }, [jobId, api]);

  return { job, loading, error, reload: loadJob };
}

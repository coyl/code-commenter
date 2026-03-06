"use client";

import { useCallback, useState } from "react";
import { fetchApiAdapter } from "@/adapters/api";

export function useTask(api = fetchApiAdapter) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runTask = useCallback(
    async (task: string, language: string) => {
      setError(null);
      setLoading(true);
      try {
        const data = await api.postTask(task, language);
        return data;
      } catch (e) {
        const msg = e instanceof Error ? e.message : "Request failed";
        setError(msg);
        throw e;
      } finally {
        setLoading(false);
      }
    },
    [api]
  );

  const clearError = useCallback(() => setError(null), []);

  return { runTask, loading, error, clearError };
}

"use client";

import { useCallback, useState } from "react";
import { fetchApiAdapter } from "@/adapters/api";
import type { ChangeResponse } from "@/domain/api";

export function useChange(api = fetchApiAdapter) {
  const [changing, setChanging] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const applyChange = useCallback(
    async (sessionId: string, message: string): Promise<ChangeResponse | null> => {
      setError(null);
      setChanging(true);
      try {
        const data = await api.postChange(sessionId, message);
        return data;
      } catch (e) {
        const msg = e instanceof Error ? e.message : "Change failed";
        setError(msg);
        return null;
      } finally {
        setChanging(false);
      }
    },
    [api]
  );

  return { applyChange, changing, error };
}

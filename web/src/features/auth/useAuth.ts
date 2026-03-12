"use client";

import { useState, useEffect, useCallback } from "react";
import { getApiBaseAsync } from "@/config";
import { fetchApiAdapter } from "@/adapters/api";
import type { UserInfo } from "@/domain/api";

export type AuthState = {
  user: UserInfo | null;
  loading: boolean;
  signInUrl: string | null;
  signOutUrl: string | null;
  refetch: () => Promise<void>;
};

export function useAuth(api = fetchApiAdapter): AuthState {
  const [user, setUser] = useState<UserInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [signInUrl, setSignInUrl] = useState<string | null>(null);
  const [signOutUrl, setSignOutUrl] = useState<string | null>(null);

  const refetch = useCallback(async () => {
    try {
      const base = await getApiBaseAsync();
      const redirect = typeof window !== "undefined" ? window.location.origin + window.location.pathname : "";
      setSignInUrl(`${base}/auth/start?redirect=${encodeURIComponent(redirect)}`);
      setSignOutUrl(`${base}/auth/logout?redirect=${encodeURIComponent(redirect)}`);
      const me = await api.getMe();
      setUser(me);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    refetch();
  }, [refetch]);

  return { user, loading, signInUrl, signOutUrl, refetch };
}

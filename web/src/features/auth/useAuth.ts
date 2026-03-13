"use client";

import { useState, useEffect, useCallback } from "react";
import { getApiBaseAsync } from "@/config";
import { fetchApiAdapter } from "@/adapters/api";
import type { UserInfo } from "@/domain/api";

export type AuthState = {
  user: UserInfo | null;
  loading: boolean;
  /** When false, API has no OAuth; allow unauthenticated use and do not show sign-in or jobs list. */
  authConfigured: boolean;
  signInUrl: string | null;
  signOutUrl: string | null;
  /** Remaining daily generations. Undefined when quota is not configured. */
  quotaRemaining: number | undefined;
  refetch: () => Promise<void>;
};

export function useAuth(api = fetchApiAdapter): AuthState {
  const [user, setUser] = useState<UserInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [authConfigured, setAuthConfigured] = useState(false);
  const [signInUrl, setSignInUrl] = useState<string | null>(null);
  const [signOutUrl, setSignOutUrl] = useState<string | null>(null);
  const [quotaRemaining, setQuotaRemaining] = useState<number | undefined>(undefined);

  const refetch = useCallback(async () => {
    try {
      const result = await api.getMe();
      setUser(result.user);
      setAuthConfigured(result.authConfigured);
      setQuotaRemaining(result.user?.quotaRemaining);
      if (result.authConfigured) {
        const base = await getApiBaseAsync();
        const redirect = typeof window !== "undefined" ? window.location.origin + window.location.pathname : "";
        setSignInUrl(`${base}/auth/start?redirect=${encodeURIComponent(redirect)}`);
        setSignOutUrl(`${base}/auth/logout?redirect=${encodeURIComponent(redirect)}`);
      } else {
        setSignInUrl(null);
        setSignOutUrl(null);
      }
    } catch {
      setUser(null);
      setAuthConfigured(false);
      setSignInUrl(null);
      setSignOutUrl(null);
      setQuotaRemaining(undefined);
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => {
    refetch();
  }, [refetch]);

  return { user, loading, authConfigured, signInUrl, signOutUrl, quotaRemaining, refetch };
}

"use client";

import { useEffect, Suspense } from "react";
import { setSessionToken } from "@/lib/session-token";

/**
 * Only allow relative paths. Reject absolute URLs (https:, //, javascript:) to prevent
 * open redirects when the fragment is client-controlled.
 */
function safeRedirectPath(value: string | null): string {
  if (value == null || value === "") return "/";
  const s = value.trim();
  if (s === "" || s.startsWith("//") || /^[a-z][a-z0-9+.-]*:/i.test(s)) return "/";
  if (s.startsWith("/")) return s;
  return "/";
}

/**
 * OAuth callback landing page.
 * The API redirects here after a successful Google sign-in with the session token
 * in the URL fragment: /auth/callback#token=TOKEN&redirect=/path
 * We store the token in localStorage (works even when third-party cookies are blocked)
 * and navigate to the original redirect destination.
 */
function AuthCallbackInner() {
  useEffect(() => {
    const hash = window.location.hash.substring(1);
    const params = new URLSearchParams(hash);
    const token = params.get("token");
    const redirect = safeRedirectPath(params.get("redirect"));

    if (token) {
      setSessionToken(token);
    }

    window.location.replace(redirect);
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center bg-zinc-950 text-zinc-400">
      <p className="text-sm">Completing sign-in…</p>
    </div>
  );
}

export default function AuthCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center bg-zinc-950 text-zinc-400">
          <p className="text-sm">Loading…</p>
        </div>
      }
    >
      <AuthCallbackInner />
    </Suspense>
  );
}

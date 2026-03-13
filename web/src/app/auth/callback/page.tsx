"use client";

import { useSearchParams } from "next/navigation";
import { useRouter } from "next/navigation";
import { useEffect, useState, Suspense } from "react";
import { getApiBaseAsync } from "@/config";

/**
 * OAuth callback page: Google redirects here (frontend URL).
 * We forward code and state to the backend so it can exchange the code and set the session cookie.
 * Configure AUTH_CALLBACK_URL in the API to this page (e.g. http://localhost:3010/auth/callback).
 */
function AuthCallbackInner() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const code = searchParams.get("code");
    const state = searchParams.get("state");

    if (!code) {
      setError("Missing authorization code");
      return;
    }

    let cancelled = false;
    getApiBaseAsync()
      .then((base) => {
        if (cancelled) return;
        const url = new URL("/auth/callback", base);
        url.searchParams.set("code", code);
        if (state) url.searchParams.set("state", state);
        window.location.href = url.toString();
      })
      .catch(() => {
        if (!cancelled) setError("Failed to load API config");
      });

    return () => {
      cancelled = true;
    };
  }, [searchParams]);

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-zinc-950 text-zinc-300 p-4">
        <div className="text-center">
          <p className="text-red-400 mb-4">{error}</p>
          <button
            type="button"
            onClick={() => router.push("/")}
            className="px-4 py-2 rounded-lg bg-zinc-700 hover:bg-zinc-600 text-sm"
          >
            Back to app
          </button>
        </div>
      </div>
    );
  }

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

const BUILD_TIME_DEFAULT =
  (typeof process !== "undefined" && process.env?.NEXT_PUBLIC_API_URL) || "http://localhost:8080";

let cachedApiUrl: string | null = null;
let configPromise: Promise<void> | null = null;

/** Load config from /config.json (browser only). Safe to call multiple times; resolves once. */
export function loadConfig(): Promise<void> {
  if (typeof window === "undefined") return Promise.resolve();
  if (cachedApiUrl !== null) return Promise.resolve();
  if (configPromise) return configPromise;
  configPromise = fetch("/config.json")
    .then((res) => (res.ok ? res.json() : Promise.reject(new Error("config.json not found"))))
    .then((data: { apiUrl?: string }) => {
      const url = typeof data?.apiUrl === "string" && data.apiUrl.trim() ? data.apiUrl.trim() : null;
      cachedApiUrl = url ?? BUILD_TIME_DEFAULT;
    })
    .catch(() => {
      cachedApiUrl = BUILD_TIME_DEFAULT;
    });
  return configPromise;
}

/** API base URL. Sync: returns cached or build-time default (SSR or before load). */
export function getApiBase(): string {
  return cachedApiUrl ?? BUILD_TIME_DEFAULT;
}

/** API base URL, waiting for runtime config if in browser. Use before first fetch/WS. */
export async function getApiBaseAsync(): Promise<string> {
  await loadConfig();
  return getApiBase();
}

/** WebSocket origin (ws/wss) derived from API base. Sync. */
export function getWsBase(): string {
  if (typeof window === "undefined") return "";
  const base = getApiBase();
  if (!base) return "";
  try {
    const u = new URL(base);
    u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
    return u.origin;
  } catch {
    return "";
  }
}

/** WebSocket origin, waiting for runtime config if in browser. */
export async function getWsBaseAsync(): Promise<string> {
  await loadConfig();
  return getWsBase();
}

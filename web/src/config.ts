const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export function getApiBase(): string {
  return API_BASE;
}

export function getWsBase(): string {
  if (typeof window === "undefined") return "";
  const u = new URL(API_BASE);
  u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
  return u.origin;
}

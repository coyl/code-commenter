import { getApiBaseAsync } from "@/config";
import type { ApiPort } from "@/ports/api";
import type { JobResponse, UserInfo, JobMeta } from "@/domain/api";

async function parseJson<T>(res: Response): Promise<T> {
  const text = await res.text();
  if (!res.ok) throw new Error(text || res.statusText);
  return text ? (JSON.parse(text) as T) : ({} as T);
}

const fetchOpts = { credentials: "include" as RequestCredentials };

export const fetchApiAdapter: ApiPort = {
  async getJob(jobId: string): Promise<JobResponse> {
    const base = await getApiBaseAsync();
    const res = await fetch(`${base}/jobs/${jobId}`);
    if (res.status === 404) throw new Error("Job not found");
    return parseJson<JobResponse>(res);
  },
  async getMe(): Promise<UserInfo | null> {
    const base = await getApiBaseAsync();
    const res = await fetch(`${base}/me`, { ...fetchOpts });
    if (res.status === 401 || res.status === 404) return null;
    if (!res.ok) return null;
    return parseJson<UserInfo>(res);
  },
  async listMyJobs(limit = 50): Promise<JobMeta[]> {
    const base = await getApiBaseAsync();
    const url = new URL(`${base}/jobs/mine`);
    url.searchParams.set("limit", String(limit));
    const res = await fetch(url.toString(), { ...fetchOpts });
    if (res.status === 401) return [];
    if (!res.ok) throw new Error("Failed to list jobs");
    return parseJson<JobMeta[]>(res);
  },
};

import { getApiBaseAsync } from "@/config";
import type { ApiPort } from "@/ports/api";
import type { JobResponse } from "@/domain/api";

async function parseJson<T>(res: Response): Promise<T> {
  const text = await res.text();
  if (!res.ok) throw new Error(text || res.statusText);
  return text ? (JSON.parse(text) as T) : ({} as T);
}

export const fetchApiAdapter: ApiPort = {
  async getJob(jobId: string): Promise<JobResponse> {
    const base = await getApiBaseAsync();
    const res = await fetch(`${base}/jobs/${jobId}`);
    if (res.status === 404) throw new Error("Job not found");
    return parseJson<JobResponse>(res);
  },
};

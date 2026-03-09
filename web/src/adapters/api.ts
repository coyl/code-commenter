import { getApiBase } from "@/config";
import type { ApiPort } from "@/ports/api";
import type { TaskResponse, ChangeResponse, JobResponse } from "@/domain/api";

function getBase(): string {
  return getApiBase();
}

async function parseJson<T>(res: Response): Promise<T> {
  const text = await res.text();
  if (!res.ok) throw new Error(text || res.statusText);
  return text ? (JSON.parse(text) as T) : ({} as T);
}

export const fetchApiAdapter: ApiPort = {
  async postTask(task: string, language: string, narrationLanguage = "english"): Promise<TaskResponse> {
    const res = await fetch(`${getBase()}/task`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ task, language, narration_language: narrationLanguage }),
    });
    return parseJson<TaskResponse>(res);
  },

  async postChange(sessionId: string, message: string): Promise<ChangeResponse> {
    const res = await fetch(`${getBase()}/task/${sessionId}/change`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ message }),
    });
    return parseJson<ChangeResponse>(res);
  },

  async getJob(jobId: string): Promise<JobResponse> {
    const res = await fetch(`${getBase()}/jobs/${jobId}`);
    if (res.status === 404) throw new Error("Job not found");
    return parseJson<JobResponse>(res);
  },
};

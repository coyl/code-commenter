import type { TaskResponse, JobResponse } from "@/domain/api";

export type ApiPort = {
  postTask(task: string, language: string, narrationLanguage?: string): Promise<TaskResponse>;
  getJob(jobId: string): Promise<JobResponse>;
};

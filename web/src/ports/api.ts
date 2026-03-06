import type { TaskResponse, ChangeResponse, JobResponse } from "@/domain/api";

export type ApiPort = {
  postTask(task: string, language: string): Promise<TaskResponse>;
  postChange(sessionId: string, message: string): Promise<ChangeResponse>;
  getJob(jobId: string): Promise<JobResponse>;
};

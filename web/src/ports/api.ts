import type { JobResponse } from "@/domain/api";

export type ApiPort = {
  getJob(jobId: string): Promise<JobResponse>;
};

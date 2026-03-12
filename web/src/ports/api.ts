import type { JobResponse, UserInfo, JobMeta } from "@/domain/api";

export type ApiPort = {
  getJob(jobId: string): Promise<JobResponse>;
  /** Returns current user or null if not signed in / auth disabled. */
  getMe(): Promise<UserInfo | null>;
  listMyJobs(limit?: number): Promise<JobMeta[]>;
};

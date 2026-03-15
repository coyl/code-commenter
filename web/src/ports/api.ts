import type { JobResponse, UserInfo, JobMeta } from "@/domain/api";

export type GetMeResult = {
  user: UserInfo | null;
  /** False when API has no OAuth (e.g. no /me route); then no sign-in or jobs list. */
  authConfigured: boolean;
};

export type ApiPort = {
  getJob(jobId: string): Promise<JobResponse>;
  /** Returns user and whether auth is configured. When authConfigured is false, allow unauthenticated use and hide jobs list. */
  getMe(): Promise<GetMeResult>;
  listMyJobs(limit?: number): Promise<JobMeta[]>;
  /** Returns the most recently created public jobs (title + id). No auth required. */
  listRecentJobs(): Promise<JobMeta[]>;
};

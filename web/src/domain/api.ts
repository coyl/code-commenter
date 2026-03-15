/**
 * Current user from GET /me (when auth is enabled).
 */
export type UserInfo = {
  sub: string;
  email: string;
  /** Remaining daily generations. Undefined when quota is not configured on the backend. */
  quotaRemaining?: number;
};

/**
 * Job list item from GET /jobs/mine.
 */
export type JobMeta = {
  id: string;
  title: string;
  createdAt: number;
};

/**
 * API response types (REST). Single source of truth for /jobs/:id.
 */
export type JobSegmentStored = {
  code: string;
  codePlain: string;
  narration: string;
  audioChunks: string[];
};

export type JobResponse = {
  prompt: string;
  rawJson: string;
  fullCode: string;
  css?: string;
  title?: string;
  narrationLang?: string;
  storyHtml?: string;
  previewImageBase64?: string;
  illustrationImageBase64?: string;
  segments: JobSegmentStored[];
};

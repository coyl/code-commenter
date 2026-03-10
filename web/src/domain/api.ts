/**
 * API response types (REST). Single source of truth for /task, /jobs/:id.
 */
export type TaskResponse = {
  id: string;
  css: string;
  code: string;
  spec?: string;
  narration?: string;
  voiceoverUrl?: string;
};

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
  segments: JobSegmentStored[];
};

import { audioDurationSeconds } from "@/lib/audio";

/** Split HTML into atomic chunks: each <span ...>...</span> is one chunk; plain text between spans is per-character. */
export function getHTMLChunks(html: string): string[] {
  const chunks: string[] = [];
  let i = 0;
  while (i < html.length) {
    if (html[i] === "<") {
      const spanEnd = html.indexOf("</span>", i);
      if (spanEnd !== -1) {
        chunks.push(html.slice(i, spanEnd + 7));
        i = spanEnd + 7;
      } else {
        const tagEnd = html.indexOf(">", i);
        if (tagEnd !== -1) {
          chunks.push(html.slice(i, tagEnd + 1));
          i = tagEnd + 1;
        } else {
          chunks.push(html[i]);
          i++;
        }
      }
    } else {
      chunks.push(html[i]);
      i++;
    }
  }
  return chunks;
}

/**
 * Typing speed so text finishes by 80% of audio length.
 * @param numTypingSteps - number of typing steps (chunks) for this segment
 * @param audioChunks - base64 PCM chunks for this segment
 * @returns ms per typing step so that total typing time = 80% of audio duration
 */
export function typingSpeedFor80Percent(numTypingSteps: number, audioChunks: string[]): number {
  if (numTypingSteps <= 0) return 20;
  const durationSec = audioDurationSeconds(audioChunks);
  if (durationSec <= 0) return 20;
  const targetTypingMs = 0.8 * durationSec * 1000;
  const msPerStep = targetTypingMs / numTypingSteps;
  return Math.max(2, Math.min(200, msPerStep));
}

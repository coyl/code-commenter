import { describe, it, expect } from "vitest";
import { getHTMLChunks, typingSpeedFor80Percent } from "./codePlayer";

describe("getHTMLChunks", () => {
  it("splits span tags as single chunks", () => {
    const html = '<span class="x">a</span>';
    expect(getHTMLChunks(html)).toEqual(['<span class="x">a</span>']);
  });

  it("splits plain text by character", () => {
    expect(getHTMLChunks("ab")).toEqual(["a", "b"]);
  });

  it("mixed HTML and text", () => {
    const html = '<span class="k">const</span> x';
    const chunks = getHTMLChunks(html);
    expect(chunks[0]).toBe('<span class="k">const</span>');
    expect(chunks.slice(1)).toEqual([" ", "x"]);
  });
});

describe("typingSpeedFor80Percent", () => {
  it("returns default when no steps", () => {
    expect(typingSpeedFor80Percent(0, [])).toBe(20);
  });

  it("returns value between 2 and 200 ms", () => {
    // Valid base64 for 1000 int16 samples (2000 bytes) → ~0.04s at 24kHz
    const b64 = "AAAA".repeat(500);
    const ms = typingSpeedFor80Percent(10, [b64]);
    expect(ms).toBeGreaterThanOrEqual(2);
    expect(ms).toBeLessThanOrEqual(200);
  });
});

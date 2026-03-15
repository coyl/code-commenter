import { describe, it, expect } from "vitest";
import { isStreamEvent } from "./stream";

describe("isStreamEvent", () => {
  it("accepts valid event types", () => {
    expect(isStreamEvent({ type: "job_started", id: "x" })).toBe(true);
    expect(isStreamEvent({ type: "spec", narration: "a" })).toBe(true);
    expect(isStreamEvent({ type: "css", css: ".x{}" })).toBe(true);
    expect(isStreamEvent({ type: "segment", index: 0, code: "", codePlain: "", narration: "" })).toBe(true);
    expect(isStreamEvent({ type: "audio", data: "base64" })).toBe(true);
    expect(isStreamEvent({ type: "code_done", code: "", codePlain: "" })).toBe(true);
    expect(isStreamEvent({ type: "stage", stage: "Generating CSS" })).toBe(true);
    expect(isStreamEvent({ type: "session", id: "s1" })).toBe(true);
    expect(isStreamEvent({ type: "story", storyHtml: "<p>hello</p>" })).toBe(true);
    expect(isStreamEvent({ type: "visuals", previewImageBase64: "abc", illustrationImageBase64: "def" })).toBe(true);
    expect(isStreamEvent({ type: "error", error: "msg" })).toBe(true);
  });

  it("rejects non-objects", () => {
    expect(isStreamEvent(null)).toBe(false);
    expect(isStreamEvent(undefined)).toBe(false);
    expect(isStreamEvent("string")).toBe(false);
  });

  it("rejects objects without type", () => {
    expect(isStreamEvent({ id: "x" })).toBe(false);
  });

  it("rejects unknown event types", () => {
    expect(isStreamEvent({ type: "unknown" })).toBe(false);
  });
});

import { describe, it, expect } from "vitest";
import { parseMessage } from "./stream";

describe("stream adapter parseMessage", () => {
  it("parses segment event", () => {
    const raw = JSON.stringify({
      type: "segment",
      index: 1,
      code: "<pre>code</pre>",
      codePlain: "code",
      narration: "n",
    });
    const event = parseMessage(raw);
    expect(event).toEqual({
      type: "segment",
      index: 1,
      code: "<pre>code</pre>",
      codePlain: "code",
      narration: "n",
    });
  });

  it("parses stage event", () => {
    const raw = JSON.stringify({ type: "stage", stage: "Generating code segments" });
    const event = parseMessage(raw);
    expect(event).toEqual({ type: "stage", stage: "Generating code segments" });
  });

  it("parses code_done with rawJson", () => {
    const raw = JSON.stringify({
      type: "code_done",
      code: "full",
      codePlain: "plain",
      rawJson: "[{}]",
    });
    const event = parseMessage(raw);
    expect(event).toMatchObject({ type: "code_done", code: "full", rawJson: "[{}]" });
  });

  it("returns null for invalid JSON", () => {
    expect(parseMessage("not json")).toBeNull();
  });

  it("returns null for unknown type", () => {
    expect(parseMessage(JSON.stringify({ type: "unknown" }))).toBeNull();
  });
});

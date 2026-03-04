import { describe, it, expect } from "vitest";
import { getRenderedText } from "./syntaxHighlight";

describe("syntaxHighlight", () => {
  it("stops tag content at next [[ so [[p]][[v]] parses as p=[ then v=item", () => {
    const raw =
      '[[k]]if[[/k]] [[v]]stock[[/v]] [[o]]<[[/o]] [[v]]amount[[/v]] [[p]]{[[/p]]\n' +
        '        [[v]]fmt[[/v]][[p]].[[/p]][[f]]Printf[[/f]][[p]]([[/p]][[s]]"Error: Insufficient stock for %s\\n"[[/s]][[p]],[[/p]] [[v]]item[[/v]][[p]])[[/p]]\n' +
        '        [[k]]return[[/k]]\n' +
        '    [[p]]}[/p]]\n' +
        '    [[v]]inventory[[/v]][[p]][[[/p]][[v]]item[[/v]][[p]]][[/p]] [[o]]-=[[/o]] [[v]]amount[[/v]]';
    const rendered = getRenderedText(raw);
    // Should not contain literal tag text
    expect(rendered).not.toContain("[/p]]");
    expect(rendered).not.toContain("[[v]]");
    expect(rendered).not.toContain("[[p]]");
    // Should contain the actual code
    expect(rendered).toContain("if stock < amount {");
    expect(rendered).toContain('fmt.Printf("Error: Insufficient stock for %s\\n", item)');
    expect(rendered).toContain("return");
    expect(rendered).toContain("}");
    expect(rendered).toContain("inventory[item] -= amount");
  });

  it("strips orphan malformed closers }[/p]]", () => {
    const raw = "    [[p]]}[/p]]";
    const rendered = getRenderedText(raw);
    expect(rendered).toBe("    }");
  });

  it("parses adjacent [[p]][[v]] without consuming [[v]] as content of p", () => {
    const raw = "[[p]][[[/p]][[v]]item[[/v]][[p]]][[/p]]";
    const rendered = getRenderedText(raw);
    expect(rendered).toBe("[item]");
  });
});

/**
 * Syntax highlight parsing for [[type]]content[[/type]] tags.
 * Used by the code view; getRenderedText is for tests.
 */

// Find end of token: try [[/type]], fallback to [/type]], and never consume "[[" (start of next tag).
export function findTokenEnd(
  code: string,
  contentStart: number,
  type: string
): { end: number; skipLen: number } {
  const close1 = "[[/" + type + "]]";
  const close2 = "[/" + type + "]]";
  // If our closing tag starts at contentStart, treat first "[" as content and skip "[/type]]"
  if (code.slice(contentStart, contentStart + close1.length) === close1)
    return { end: contentStart + 1, skipLen: close1.length - 1 };
  // If one "[" then our closing tag (e.g. [[p]][[[/p]]), treat that "[" as content and skip "[[/type]]"
  if (code[contentStart] === "[" && code.slice(contentStart + 1, contentStart + 1 + close1.length) === close1)
    return { end: contentStart + 1, skipLen: close1.length };
  const idxClose1 = code.indexOf(close1, contentStart);
  const idxClose2 = code.indexOf(close2, contentStart);
  const idxNextOpen = code.indexOf("[[", contentStart);
  const candidates: { pos: number; skipLen: number }[] = [];
  if (idxClose1 !== -1) candidates.push({ pos: idxClose1, skipLen: close1.length });
  if (idxClose2 !== -1) candidates.push({ pos: idxClose2, skipLen: close2.length });
  if (idxNextOpen !== -1) candidates.push({ pos: idxNextOpen, skipLen: 0 });
  if (candidates.length === 0) return { end: -1, skipLen: 0 };
  const best = candidates.reduce((a, b) =>
    a.pos !== b.pos ? (a.pos < b.pos ? a : b) : (a.skipLen >= b.skipLen ? a : b)
  );
  // When ending at "[[" (next open tag) with no skip, include one "[" and back up so next iteration parses it
  if (best.pos === contentStart && best.skipLen === 0)
    return { end: contentStart + 1, skipLen: -1 };
  return { end: best.pos, skipLen: best.skipLen };
}

const VALID_TYPES = new Set([
  "k", "s", "c", "n", "f", "o", "p", "v",
  "keyword", "string", "comment", "number", "function", "operator", "punctuation", "variable",
]);

// Remove orphan malformed closers like )[/p]] or }[/p]] from plain text.
export function stripOrphanClosers(text: string): string {
  let out = "";
  let i = 0;
  while (i < text.length) {
    const start = text.indexOf("[/", i);
    if (start === -1) {
      out += text.slice(i);
      break;
    }
    out += text.slice(i, start);
    const bracketEnd = text.indexOf("]]", start + 2);
    if (bracketEnd === -1) {
      out += text.slice(start);
      break;
    }
    const type = text.slice(start + 2, bracketEnd).trim().toLowerCase();
    if (VALID_TYPES.has(type) || /^[a-z]{1,2}$/.test(type)) {
      i = bracketEnd + 2;
    } else {
      out += text.slice(start, bracketEnd + 2);
      i = bracketEnd + 2;
    }
  }
  return out;
}

/**
 * Returns the plain text that would be shown after parsing (all tag content + stripped plain text).
 * Used by tests to assert correct parsing.
 */
export function getRenderedText(code: string): string {
  const out: string[] = [];
  let i = 0;
  while (i < code.length) {
    const open = code.indexOf("[[", i);
    if (open === -1) {
      if (i < code.length) out.push(stripOrphanClosers(code.slice(i)));
      break;
    }
    if (open > i) out.push(stripOrphanClosers(code.slice(i, open)));
    const typeEnd = code.indexOf("]]", open + 2);
    if (typeEnd === -1) {
      out.push(code.slice(open));
      break;
    }
    const type = code.slice(open + 2, typeEnd).trim().toLowerCase();
    const contentStart = typeEnd + 2;
    const { end: contentEnd, skipLen } = findTokenEnd(code, contentStart, type);
    const content = contentEnd === -1 ? code.slice(contentStart) : code.slice(contentStart, contentEnd);
    out.push(content);
    i = contentEnd === -1 ? code.length : contentEnd + skipLen;
  }
  return out.join("");
}

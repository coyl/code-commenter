// Package highlight produces syntax-highlighted HTML for code.
//
// Primary path: CodeToHTML uses Chroma to highlight plain source code and
// outputs <span class="token-{type}"> to match frontend CSS (token-keyword,
// token-string, token-comment, etc.).
//
// Legacy: TokensToHTML/TokensToPlain and Convert/PlainText support the old
// token-based and [[tag]] markup paths (kept for tests and compatibility).
package highlight

import (
	"html"
	"strconv"
	"strings"
)

// Token is a single syntax element. JSON uses short keys to reduce LLM output tokens.
type Token struct {
	Type string `json:"t"`
	Text string `json:"x"`
}

// Short (single-char) and long type names → CSS class suffix.
var tokenTypeMap = map[string]string{
	"k": "keyword", "s": "string", "c": "comment", "n": "number",
	"f": "function", "o": "operator", "p": "punctuation", "v": "variable",
	"keyword": "keyword", "string": "string", "comment": "comment", "number": "number",
	"function": "function", "operator": "operator", "punctuation": "punctuation", "variable": "variable",
}

// expandBreak returns n newlines where n is parsed from s (defaults to 1).
func expandBreak(s string) string {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		n = 1
	}
	return strings.Repeat("\n", n)
}

// TokensToHTML converts a token slice into highlighted HTML.
// Each token with a known type becomes <span class="token-{type}">escaped text</span>.
// "b" tokens expand x as a newline count. Unknown types are emitted as escaped plain text.
func TokensToHTML(tokens []Token) string {
	var b strings.Builder
	for _, t := range tokens {
		if t.Type == "b" {
			b.WriteString(expandBreak(t.Text))
			continue
		}
		escaped := html.EscapeString(t.Text)
		if cls, ok := tokenTypeMap[t.Type]; ok {
			b.WriteString(`<span class="token-`)
			b.WriteString(cls)
			b.WriteString(`">`)
			b.WriteString(escaped)
			b.WriteString("</span>")
		} else {
			b.WriteString(escaped)
		}
	}
	return b.String()
}

// TokensToPlain extracts just the visible source text from a token slice.
// "b" tokens are expanded to the corresponding number of newlines.
func TokensToPlain(tokens []Token) string {
	var b strings.Builder
	for _, t := range tokens {
		if t.Type == "b" {
			b.WriteString(expandBreak(t.Text))
			continue
		}
		b.WriteString(t.Text)
	}
	return b.String()
}

// ---------- Legacy [[tag]] support (kept for Convert / PlainText) ----------

var tagMap = map[string]string{
	"k": "keyword", "s": "string", "c": "comment", "n": "number",
	"f": "function", "o": "operator", "p": "punctuation", "v": "variable",
	"keyword": "keyword", "string": "string", "comment": "comment", "number": "number",
	"function": "function", "operator": "operator", "punctuation": "punctuation", "variable": "variable",
}

func cssClass(tag string) string {
	t := strings.TrimSpace(strings.ToLower(tag))
	if c, ok := tagMap[t]; ok {
		return "token-" + c
	}
	return ""
}

func isValidType(t string) bool {
	_, ok := tagMap[strings.TrimSpace(strings.ToLower(t))]
	return ok
}

// Convert transforms raw LLM [[tag]] markup into HTML (legacy path).
func Convert(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	i := 0
	for i < len(raw) {
		openIdx := strings.Index(raw[i:], "[[")
		if openIdx == -1 {
			b.WriteString(html.EscapeString(stripOrphanClosers(raw[i:])))
			break
		}
		openIdx += i
		if openIdx > i {
			b.WriteString(html.EscapeString(stripOrphanClosers(raw[i:openIdx])))
		}
		if strings.HasPrefix(raw[openIdx:], "[[/") {
			closeEnd := strings.Index(raw[openIdx+3:], "]]")
			if closeEnd != -1 {
				closerType := raw[openIdx+3 : openIdx+3+closeEnd]
				if isValidType(closerType) {
					i = openIdx + 3 + closeEnd + 2
					continue
				}
			}
			b.WriteString(html.EscapeString(raw[openIdx : openIdx+2]))
			i = openIdx + 2
			continue
		}
		typeEndIdx := strings.Index(raw[openIdx+2:], "]]")
		if typeEndIdx == -1 {
			b.WriteString(html.EscapeString(raw[openIdx:]))
			break
		}
		typeEndIdx += openIdx + 2
		tag := raw[openIdx+2 : typeEndIdx]
		contentStart := typeEndIdx + 2
		cls := cssClass(tag)
		if cls == "" {
			b.WriteString(html.EscapeString(raw[openIdx:contentStart]))
			i = contentStart
			continue
		}
		content, advance := extractContent(raw, contentStart, strings.TrimSpace(strings.ToLower(tag)))
		b.WriteString(`<span class="`)
		b.WriteString(cls)
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(content))
		b.WriteString("</span>")
		i = contentStart + advance
	}
	return b.String()
}

func extractContent(raw string, contentStart int, tagType string) (content string, advance int) {
	close1 := "[[/" + tagType + "]]"
	close2 := "[/" + tagType + "]]"
	pos := contentStart
	var buf strings.Builder
	for pos < len(raw) {
		remaining := raw[pos:]
		if strings.HasPrefix(remaining, close1) {
			return buf.String(), pos - contentStart + len(close1)
		}
		if strings.HasPrefix(remaining, close2) {
			return buf.String(), pos - contentStart + len(close2)
		}
		if strings.HasPrefix(remaining, "[[/") {
			end := strings.Index(remaining[3:], "]]")
			if end != -1 {
				closerType := remaining[3 : 3+end]
				if isValidType(closerType) {
					return buf.String(), pos - contentStart + 3 + end + 2
				}
			}
		}
		if strings.HasPrefix(remaining, "[[") && !strings.HasPrefix(remaining, "[[/") {
			typeEnd := strings.Index(remaining[2:], "]]")
			if typeEnd != -1 {
				potentialTag := remaining[2 : 2+typeEnd]
				if isValidType(potentialTag) {
					return buf.String(), pos - contentStart
				}
			}
		}
		buf.WriteByte(raw[pos])
		pos++
	}
	return buf.String(), pos - contentStart
}

func stripOrphanClosers(text string) string {
	var b strings.Builder
	i := 0
	for i < len(text) {
		idx := strings.Index(text[i:], "[/")
		if idx == -1 {
			b.WriteString(text[i:])
			break
		}
		b.WriteString(text[i : i+idx])
		endBracket := strings.Index(text[i+idx+2:], "]]")
		if endBracket == -1 {
			b.WriteString(text[i+idx:])
			break
		}
		closer := text[i+idx+2 : i+idx+2+endBracket]
		if isValidType(closer) {
			i = i + idx + 2 + endBracket + 2
		} else {
			b.WriteString(text[i+idx : i+idx+2+endBracket+2])
			i = i + idx + 2 + endBracket + 2
		}
	}
	return b.String()
}

// PlainText extracts visible text from raw [[tag]] markup (legacy path).
func PlainText(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	i := 0
	for i < len(raw) {
		openIdx := strings.Index(raw[i:], "[[")
		if openIdx == -1 {
			b.WriteString(stripOrphanClosers(raw[i:]))
			break
		}
		openIdx += i
		if openIdx > i {
			b.WriteString(stripOrphanClosers(raw[i:openIdx]))
		}
		if strings.HasPrefix(raw[openIdx:], "[[/") {
			closeEnd := strings.Index(raw[openIdx+3:], "]]")
			if closeEnd != -1 {
				closerType := raw[openIdx+3 : openIdx+3+closeEnd]
				if isValidType(closerType) {
					i = openIdx + 3 + closeEnd + 2
					continue
				}
			}
			b.WriteString(raw[openIdx : openIdx+2])
			i = openIdx + 2
			continue
		}
		typeEndIdx := strings.Index(raw[openIdx+2:], "]]")
		if typeEndIdx == -1 {
			b.WriteString(raw[openIdx:])
			break
		}
		typeEndIdx += openIdx + 2
		tag := raw[openIdx+2 : typeEndIdx]
		contentStart := typeEndIdx + 2
		cls := cssClass(tag)
		if cls == "" {
			b.WriteString(raw[openIdx:contentStart])
			i = contentStart
			continue
		}
		content, advance := extractContent(raw, contentStart, strings.TrimSpace(strings.ToLower(tag)))
		b.WriteString(content)
		i = contentStart + advance
	}
	return b.String()
}

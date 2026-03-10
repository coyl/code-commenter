package highlight

import (
	"testing"
)

// ==================== Chroma (plain code) tests ====================

func TestCodeToHTML_Go(t *testing.T) {
	code := `package main
func main() {
	x := 42
}
`
	got, err := CodeToHTML(code, "go")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Error("CodeToHTML returned empty")
	}
	if !containsStr(got, "token-keyword") {
		t.Error("expected token-keyword in output")
	}
	if !containsStr(got, "main") {
		t.Error("expected source text in output")
	}
	if containsStr(got, "[[") || containsStr(got, "]]") {
		t.Error("output should not contain bracket markup")
	}
}

func TestCodeToHTML_UnknownLanguage(t *testing.T) {
	code := "hello"
	got, err := CodeToHTML(code, "nonexistent-lexer")
	if err != nil {
		t.Fatal(err)
	}
	// Fallback lexer should still escape and return something
	if got == "" {
		t.Error("expected non-empty output")
	}
}

// TestCodeToHTML_EmptyLanguage verifies that when language is empty, Chroma's Analyse is used
// so that syntax highlighting still produces token spans (e.g. for "Your code" flow).
func TestCodeToHTML_EmptyLanguage(t *testing.T) {
	code := `package main
func main() {
	x := 42
}
`
	got, err := CodeToHTML(code, "")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Error("CodeToHTML with empty language returned empty")
	}
	// Auto-detection should recognize Go and emit token spans
	if !containsStr(got, "token-keyword") {
		t.Error("expected token-keyword when language is empty (auto-detect)")
	}
	if !containsStr(got, "main") {
		t.Error("expected source text in output")
	}
}

// ==================== Token-based (JSON AST) tests ====================

func TestTokensToHTML_Basic(t *testing.T) {
	tokens := []Token{
		{Type: "keyword", Text: "if"},
		{Type: "", Text: " "},
		{Type: "variable", Text: "x"},
		{Type: "", Text: " "},
		{Type: "operator", Text: ">"},
		{Type: "", Text: " "},
		{Type: "number", Text: "0"},
	}
	got := TokensToHTML(tokens)
	want := `<span class="token-keyword">if</span> <span class="token-variable">x</span> <span class="token-operator">&gt;</span> <span class="token-number">0</span>`
	if got != want {
		t.Errorf("TokensToHTML basic:\n  got  %q\n  want %q", got, want)
	}
}

func TestTokensToHTML_HTMLEscaping(t *testing.T) {
	tokens := []Token{
		{Type: "string", Text: `"<script>alert('xss')</script>"`},
	}
	got := TokensToHTML(tokens)
	want := `<span class="token-string">&#34;&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;&#34;</span>`
	if got != want {
		t.Errorf("TokensToHTML escaping:\n  got  %q\n  want %q", got, want)
	}
}

func TestTokensToHTML_UnknownType(t *testing.T) {
	tokens := []Token{
		{Type: "bogus", Text: "hello"},
		{Type: "", Text: " "},
		{Type: "keyword", Text: "world"},
	}
	got := TokensToHTML(tokens)
	want := `hello <span class="token-keyword">world</span>`
	if got != want {
		t.Errorf("TokensToHTML unknown type:\n  got  %q\n  want %q", got, want)
	}
}

func TestTokensToHTML_Empty(t *testing.T) {
	got := TokensToHTML(nil)
	if got != "" {
		t.Errorf("TokensToHTML nil: got %q want empty", got)
	}
	got = TokensToHTML([]Token{})
	if got != "" {
		t.Errorf("TokensToHTML empty: got %q want empty", got)
	}
}

func TestTokensToHTML_FullGoSnippet(t *testing.T) {
	tokens := []Token{
		{Type: "keyword", Text: "func"},
		{Type: "", Text: " "},
		{Type: "punctuation", Text: "("},
		{Type: "variable", Text: "i"},
		{Type: "", Text: " "},
		{Type: "operator", Text: "*"},
		{Type: "variable", Text: "Inventory"},
		{Type: "punctuation", Text: ")"},
		{Type: "", Text: " "},
		{Type: "function", Text: "ProcessOrder"},
		{Type: "punctuation", Text: "("},
		{Type: "variable", Text: "name"},
		{Type: "", Text: " "},
		{Type: "keyword", Text: "string"},
		{Type: "punctuation", Text: ","},
		{Type: "", Text: " "},
		{Type: "variable", Text: "qty"},
		{Type: "", Text: " "},
		{Type: "keyword", Text: "int"},
		{Type: "punctuation", Text: ")"},
		{Type: "", Text: " "},
		{Type: "variable", Text: "error"},
		{Type: "", Text: " "},
		{Type: "punctuation", Text: "{"},
		{Type: "", Text: "\n  "},
		{Type: "variable", Text: "i"},
		{Type: "punctuation", Text: "."},
		{Type: "variable", Text: "mu"},
		{Type: "punctuation", Text: "."},
		{Type: "function", Text: "Lock"},
		{Type: "punctuation", Text: "()"},
		{Type: "", Text: "\n  "},
		{Type: "keyword", Text: "defer"},
		{Type: "", Text: " "},
		{Type: "variable", Text: "i"},
		{Type: "punctuation", Text: "."},
		{Type: "variable", Text: "mu"},
		{Type: "punctuation", Text: "."},
		{Type: "function", Text: "Unlock"},
		{Type: "punctuation", Text: "()"},
		{Type: "", Text: "\n"},
		{Type: "punctuation", Text: "}"},
	}
	plain := TokensToPlain(tokens)
	want := "func (i *Inventory) ProcessOrder(name string, qty int) error {\n  i.mu.Lock()\n  defer i.mu.Unlock()\n}"
	if plain != want {
		t.Errorf("TokensToPlain full snippet:\n  got  %q\n  want %q", plain, want)
	}
	html := TokensToHTML(tokens)
	// Should not contain any raw brackets from the old format
	for _, bad := range []string{"[[", "]]", "[/"} {
		if containsStr(html, bad) {
			t.Errorf("TokensToHTML contains raw bracket %q", bad)
		}
	}
	// Should contain properly escaped token spans
	if !containsStr(html, `<span class="token-keyword">func</span>`) {
		t.Error("missing keyword span")
	}
	if !containsStr(html, `<span class="token-function">ProcessOrder</span>`) {
		t.Error("missing function span")
	}
	if !containsStr(html, `<span class="token-variable">i</span>`) {
		t.Error("missing variable span")
	}
}

func TestTokensToPlain(t *testing.T) {
	tokens := []Token{
		{Type: "keyword", Text: "if"},
		{Type: "", Text: " "},
		{Type: "variable", Text: "x"},
	}
	got := TokensToPlain(tokens)
	if got != "if x" {
		t.Errorf("TokensToPlain: got %q want %q", got, "if x")
	}
}

func TestTokensToHTML_BreakSingle(t *testing.T) {
	tokens := []Token{
		{Type: "p", Text: "}"},
		{Type: "b", Text: "2"},
		{Type: "k", Text: "func"},
	}
	got := TokensToHTML(tokens)
	want := `<span class="token-punctuation">}</span>` + "\n\n" + `<span class="token-keyword">func</span>`
	if got != want {
		t.Errorf("TokensToHTML break:\n  got  %q\n  want %q", got, want)
	}
}

func TestTokensToHTML_BreakDefault(t *testing.T) {
	tokens := []Token{
		{Type: "p", Text: "}"},
		{Type: "b", Text: "1"},
		{Type: "k", Text: "func"},
	}
	got := TokensToHTML(tokens)
	want := `<span class="token-punctuation">}</span>` + "\n" + `<span class="token-keyword">func</span>`
	if got != want {
		t.Errorf("TokensToHTML break(1):\n  got  %q\n  want %q", got, want)
	}
}

func TestTokensToHTML_BreakInvalid(t *testing.T) {
	tokens := []Token{
		{Type: "b", Text: "abc"},
	}
	got := TokensToHTML(tokens)
	if got != "\n" {
		t.Errorf("TokensToHTML break invalid: got %q want %q", got, "\n")
	}
}

func TestTokensToPlain_Break(t *testing.T) {
	tokens := []Token{
		{Type: "k", Text: "return"},
		{Type: "b", Text: "3"},
		{Type: "k", Text: "func"},
	}
	got := TokensToPlain(tokens)
	want := "return\n\n\nfunc"
	if got != want {
		t.Errorf("TokensToPlain break:\n  got  %q\n  want %q", got, want)
	}
}

func TestTokensToHTML_BracketContent(t *testing.T) {
	tokens := []Token{
		{Type: "variable", Text: "products"},
		{Type: "punctuation", Text: "["},
		{Type: "variable", Text: "name"},
		{Type: "punctuation", Text: "]"},
	}
	got := TokensToHTML(tokens)
	want := `<span class="token-variable">products</span><span class="token-punctuation">[</span><span class="token-variable">name</span><span class="token-punctuation">]</span>`
	if got != want {
		t.Errorf("TokensToHTML brackets:\n  got  %q\n  want %q", got, want)
	}
	plain := TokensToPlain(tokens)
	if plain != "products[name]" {
		t.Errorf("TokensToPlain brackets: got %q want %q", plain, "products[name]")
	}
}

func TestTokensToHTML_AllTypes(t *testing.T) {
	// Long form
	types := []string{"keyword", "string", "comment", "number", "function", "operator", "punctuation", "variable"}
	for _, typ := range types {
		tokens := []Token{{Type: typ, Text: "x"}}
		got := TokensToHTML(tokens)
		want := `<span class="token-` + typ + `">x</span>`
		if got != want {
			t.Errorf("TokensToHTML long type %q: got %q want %q", typ, got, want)
		}
	}
	// Short form (what the LLM actually returns)
	short := map[string]string{
		"k": "keyword", "s": "string", "c": "comment", "n": "number",
		"f": "function", "o": "operator", "p": "punctuation", "v": "variable",
	}
	for s, long := range short {
		tokens := []Token{{Type: s, Text: "x"}}
		got := TokensToHTML(tokens)
		want := `<span class="token-` + long + `">x</span>`
		if got != want {
			t.Errorf("TokensToHTML short type %q: got %q want %q", s, got, want)
		}
	}
}

func TestTokensToHTML_ShortForm(t *testing.T) {
	tokens := []Token{
		{Type: "k", Text: "if"},
		{Type: "", Text: " "},
		{Type: "v", Text: "x"},
		{Type: "", Text: " "},
		{Type: "o", Text: ">"},
		{Type: "", Text: " "},
		{Type: "n", Text: "0"},
	}
	got := TokensToHTML(tokens)
	want := `<span class="token-keyword">if</span> <span class="token-variable">x</span> <span class="token-operator">&gt;</span> <span class="token-number">0</span>`
	if got != want {
		t.Errorf("TokensToHTML short form:\n  got  %q\n  want %q", got, want)
	}
}

// ==================== Legacy [[tag]] tests (kept for backward compat) ====================

func TestConvert_SimpleTokens(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "keyword",
			raw:  "[[k]]if[[/k]]",
			want: `<span class="token-keyword">if</span>`,
		},
		{
			name: "variable",
			raw:  "[[v]]x[[/v]]",
			want: `<span class="token-variable">x</span>`,
		},
		{
			name: "long tag name",
			raw:  "[[keyword]]func[[/keyword]]",
			want: `<span class="token-keyword">func</span>`,
		},
		{
			name: "multiple tokens with plain text",
			raw:  "[[k]]if[[/k]] [[v]]x[[/v]] [[o]]>[[/o]] [[n]]0[[/n]]",
			want: `<span class="token-keyword">if</span> <span class="token-variable">x</span> <span class="token-operator">&gt;</span> <span class="token-number">0</span>`,
		},
		{
			name: "plain text only",
			raw:  "hello world",
			want: "hello world",
		},
		{
			name: "empty string",
			raw:  "",
			want: "",
		},
		{
			name: "string with HTML chars",
			raw:  `[[s]]"<script>"[[/s]]`,
			want: `<span class="token-string">&#34;&lt;script&gt;&#34;</span>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Convert(tt.raw)
			if got != tt.want {
				t.Errorf("Convert(%q)\n  got  %q\n  want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestConvert_MismatchedCloser(t *testing.T) {
	// [[p]].[[/v]] — opener is p but closer is v. The [[/v]] terminates the [[p]] tag.
	// "mu[[/v]]" has no opener so "mu" is plain text and the orphan [[/v]] is stripped.
	raw := "[[v]]i[[/v]][[p]].[[/v]]mu[[/v]]"
	got := Convert(raw)
	want := `<span class="token-variable">i</span><span class="token-punctuation">.</span>mu`
	if got != want {
		t.Errorf("mismatched closer:\n  got  %q\n  want %q", got, want)
	}

	pt := PlainText(raw)
	if pt != "i.mu" {
		t.Errorf("mismatched closer PlainText: got %q want %q", pt, "i.mu")
	}
}

func TestConvert_MalformedCloser(t *testing.T) {
	// [[p]]}[/p]] — single-bracket closer
	raw := "    [[p]]}[/p]]"
	got := Convert(raw)
	want := `    <span class="token-punctuation">}</span>`
	if got != want {
		t.Errorf("malformed closer:\n  got  %q\n  want %q", got, want)
	}
}

func TestConvert_BracketInContent(t *testing.T) {
	// [[p]][[[/p]][[v]]item[[/v]][[p]]][[/p]] → <span class="token-punctuation">[</span><span class="token-variable">item</span><span class="token-punctuation">]</span>
	raw := "[[p]][[[/p]][[v]]item[[/v]][[p]]][[/p]]"
	got := Convert(raw)
	want := `<span class="token-punctuation">[</span><span class="token-variable">item</span><span class="token-punctuation">]</span>`
	if got != want {
		t.Errorf("bracket in content:\n  got  %q\n  want %q", got, want)
	}
}

func TestConvert_OrphanCloserInPlainText(t *testing.T) {
	// Orphan [/p]] in plain text should be stripped.
	raw := "foo[/p]]bar"
	got := Convert(raw)
	want := "foobar"
	if got != want {
		t.Errorf("orphan closer:\n  got  %q\n  want %q", got, want)
	}
}

func TestConvert_FullSnippetOriginalBug(t *testing.T) {
	// The original bug report snippet.
	raw := "[[k]]if[[/k]] [[v]]stock[[/v]] [[o]]<[[/o]] [[v]]amount[[/v]] [[p]]{[[/p]]\n" +
		"        [[v]]fmt[[/v]][[p]].[[/p]][[f]]Printf[[/f]][[p]]([[/p]][[s]]\"Error: Insufficient stock for %s\\n\"[[/s]][[p]],[[/p]] [[v]]item[[/v]][[p]])[[/p]]\n" +
		"        [[k]]return[[/k]]\n" +
		"    [[p]]}[/p]]\n" +
		"    [[v]]inventory[[/v]][[p]][[[/p]][[v]]item[[/v]][[p]]][[/p]] [[o]]-=[[/o]] [[v]]amount[[/v]]"
	got := PlainText(raw)
	expect := []string{
		"if stock < amount {",
		"fmt.Printf(\"Error: Insufficient stock for %s\\n\", item)",
		"return",
		"}",
		"inventory[item] -= amount",
	}
	for _, e := range expect {
		if !containsStr(got, e) {
			t.Errorf("PlainText missing %q in:\n%s", e, got)
		}
	}
}

func TestConvert_FullSnippetNewBug(t *testing.T) {
	// The new bug report: [[p]].[[/v]] mismatched closers everywhere.
	raw := "[[k]]func[[/k]] [[p]]([[/p]][[v]]i[[/v]] [[o]]*[[/o]][[v]]Inventory[[/v]][[p]])[[/p]] [[f]]ProcessOrder[[/f]][[p]]([[/p]][[v]]name[[/v]] [[s]]string[[/s]][[p]],[[/p]] [[v]]qty[[/v]] [[n]]int[[/n]][[p]])[[/p]] [[v]]error[[/v]] [[p]]{[[/p]]\n" +
		"  [[v]]i[[/v]][[p]].[[/v]]mu[[/v]][[p]].[[/v]]Lock[[/v]][[p]]()[[/p]]\n" +
		"  [[k]]defer[[/k]] [[v]]i[[/v]][[p]].[[/v]]mu[[/v]][[p]].[[/v]]Unlock[[/v]][[p]]()[[/p]]\n" +
		"\n" +
		"  [[k]]if[[/k]] [[v]]i[[/v]][[p]].[[/v]]products[[/v]][[p]][[[/p]][[v]]name[[/v]][[p]]][[/p]] [[o]]<[[/o]] [[v]]qty[[/v]] [[p]]{[[/p]]\n" +
		"    [[k]]return[[/k]] [[v]]fmt[[/v]][[p]].[[/v]]Errorf[[/v]][[p]]([[/s]]\"insufficient stock\"[[/s]][[p]])[[/p]]\n" +
		"  [[p]]}[[/p]]\n" +
		"  [[v]]i[[/v]][[p]].[[/v]]products[[/v]][[p]][[[/p]][[v]]name[[/v]][[p]]][[/p]] [[o]]-=[[/o]] [[v]]qty[[/v]]\n" +
		"  [[k]]return[[/k]] [[k]]nil[[/k]]\n" +
		"[[p]]}[[/p]]"
	got := PlainText(raw)
	expect := []string{
		"func (i *Inventory) ProcessOrder(name string, qty int) error {",
		"i.mu.Lock()",
		"defer i.mu.Unlock()",
		"if i.products[name] < qty {",
		`return fmt.Errorf("insufficient stock")`,
		"i.products[name] -= qty",
		"return nil",
	}
	for _, e := range expect {
		if !containsStr(got, e) {
			t.Errorf("PlainText missing %q in:\n%s", e, got)
		}
	}
	// HTML output should not contain any raw tag markers.
	html := Convert(raw)
	for _, bad := range []string{"[[v]]", "[[p]]", "[[/v]]", "[[/p]]", "[/p]]", "[/v]]"} {
		if containsStr(html, bad) {
			t.Errorf("Convert output contains raw tag %q:\n%s", bad, html)
		}
	}
}

func TestConvert_AdjacentTags(t *testing.T) {
	// [[v]]fmt[[/v]][[p]].[[/p]][[f]]Printf[[/f]] → fmt.Printf
	raw := "[[v]]fmt[[/v]][[p]].[[/p]][[f]]Printf[[/f]]"
	got := PlainText(raw)
	if got != "fmt.Printf" {
		t.Errorf("adjacent tags: got %q want %q", got, "fmt.Printf")
	}
}

func TestConvert_EmptyContent(t *testing.T) {
	raw := "[[k]][[/k]]"
	got := Convert(raw)
	want := `<span class="token-keyword"></span>`
	if got != want {
		t.Errorf("empty content: got %q want %q", got, want)
	}
}

func TestConvert_UnknownTag(t *testing.T) {
	raw := "[[xyz]]hello[[/xyz]]"
	got := Convert(raw)
	want := "[[xyz]]hello[[/xyz]]"
	if got != want {
		t.Errorf("unknown tag: got %q want %q", got, want)
	}
}

func TestConvert_UnclosedTag(t *testing.T) {
	raw := "[[k]]forever"
	got := Convert(raw)
	want := `<span class="token-keyword">forever</span>`
	if got != want {
		t.Errorf("unclosed tag: got %q want %q", got, want)
	}
}

func TestConvert_NestedBrackets(t *testing.T) {
	// [[p]]([[/p]][[s]]"insufficient stock"[[/s]][[p]])[[/p]]
	raw := `[[p]]([[/p]][[s]]"insufficient stock"[[/s]][[p]])[[/p]]`
	got := PlainText(raw)
	want := `("insufficient stock")`
	if got != want {
		t.Errorf("nested brackets: got %q want %q", got, want)
	}
}

func TestConvert_WrongTypeCloser_MidStream(t *testing.T) {
	// [[p]]([[/s]] — opener is p, closer is s. Content should be "("
	raw := "[[p]]([[/s]]"
	got := PlainText(raw)
	if got != "(" {
		t.Errorf("wrong-type closer: got %q want %q", got, "(")
	}
}

func TestPlainText_MatchesStrippedHTML(t *testing.T) {
	raw := "[[k]]if[[/k]] [[v]]x[[/v]]"
	pt := PlainText(raw)
	if pt != "if x" {
		t.Errorf("PlainText: got %q want %q", pt, "if x")
	}
}

func TestConvert_MultipleLinesNewlines(t *testing.T) {
	raw := "[[k]]func[[/k]] [[f]]main[[/f]][[p]]()[[/p]] [[p]]{[[/p]]\n  [[v]]fmt[[/v]][[p]].[[/p]][[f]]Println[[/f]][[p]]([[/p]][[s]]\"hello\"[[/s]][[p]])[[/p]]\n[[p]]}[[/p]]"
	pt := PlainText(raw)
	want := "func main() {\n  fmt.Println(\"hello\")\n}"
	if pt != want {
		t.Errorf("multiline:\n  got  %q\n  want %q", pt, want)
	}
}

func TestConvert_CloserWithoutOpener(t *testing.T) {
	raw := "hello[[/k]]world"
	got := PlainText(raw)
	if got != "helloworld" {
		t.Errorf("closer without opener: got %q want %q", got, "helloworld")
	}
}

func TestStripOrphanClosers(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no orphans", "hello world", "hello world"},
		{"single orphan", "}[/p]]", "}"},
		{"multiple orphans", "a[/k]]b[/v]]c", "abc"},
		{"non-valid type preserved", "a[/xyz]]b", "a[/xyz]]b"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripOrphanClosers(tt.in)
			if got != tt.want {
				t.Errorf("stripOrphanClosers(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func containsStr(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (haystack == needle || len(haystack) > 0 && containsIndex(haystack, needle))
}

func containsIndex(h, n string) bool {
	for i := 0; i <= len(h)-len(n); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}

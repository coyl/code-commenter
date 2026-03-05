// Package highlight: Chroma-based syntax highlighting for plain code.
// Outputs HTML with token-keyword, token-string, etc. to match existing frontend CSS.

package highlight

import (
	"html"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// CodeToHTML highlights plain source code using Chroma and returns HTML with
// class names token-keyword, token-string, token-comment, token-number,
// token-function, token-operator, token-punctuation, token-variable.
// Language is the Chroma lexer name (e.g. "go", "javascript", "python").
func CodeToHTML(code, language string) (string, error) {
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for token := it(); token != chroma.EOF; token = it() {
		cls := chromaTokenToClass(token.Type)
		escaped := html.EscapeString(token.Value)
		if cls != "" {
			b.WriteString(`<span class="token-`)
			b.WriteString(cls)
			b.WriteString(`">`)
			b.WriteString(escaped)
			b.WriteString("</span>")
		} else {
			b.WriteString(escaped)
		}
	}
	return b.String(), nil
}

// chromaTokenToClass maps Chroma token types to our CSS class suffixes.
func chromaTokenToClass(tt chroma.TokenType) string {
	switch {
	case tt.InCategory(chroma.Keyword):
		return "keyword"
	case tt.InCategory(chroma.Comment):
		return "comment"
	case tt.InCategory(chroma.LiteralString), tt.InCategory(chroma.LiteralStringAffix):
		return "string"
	case tt.InCategory(chroma.LiteralNumber):
		return "number"
	case tt.InCategory(chroma.Operator):
		return "operator"
	case tt.InCategory(chroma.Punctuation):
		return "punctuation"
	case tt.InCategory(chroma.NameFunction), tt.InCategory(chroma.NameBuiltin):
		return "function"
	case tt.InCategory(chroma.NameVariable), tt.InCategory(chroma.Name), tt.InCategory(chroma.NameConstant):
		return "variable"
	default:
		return "" // Text, Generic, etc. — no span
	}
}

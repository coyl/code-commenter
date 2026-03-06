package highlight

import core "code-commenter/api/internal/highlight"

// Adapter maps highlight package into renderer port.
type Adapter struct{}

func (Adapter) CodeToHTML(code, language string) (string, error) {
	return core.CodeToHTML(code, language)
}

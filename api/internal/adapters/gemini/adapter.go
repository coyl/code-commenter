package gemini

import (
	"context"

	core "code-commenter/api/internal/gemini"
	"code-commenter/api/internal/ports"
)

// Adapter maps the concrete Gemini client to app ports.
type Adapter struct {
	Client   *core.Client
	TTSModel string
}

func (a *Adapter) GenerateTaskSpec(ctx context.Context, task, language string) (string, string, error) {
	return a.Client.GenerateTaskSpec(ctx, task, language)
}

func (a *Adapter) GenerateCSS(ctx context.Context, spec, language string) (string, error) {
	return a.Client.GenerateCSS(ctx, spec, language)
}

func (a *Adapter) GenerateCode(ctx context.Context, spec, language string) (string, error) {
	return a.Client.GenerateCode(ctx, spec, language)
}

func (a *Adapter) GenerateCodeSegments(ctx context.Context, spec, language string) ([]ports.CodeSegment, string, error) {
	segments, rawJSON, err := a.Client.GenerateCodeSegments(ctx, spec, language)
	if err != nil {
		return nil, rawJSON, err
	}
	out := make([]ports.CodeSegment, 0, len(segments))
	for _, seg := range segments {
		out = append(out, ports.CodeSegment{
			Code:      seg.Code,
			Narration: seg.Narration,
		})
	}
	return out, rawJSON, nil
}

func (a *Adapter) GenerateWrappingNarration(ctx context.Context, spec, language string) (string, error) {
	return a.Client.GenerateWrappingNarration(ctx, spec, language)
}

func (a *Adapter) GenerateChange(ctx context.Context, currentCSS, currentCode, userMessage, language string) (string, string, string, error) {
	return a.Client.GenerateChange(ctx, currentCSS, currentCode, userMessage, language)
}

func (a *Adapter) GenerateAudioChunks(ctx context.Context, narration string) ([]string, error) {
	chunks := make([]string, 0, 32)
	err := a.Client.GenerateAudioStream(ctx, a.TTSModel, narration, func(base64Chunk string) error {
		chunks = append(chunks, base64Chunk)
		return nil
	})
	return chunks, err
}

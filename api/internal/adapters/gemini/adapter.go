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

func (a *Adapter) GenerateTaskSpec(ctx context.Context, task, language, narrationLang string) (string, string, error) {
	return a.Client.GenerateTaskSpec(ctx, task, language, narrationLang)
}

func (a *Adapter) GenerateCSS(ctx context.Context, spec, language string) (string, error) {
	return a.Client.GenerateCSS(ctx, spec, language)
}

func (a *Adapter) GenerateCode(ctx context.Context, spec, language string) (string, error) {
	return a.Client.GenerateCode(ctx, spec, language)
}

func (a *Adapter) GenerateCodeSegments(ctx context.Context, spec, language, narrationLang string) ([]ports.CodeSegment, string, error) {
	segments, rawJSON, err := a.Client.GenerateCodeSegments(ctx, spec, language, narrationLang)
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

func (a *Adapter) FormatAndSegmentCode(ctx context.Context, code, narrationLang string) ([]ports.CodeSegment, string, error) {
	segments, rawJSON, err := a.Client.FormatAndSegmentCode(ctx, code, narrationLang)
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

func (a *Adapter) GenerateWrappingNarration(ctx context.Context, spec, language, narrationLang string) (string, error) {
	return a.Client.GenerateWrappingNarration(ctx, spec, language, narrationLang)
}

func (a *Adapter) GenerateWrappingNarrationForUserCode(ctx context.Context, segmentNarrationsSummary, narrationLang string) (string, error) {
	return a.Client.GenerateWrappingNarrationForUserCode(ctx, segmentNarrationsSummary, narrationLang)
}
func (a *Adapter) GenerateTitle(ctx context.Context, spec, prompt string) (string, error) {
	return a.Client.GenerateTitle(ctx, spec, prompt)
}

func (a *Adapter) GenerateAudioChunks(ctx context.Context, narration string) ([]string, error) {
	chunks := make([]string, 0, 32)
	err := a.Client.GenerateAudioStream(ctx, a.TTSModel, narration, func(base64Chunk string) error {
		chunks = append(chunks, base64Chunk)
		return nil
	})
	return chunks, err
}

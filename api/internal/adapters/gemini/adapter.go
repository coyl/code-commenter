package gemini

import (
	"context"
	"encoding/base64"
	"strings"

	core "code-commenter/api/internal/gemini"
	"code-commenter/api/internal/ports"
)

// Adapter maps the concrete Gemini client to app ports.
type Adapter struct {
	Client         *core.Client
	TTSModel       string
	TimestampModel string // cheap model for audio-timestamp extraction (e.g. gemini-2.5-flash-lite)
}

func (a *Adapter) GenerateTaskSpec(ctx context.Context, task, language, narrationLang string) (string, string, error) {
	return a.Client.GenerateTaskSpec(ctx, task, language, narrationLang)
}

func (a *Adapter) GenerateCSS(ctx context.Context, spec, language string) (string, error) {
	return a.Client.GenerateCSS(ctx, spec, language)
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

func (a *Adapter) GenerateStory(ctx context.Context, title, spec, language, narrationLang, segmentNarrations string) (string, error) {
	return a.Client.GenerateStory(ctx, title, spec, language, narrationLang, segmentNarrations)
}

func (a *Adapter) GenerateImages(ctx context.Context, title, spec, language, segmentNarrations string) (ports.JobImages, error) {
	preview, illustration, err := a.Client.GenerateImages(ctx, title, spec, language, segmentNarrations)
	return ports.JobImages{
		PreviewImageBase64:      preview,
		IllustrationImageBase64: illustration,
	}, err
}

func (a *Adapter) GenerateAudioChunks(ctx context.Context, narration string) ([]string, error) {
	chunks := make([]string, 0, 32)
	err := a.Client.GenerateAudioStream(ctx, a.TTSModel, narration, func(base64Chunk string) error {
		chunks = append(chunks, base64Chunk)
		return nil
	})
	return chunks, err
}

// GenerateAudioBatched runs one TTS request for all narrations (joined with pauses),
// then uses a cheap LLM to find per-segment timestamps and splits the PCM accordingly.
func (a *Adapter) GenerateAudioBatched(ctx context.Context, narrations []string) (map[int][]string, error) {
	if len(narrations) == 0 {
		return nil, nil
	}
	fullScript := strings.Join(narrations, "\n\n\n")
	var pcm []byte
	err := a.Client.GenerateAudioStream(ctx, a.TTSModel, fullScript, func(b64 string) error {
		raw, decErr := base64.StdEncoding.DecodeString(b64)
		if decErr != nil {
			return decErr
		}
		pcm = append(pcm, raw...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(narrations) == 1 {
		return map[int][]string{0: {base64.StdEncoding.EncodeToString(pcm)}}, nil
	}

	wav := core.PCMToWAV(pcm, core.TTSSampleRate)
	offsets, tsErr := a.Client.FindAudioTimestamps(ctx, a.TimestampModel, wav, narrations)
	if tsErr != nil {
		// Fallback: proportional split by word count.
		return a.splitByWordCount(pcm, narrations), nil
	}
	segments := core.SplitPCMAtOffsets(pcm, core.TTSSampleRate, offsets)
	out := make(map[int][]string, len(segments))
	for i, seg := range segments {
		if len(seg) == 0 {
			continue
		}
		out[i] = []string{base64.StdEncoding.EncodeToString(seg)}
	}
	return out, nil
}

// splitByWordCount is the fallback when LLM timestamp detection fails.
func (a *Adapter) splitByWordCount(pcm []byte, narrations []string) map[int][]string {
	totalWords := 0
	wordCounts := make([]int, len(narrations))
	for i, n := range narrations {
		wc := len(strings.Fields(n))
		if wc == 0 {
			wc = 1
		}
		wordCounts[i] = wc
		totalWords += wc
	}
	if totalWords == 0 {
		totalWords = 1
	}
	totalSamples := len(pcm) / 2
	out := make(map[int][]string, len(narrations))
	offset := 0
	for i, wc := range wordCounts {
		segSamples := totalSamples * wc / totalWords
		end := offset + segSamples
		if i == len(narrations)-1 {
			end = totalSamples
		}
		if end > totalSamples {
			end = totalSamples
		}
		seg := pcm[offset*2 : end*2]
		if len(seg) > 0 {
			out[i] = []string{base64.StdEncoding.EncodeToString(seg)}
		}
		offset = end
	}
	return out
}

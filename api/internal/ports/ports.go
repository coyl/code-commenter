package ports

import "context"

// CodeSegment is plain code with narration text.
type CodeSegment struct {
	Code      string
	Narration string
}

// SessionData is the canonical application session shape.
type SessionData struct {
	ID        string
	CSS       string
	Code      string
	Language  string
	Spec      string
	Narration string
}

// JobSegment persists one emitted segment.
type JobSegment struct {
	Code      string
	CodePlain string
	Narration string
}

// GenerationPort owns text/code generation operations.
// narrationLang is the language for all voiceover/narration text (e.g. "english", "german", "spanish", "italian", "chinese").
type GenerationPort interface {
	GenerateTaskSpec(ctx context.Context, task, language, narrationLang string) (spec, narration string, err error)
	GenerateCSS(ctx context.Context, spec, language string) (string, error)
	GenerateCode(ctx context.Context, spec, language string) (string, error)
	GenerateCodeSegments(ctx context.Context, spec, language, narrationLang string) ([]CodeSegment, string, error)
	// FormatAndSegmentCode takes user-provided code, beautifies only indentation/newlines, and returns segments with narration. Language is inferred by the LLM.
	FormatAndSegmentCode(ctx context.Context, code, narrationLang string) ([]CodeSegment, string, error)
	GenerateWrappingNarration(ctx context.Context, spec, language, narrationLang string) (string, error)
	// GenerateWrappingNarrationForUserCode returns a short closing voiceover for user-pasted code (segmentNarrationsSummary is concatenated segment narrations).
	GenerateWrappingNarrationForUserCode(ctx context.Context, segmentNarrationsSummary, narrationLang string) (string, error)
	GenerateChange(ctx context.Context, currentCSS, currentCode, userMessage, language string) (newCSS, newCode, unifiedDiff string, err error)
}

// AudioPort owns narration -> audio chunk generation.
type AudioPort interface {
	GenerateAudioChunks(ctx context.Context, narration string) ([]string, error)
}

// RendererPort converts source code into renderable HTML.
type RendererPort interface {
	CodeToHTML(code, language string) (string, error)
}

// SessionRepository stores and retrieves sessions.
type SessionRepository interface {
	Put(sess SessionData)
	Get(id string) *SessionData
}

// JobRepository archives generated jobs and loads them by id.
type JobRepository interface {
	UploadJob(ctx context.Context, jobID, prompt, rawJSON, fullCode, fullCodePlain string, segments []JobSegment, segmentAudio [][]byte) error
	GetJob(ctx context.Context, jobID string) (interface{}, error)
	IsEnabled() bool
}

// StreamEvent is a typed internal event for stream delivery.
type StreamEvent struct {
	Type         string
	EventVersion int
	JobID        string
	TimestampMs  int64

	ID        string
	Spec      string
	CSS       string
	Code      string
	CodePlain string
	Narration string
	RawJSON   string
	Index     int
	Error     string
	AudioData string
}

// EventSink emits stream events to transport.
type EventSink interface {
	Emit(event StreamEvent) error
}

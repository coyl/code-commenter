package ports

import "context"

// UserInfo identifies the authenticated user (from session).
type UserInfo struct {
	Sub   string // Google subject ID
	Email string
}

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
	GenerateCodeSegments(ctx context.Context, spec, language, narrationLang string) ([]CodeSegment, string, error)
	// FormatAndSegmentCode takes user-provided code, beautifies only indentation/newlines, and returns segments with narration. Language is inferred by the LLM.
	FormatAndSegmentCode(ctx context.Context, code, narrationLang string) ([]CodeSegment, string, error)
	GenerateWrappingNarration(ctx context.Context, spec, language, narrationLang string) (string, error)
	// GenerateWrappingNarrationForUserCode returns a short closing voiceover for user-pasted code (segmentNarrationsSummary is concatenated segment narrations).
	GenerateWrappingNarrationForUserCode(ctx context.Context, segmentNarrationsSummary, narrationLang string) (string, error)
	GenerateTitle(ctx context.Context, spec, prompt string) (string, error)
	// GenerateStory returns an HTML article body (no html/head/body tags) describing the problem and solution.
	// The text includes the marker {{EMBED_PLAYER}} exactly once, positioned in the middle so an embed iframe can be injected there.
	GenerateStory(ctx context.Context, title, spec, language, segmentNarrations string) (string, error)
}

// AudioPort owns narration -> audio chunk generation.
type AudioPort interface {
	GenerateAudioChunks(ctx context.Context, narration string) ([]string, error)
	// GenerateAudioBatched runs one TTS request for all narrations (joined with pauses), then splits by silence.
	// narrations[i] is segment i; len(narrations) may include a final wrapping segment. Returns map[index]->base64 chunks.
	GenerateAudioBatched(ctx context.Context, narrations []string) (map[int][]string, error)
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
	UploadJob(ctx context.Context, jobID, prompt, rawJSON, fullCode, fullCodePlain, css, title, narrationLang, ownerSub, ownerEmail, storyHTML string, segments []JobSegment, segmentAudio [][]byte) error
	GetJob(ctx context.Context, jobID string) (interface{}, error)
	IsEnabled() bool
}

// JobMeta is a minimal job entry for listing (e.g. "my jobs").
type JobMeta struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"createdAt"`
}

// JobIndex stores job metadata for listing by owner. Optional; used for GET /jobs/mine and GET /jobs/recent.
type JobIndex interface {
	Add(ctx context.Context, jobID, ownerSub, ownerEmail, title string) error
	ListByOwner(ctx context.Context, ownerSub string, limit int) ([]JobMeta, error)
	// ListRecent returns the most recently created jobs across all owners, newest first.
	ListRecent(ctx context.Context, limit int) ([]JobMeta, error)
}

// DailyQuota limits generations per user per day. When auth is enabled, stream handler uses TryConsumeSlot before run and ReleaseSlot on failure.
type DailyQuota interface {
	GetTodayCount(ctx context.Context, ownerSub string) (int, error)
	// TryConsumeSlot atomically consumes one slot if under limit. Returns true if consumed, false if at limit. Use ReleaseSlot if generation later fails.
	TryConsumeSlot(ctx context.Context, ownerSub string) (ok bool, err error)
	// ReleaseSlot returns one slot (e.g. when generation failed after TryConsumeSlot succeeded).
	ReleaseSlot(ctx context.Context, ownerSub string) error
}

// DailyGenerationLimit is the max generations per user per day when quota is enforced.
const DailyGenerationLimit = 3

// StreamEvent is a typed internal event for stream delivery.
type StreamEvent struct {
	Type         string
	EventVersion int
	JobID        string
	TimestampMs  int64

	ID        string
	Stage     string // human-readable stage title for progress UI (e.g. "Generating CSS…")
	Spec      string
	CSS       string
	Code      string
	CodePlain string
	Narration string
	RawJSON   string
	StoryHTML string
	Index     int
	Error     string
	AudioData string
}

// EventSink emits stream events to transport.
type EventSink interface {
	Emit(event StreamEvent) error
}

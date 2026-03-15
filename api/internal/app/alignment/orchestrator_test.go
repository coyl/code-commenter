package alignment

import (
	"context"
	"errors"
	"strings"
	"testing"

	domain "code-commenter/api/internal/domain/alignment"
	"code-commenter/api/internal/ports"
)

type fakeGeneration struct {
	segments []ports.CodeSegment
}

func (f fakeGeneration) GenerateTaskSpec(context.Context, string, string, string) (string, string, error) {
	return "spec", "intro", nil
}
func (f fakeGeneration) GenerateCSS(context.Context, string, string) (string, error) {
	return ".x{}", nil
}
func (f fakeGeneration) GenerateCodeSegments(context.Context, string, string, string) ([]ports.CodeSegment, string, error) {
	return f.segments, `[{"c":"code","n":"narration"}]`, nil
}
func (f fakeGeneration) FormatAndSegmentCode(context.Context, string, string) ([]ports.CodeSegment, string, error) {
	return f.segments, `[{"c":"code","n":"narration"}]`, nil
}
func (f fakeGeneration) GenerateWrappingNarration(context.Context, string, string, string) (string, error) {
	return "", nil
}
func (f fakeGeneration) GenerateWrappingNarrationForUserCode(context.Context, string, string) (string, error) {
	return "", nil
}
func (f fakeGeneration) GenerateTitle(context.Context, string, string) (string, error) {
	return "Test title", nil
}
func (f fakeGeneration) GenerateStory(context.Context, string, string, string, string, string) (string, error) {
	return "<p>Intro</p>\n{{EMBED_PLAYER}}\n<p>Outro</p>", nil
}
func (f fakeGeneration) GenerateImages(context.Context, string, string, string, string) (ports.JobImages, error) {
	return ports.JobImages{
		PreviewImageBase64:      "fake-preview",
		IllustrationImageBase64: "fake-illustration",
	}, nil
}

type inspectingGeneration struct {
	fakeGeneration
	titleHadDeadline bool
	storyHadDeadline bool
}

func (g *inspectingGeneration) GenerateTitle(ctx context.Context, spec, prompt string) (string, error) {
	_, g.titleHadDeadline = ctx.Deadline()
	return g.fakeGeneration.GenerateTitle(ctx, spec, prompt)
}

func (g *inspectingGeneration) GenerateStory(ctx context.Context, title, spec, language, narrationLang, segmentNarrations string) (string, error) {
	_, g.storyHadDeadline = ctx.Deadline()
	return g.fakeGeneration.GenerateStory(ctx, title, spec, language, narrationLang, segmentNarrations)
}

func (g *inspectingGeneration) GenerateImages(ctx context.Context, title, spec, language, segmentNarrations string) (ports.JobImages, error) {
	return g.fakeGeneration.GenerateImages(ctx, title, spec, language, segmentNarrations)
}

type fakeAudio struct {
	errFor map[string]error
}

func (f fakeAudio) GenerateAudioChunks(_ context.Context, narration string) ([]string, error) {
	if err := f.errFor[narration]; err != nil {
		return nil, err
	}
	return []string{"audio-" + narration}, nil
}

func (f fakeAudio) GenerateAudioBatched(_ context.Context, narrations []string) (map[int][]string, error) {
	out := make(map[int][]string, len(narrations))
	for i, n := range narrations {
		if err := f.errFor[n]; err != nil {
			return nil, err
		}
		out[i] = []string{"batched-" + n}
	}
	return out, nil
}

type fakeRenderer struct{}

func (fakeRenderer) CodeToHTML(code, _ string) (string, error) {
	return "<pre>" + code + "</pre>", nil
}

type fakeSessions struct {
	last ports.SessionData
}

func (f *fakeSessions) Put(sess ports.SessionData) {
	f.last = sess
}
func (f *fakeSessions) Get(string) *ports.SessionData {
	return nil
}

type fakeJobs struct{ enabled bool }

func (fakeJobs) UploadJob(_ context.Context, _, _, _, _, _, _, _, _, _, _, _ string, _ ports.JobImages, _ []ports.JobSegment, _ [][]byte) error {
	return nil
}
func (fakeJobs) GetJob(context.Context, string) (interface{}, error) {
	return nil, errors.New("not implemented")
}
func (f fakeJobs) IsEnabled() bool { return f.enabled }

type inspectingJobs struct {
	enabled           bool
	uploadHadDeadline bool
}

func (j *inspectingJobs) UploadJob(ctx context.Context, _, _, _, _, _, _, _, _, _, _, _ string, _ ports.JobImages, _ []ports.JobSegment, _ [][]byte) error {
	_, j.uploadHadDeadline = ctx.Deadline()
	return nil
}
func (j *inspectingJobs) GetJob(context.Context, string) (interface{}, error) {
	return nil, errors.New("not implemented")
}
func (j *inspectingJobs) IsEnabled() bool { return j.enabled }

type captureSink struct {
	events []ports.StreamEvent
}

func (s *captureSink) Emit(e ports.StreamEvent) error {
	s.events = append(s.events, e)
	return nil
}

func eventTypes(events []ports.StreamEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

func TestOrchestratorEmitsCompatibleOrder(t *testing.T) {
	sessions := &fakeSessions{}
	sink := &captureSink{}
	o := &StreamOrchestrator{
		Generation: fakeGeneration{segments: []ports.CodeSegment{
			{Code: "const a = 1", Narration: "segment-one"},
		}},
		Audio:    fakeAudio{errFor: map[string]error{}},
		Renderer: fakeRenderer{},
		Sessions: sessions,
		Jobs:     fakeJobs{enabled: true},
		Aligner:  domain.Service{},
	}

	_, err := o.Run(context.Background(), StreamRequest{
		Task:     "build x",
		Language: "javascript",
	}, sink)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}

	if len(sink.events) < 7 {
		t.Fatalf("expected at least 7 events, got %d", len(sink.events))
	}
	// Event order must include these types in order (stage events may appear in between).
	// story and visuals are delivered before session so the client receives all assets first.
	wantOrder := []string{"job_started", "spec", "css", "segment", "audio", "code_done", "story", "visuals", "session"}
	idx := 0
	for _, e := range sink.events {
		if idx < len(wantOrder) && e.Type == wantOrder[idx] {
			idx++
		}
	}
	if idx != len(wantOrder) {
		t.Fatalf("expected event order prefix %v, got %d matches (events: %v)",
			wantOrder, idx, eventTypes(sink.events))
	}
	if sessions.last.ID == "" {
		t.Fatal("session should be persisted")
	}
}

func TestOrchestratorContinuesWhenSegmentTTSFails(t *testing.T) {
	sessions := &fakeSessions{}
	sink := &captureSink{}
	o := &StreamOrchestrator{
		Generation:     fakeGeneration{segments: []ports.CodeSegment{
			{Code: "fmt.Println(1)", Narration: "bad-segment"},
		}},
		Audio:          fakeAudio{errFor: map[string]error{"bad-segment": errors.New("tts down")}},
		Renderer:       fakeRenderer{},
		Sessions:       sessions,
		Jobs:           fakeJobs{},
		Aligner:        domain.Service{},
		TTSPerSegment:  true, // per-segment mode: one segment can fail and we still emit session
	}

	_, err := o.Run(context.Background(), StreamRequest{Task: "x", Language: "go"}, sink)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}

	foundError := false
	foundSession := false
	for _, e := range sink.events {
		if e.Type == "error" {
			foundError = true
		}
		if e.Type == "session" {
			foundSession = true
		}
	}
	if !foundError {
		t.Fatal("expected error event for failed TTS")
	}
	if !foundSession {
		t.Fatal("expected final session event despite TTS error")
	}
}

func TestOrchestratorRejectsUserCodeOverLimit(t *testing.T) {
	sink := &captureSink{}
	o := &StreamOrchestrator{
		Generation: fakeGeneration{segments: []ports.CodeSegment{{Code: "x", Narration: "n"}}},
		Audio:      fakeAudio{errFor: map[string]error{}},
		Renderer:   fakeRenderer{},
		Sessions:   &fakeSessions{},
		Jobs:       fakeJobs{},
		Aligner:    domain.Service{},
	}

	overLimit := strings.Repeat("x", MaxUserCodeLength+1)
	_, err := o.Run(context.Background(), StreamRequest{
		Code:              overLimit,
		NarrationLanguage: "english",
	}, sink)
	if err == nil {
		t.Fatal("Run() expected error when code exceeds MaxUserCodeLength")
	}

	var foundErrorEvent bool
	for _, e := range sink.events {
		if e.Type == "error" && strings.Contains(e.Error, "exceeds maximum length") {
			foundErrorEvent = true
			break
		}
	}
	if !foundErrorEvent {
		t.Fatalf("expected error event with limit message; events: %v", sink.events)
	}
	// Generation (FormatAndSegmentCode) must not be called: no segment events from real generation
	segmentCount := 0
	for _, e := range sink.events {
		if e.Type == "segment" {
			segmentCount++
		}
	}
	if segmentCount > 0 {
		t.Fatalf("expected no segment events when over limit, got %d", segmentCount)
	}
}

func TestOrchestratorUsesFreshTimeoutOnlyForUpload(t *testing.T) {
	sessions := &fakeSessions{}
	sink := &captureSink{}
	gen := &inspectingGeneration{
		fakeGeneration: fakeGeneration{
			segments: []ports.CodeSegment{
				{Code: "const a = 1", Narration: "segment-one"},
			},
		},
	}
	jobs := &inspectingJobs{enabled: true}
	o := &StreamOrchestrator{
		Generation: gen,
		Audio:      fakeAudio{errFor: map[string]error{}},
		Renderer:   fakeRenderer{},
		Sessions:   sessions,
		Jobs:       jobs,
		Aligner:    domain.Service{},
	}

	_, err := o.Run(context.Background(), StreamRequest{
		Task:     "build x",
		Language: "javascript",
	}, sink)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}
	if gen.titleHadDeadline {
		t.Fatal("expected GenerateTitle context without timeout deadline")
	}
	if gen.storyHadDeadline {
		t.Fatal("expected GenerateStory context without timeout deadline")
	}
	if !jobs.uploadHadDeadline {
		t.Fatal("expected UploadJob context to include timeout deadline")
	}
}

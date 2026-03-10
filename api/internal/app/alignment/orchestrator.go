package alignment

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	domain "code-commenter/api/internal/domain/alignment"
	"code-commenter/api/internal/ports"
)

const s3UploadTimeout = 90 * time.Second

// MaxUserCodeLength is the maximum allowed length (in runes/characters) for pasted code in the "Your code" flow. Must match client limit.
const MaxUserCodeLength = 5000

// StreamRequest is the inbound request for a stream session.
// If Code is non-empty, the flow uses the user's code (format + segment only); otherwise task is used to generate code.
// NarrationLanguage is the language for voiceover text (e.g. "english", "german").
type StreamRequest struct {
	Task              string
	Language          string
	Code              string // optional: user-provided code for "Your code" flow
	NarrationLanguage string
}

// StreamOrchestrator coordinates generation, alignment, and persistence.
type StreamOrchestrator struct {
	Generation ports.GenerationPort
	Audio      ports.AudioPort
	Renderer   ports.RendererPort
	Sessions   ports.SessionRepository
	Jobs       ports.JobRepository
	Aligner    domain.Service
}

// Run executes the end-to-end stream flow and emits transport-neutral events.
func (o *StreamOrchestrator) Run(ctx context.Context, req StreamRequest, sink ports.EventSink) (string, error) {
	if strings.TrimSpace(req.NarrationLanguage) == "" {
		req.NarrationLanguage = "english"
	}
	userCodeMode := strings.TrimSpace(req.Code) != ""
	if !userCodeMode && req.Language == "" {
		req.Language = "javascript"
	}
	if !userCodeMode && req.Task == "" {
		req.Task = "a simple hello world"
	}

	jobID := ""
	if id, err := uuid.NewV7(); err == nil {
		jobID = id.String()
		if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "job_started", ID: jobID})); err != nil {
			return "", err
		}
	}

	if userCodeMode {
		trimmed := strings.TrimSpace(req.Code)
		if runeCount := len([]rune(trimmed)); runeCount > MaxUserCodeLength {
			errMsg := fmt.Sprintf("code exceeds maximum length (%d characters, limit %d)", runeCount, MaxUserCodeLength)
			_ = sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: errMsg}))
			return "", fmt.Errorf("%s", errMsg)
		}
	}

	streamStart := time.Now()
	log.Info().Str("phase", "start").Str("job", jobID).Bool("userCode", userCodeMode).Dur("elapsed", 0).Msg("stream task")

	var spec, narration string
	if userCodeMode {
		spec = "User-provided code snippet for narration."
		if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "spec", Spec: spec, Narration: narration})); err != nil {
			return "", err
		}
	} else {
		var err error
		spec, narration, err = o.Generation.GenerateTaskSpec(ctx, req.Task, req.Language, req.NarrationLanguage)
		log.Info().Str("phase", "spec").Dur("elapsed", time.Since(streamStart)).Msg("stream task")
		if err != nil {
			_ = sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: "spec: " + err.Error()}))
			return "", err
		}
		if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "spec", Spec: spec, Narration: narration})); err != nil {
			return "", err
		}
	}

	css, err := o.Generation.GenerateCSS(ctx, spec, req.Language)
	if err != nil {
		_ = sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: "css: " + err.Error()}))
		return "", err
	}
	if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "css", CSS: css})); err != nil {
		return "", err
	}
	log.Info().Str("phase", "css").Dur("elapsed", time.Since(streamStart)).Msg("stream task")

	var segments []ports.CodeSegment
	var rawSegmentsJSON string
	if userCodeMode {
		segments, rawSegmentsJSON, err = o.Generation.FormatAndSegmentCode(ctx, strings.TrimSpace(req.Code), req.NarrationLanguage)
	} else {
		segments, rawSegmentsJSON, err = o.Generation.GenerateCodeSegments(ctx, spec, req.Language, req.NarrationLanguage)
	}
	if err != nil {
		_ = sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: "segments: " + err.Error()}))
		return "", err
	}
	log.Info().Str("phase", "segments").Int("n", len(segments)).Dur("elapsed", time.Since(streamStart)).Msg("stream task")

	// Token bucket: 10 TTS requests per minute (gemini-2.5-flash-tts quota). Burst 1 so we never start more than 1 per 6s.
	ttsLimiter := rate.NewLimiter(rate.Every(6*time.Second), 1)
	audioByIndex := make(map[int]domain.SegmentAudio, len(segments))
	audioResults := make(chan domain.SegmentAudio, len(segments)+1)
	var wg sync.WaitGroup
	for i, seg := range segments {
		if seg.Narration == "" {
			continue
		}
		idx, narrationText := i, seg.Narration
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ttsLimiter.Wait(ctx); err != nil {
				audioResults <- domain.SegmentAudio{Index: idx, Err: err}
				return
			}
			chunks, ttsErr := o.Audio.GenerateAudioChunks(ctx, narrationText)
			audioResults <- domain.SegmentAudio{Index: idx, Chunks: chunks, Err: ttsErr}
		}()
	}
	go func() {
		wg.Wait()
		close(audioResults)
	}()
	for res := range audioResults {
		audioByIndex[res.Index] = res
	}

	lang := normalizeLexerLanguage(req.Language)
	segmentEntries := make([]domain.Segment, 0, len(segments))
	var fullPlain strings.Builder
	var fullHTML strings.Builder
	for i, seg := range segments {
		segHTML, renderErr := o.Renderer.CodeToHTML(seg.Code, lang)
		if renderErr != nil {
			segHTML = html.EscapeString(seg.Code)
		}

		if i > 0 {
			fullPlain.WriteString("\n")
			fullHTML.WriteString("\n")
			segHTML = "\n" + segHTML
		}
		fullPlain.WriteString(seg.Code)
		fullHTML.WriteString(strings.TrimPrefix(segHTML, "\n"))

		plain := seg.Code
		if i > 0 {
			plain = "\n" + seg.Code
		}
		segmentEntries = append(segmentEntries, domain.Segment{
			Index:     i,
			Code:      segHTML,
			CodePlain: plain,
			Narration: seg.Narration,
		})
	}

	aligned, err := o.Aligner.Align(segmentEntries, audioByIndex)
	if err != nil {
		_ = sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: "align: " + err.Error()}))
		return "", err
	}
	for _, item := range aligned {
		if err := sink.Emit(o.event(jobID, ports.StreamEvent{
			Type:      "segment",
			Index:     item.Segment.Index,
			Code:      item.Segment.Code,
			CodePlain: item.Segment.CodePlain,
			Narration: item.Segment.Narration,
		})); err != nil {
			return "", err
		}
		if item.Err != nil {
			log.Error().Err(item.Err).Int("segment", item.Segment.Index).Msg("TTS error")
			if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: "TTS: " + item.Err.Error()})); err != nil {
				return "", err
			}
			continue
		}
		for _, b64 := range item.Audio {
			if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "audio", AudioData: b64})); err != nil {
				return "", err
			}
		}
	}

	wrapAudio := []string{}
	var wrapping string
	if userCodeMode {
		var summary strings.Builder
		for i, seg := range segments {
			if i > 0 {
				summary.WriteString(" ")
			}
			summary.WriteString(strings.TrimSpace(seg.Narration))
		}
		s := summary.String()
		const maxSummaryBytes = 1500
		if len(s) > maxSummaryBytes {
			end := maxSummaryBytes
			for end > 0 && !utf8.ValidString(s[:end]) {
				end--
			}
			s = s[:end] + "..."
		}
		if s != "" {
			wrapping, _ = o.Generation.GenerateWrappingNarrationForUserCode(ctx, s, req.NarrationLanguage)
		}
	} else {
		wrapping, _ = o.Generation.GenerateWrappingNarration(ctx, spec, req.Language, req.NarrationLanguage)
	}
	log.Info().Str("phase", "wrapping").Dur("elapsed", time.Since(streamStart)).Msg("stream task")
	if wrapping != "" {
		if err := sink.Emit(o.event(jobID, ports.StreamEvent{
			Type:      "segment",
			Index:     len(segmentEntries),
			Code:      "",
			CodePlain: "",
			Narration: wrapping,
		})); err != nil {
			return "", err
		}
		chunks, ttsErr := o.Audio.GenerateAudioChunks(ctx, wrapping)
		if ttsErr != nil {
			log.Error().Err(ttsErr).Msg("TTS wrapping error")
			if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: "TTS: " + ttsErr.Error()})); err != nil {
				return "", err
			}
		} else {
			wrapAudio = chunks
			for _, b64 := range chunks {
				if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "audio", AudioData: b64})); err != nil {
					return "", err
				}
			}
		}
	}

	codePlain := strings.TrimSpace(fullPlain.String())
	if err := sink.Emit(o.event(jobID, ports.StreamEvent{
		Type:      "code_done",
		Code:      fullHTML.String(),
		CodePlain: codePlain,
		RawJSON:   rawSegmentsJSON,
	})); err != nil {
		return "", err
	}
	log.Info().Str("phase", "code_done").Dur("elapsed", time.Since(streamStart)).Msg("stream task")

	id := jobID
	if id == "" {
		id = uuid.NewString()
	}
	o.Sessions.Put(ports.SessionData{
		ID:        id,
		CSS:       css,
		Code:      codePlain,
		Language:  req.Language,
		Spec:      spec,
		Narration: "",
	})

	if o.Jobs != nil && o.Jobs.IsEnabled() && jobID != "" {
		storedSegments := make([]ports.JobSegment, 0, len(aligned)+1)
		segmentAudio := make([][]byte, 0, len(aligned)+1)
		for _, item := range aligned {
			storedSegments = append(storedSegments, ports.JobSegment{
				Code:      item.Segment.Code,
				CodePlain: item.Segment.CodePlain,
				Narration: item.Segment.Narration,
			})
			var pcm []byte
			if item.Err == nil {
				for _, b64 := range item.Audio {
					dec, _ := base64.StdEncoding.DecodeString(b64)
					pcm = append(pcm, dec...)
				}
			}
			segmentAudio = append(segmentAudio, pcm)
		}
		if wrapping != "" {
			storedSegments = append(storedSegments, ports.JobSegment{Narration: wrapping})
			var wrapPCM []byte
			for _, b64 := range wrapAudio {
				dec, _ := base64.StdEncoding.DecodeString(b64)
				wrapPCM = append(wrapPCM, dec...)
			}
			segmentAudio = append(segmentAudio, wrapPCM)
		}
		uploadCtx, cancelUpload := context.WithTimeout(context.WithoutCancel(ctx), s3UploadTimeout)
		defer cancelUpload()
		jobPrompt := req.Task
		if userCodeMode {
			jobPrompt = "User-provided code"
		}
		if upErr := o.Jobs.UploadJob(uploadCtx, jobID, jobPrompt, rawSegmentsJSON, fullHTML.String(), codePlain, storedSegments, segmentAudio); upErr != nil {
			ev := log.Error().Err(upErr).Str("job", jobID).Dur("timeout", s3UploadTimeout)
			if errors.Is(upErr, context.DeadlineExceeded) {
				ev.Msg("S3 upload timed out")
			} else {
				ev.Msg("S3 upload failed")
			}
		}
	}

	if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "session", ID: id})); err != nil {
		return "", err
	}
	return id, nil
}

func (o *StreamOrchestrator) event(jobID string, event ports.StreamEvent) ports.StreamEvent {
	event.JobID = jobID
	event.EventVersion = 1
	event.TimestampMs = time.Now().UnixMilli()
	return event
}

func normalizeLexerLanguage(lang string) string {
	switch strings.ToLower(lang) {
	case "go", "golang":
		return "go"
	case "js", "javascript":
		return "javascript"
	case "ts", "typescript":
		return "typescript"
	case "py", "python":
		return "python"
	case "rb", "ruby":
		return "ruby"
	case "rs", "rust":
		return "rust"
	default:
		return lang
	}
}

package alignment

import (
	"context"
	"encoding/base64"
	"errors"
	"html"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	domain "code-commenter/api/internal/domain/alignment"
	"code-commenter/api/internal/ports"
)

const s3UploadTimeout = 90 * time.Second

// StreamRequest is the inbound request for a stream session.
type StreamRequest struct {
	Task     string
	Language string
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
	if req.Task == "" {
		req.Task = "a simple hello world"
	}
	if req.Language == "" {
		req.Language = "javascript"
	}

	jobID := ""
	if id, err := uuid.NewV7(); err == nil {
		jobID = id.String()
		if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "job_started", ID: jobID})); err != nil {
			return "", err
		}
	}

	streamStart := time.Now()
	log.Info().Str("phase", "start").Str("job", jobID).Dur("elapsed", 0).Msg("stream task")

	spec, narration, err := o.Generation.GenerateTaskSpec(ctx, req.Task, req.Language)
	log.Info().Str("phase", "spec").Dur("elapsed", time.Since(streamStart)).Msg("stream task")
	if err != nil {
		_ = sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: "spec: " + err.Error()}))
		return "", err
	}
	if err := sink.Emit(o.event(jobID, ports.StreamEvent{Type: "spec", Spec: spec, Narration: narration})); err != nil {
		return "", err
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

	segments, rawSegmentsJSON, err := o.Generation.GenerateCodeSegments(ctx, spec, req.Language)
	if err != nil {
		_ = sink.Emit(o.event(jobID, ports.StreamEvent{Type: "error", Error: "segments: " + err.Error()}))
		return "", err
	}
	log.Info().Str("phase", "segments").Int("n", len(segments)).Dur("elapsed", time.Since(streamStart)).Msg("stream task")

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
	wrapping, wrapErr := o.Generation.GenerateWrappingNarration(ctx, spec, req.Language)
	log.Info().Str("phase", "wrapping").Dur("elapsed", time.Since(streamStart)).Msg("stream task")
	if wrapErr == nil && wrapping != "" {
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
		if upErr := o.Jobs.UploadJob(uploadCtx, jobID, req.Task, rawSegmentsJSON, fullHTML.String(), codePlain, storedSegments, segmentAudio); upErr != nil {
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

package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
	"code-commenter/api/internal/gemini"
	"code-commenter/api/internal/highlight"
	"code-commenter/api/internal/jobstore"
	"code-commenter/api/internal/store"
)

// StreamTaskRequest is the first message from client: { "task", "language" }.
type StreamTaskRequest struct {
	Task     string `json:"task"`
	Language string `json:"language"`
}

const s3UploadTimeout = 90 * time.Second

// HandleStreamTask runs the full pipeline (spec → CSS → code segments with synced narration); each segment sends code + narration then TTS for that narration.
func HandleStreamTask(gc *gemini.Client, st *store.Store, apiKey, liveModel, ttsModel string, jobStore *jobstore.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			http.Error(w, "API not configured", http.StatusServiceUnavailable)
			return
		}
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()

		var req StreamTaskRequest
		if err := ws.ReadJSON(&req); err != nil {
			_ = writeStreamErr(ws, err.Error())
			return
		}
		if req.Task == "" {
			req.Task = "a simple hello world"
		}
		if req.Language == "" {
			req.Language = "javascript"
		}

		jobID := ""
		if id, err := uuid.NewV7(); err == nil {
			jobID = id.String()
			if ws.WriteJSON(map[string]string{"type": "job_started", "id": jobID}) != nil {
				return
			}
		}

		ctx := r.Context()
		send := func(v interface{}) bool {
			return ws.WriteJSON(v) == nil
		}

		streamStart := time.Now()
		log.Info().Str("phase", "start").Str("job", jobID).Dur("elapsed", 0).Msg("stream task")

		// 1) Spec + narration
		spec, narration, err := gc.GenerateTaskSpec(ctx, req.Task, req.Language)
		log.Info().Str("phase", "spec").Dur("elapsed", time.Since(streamStart)).Msg("stream task")
		if err != nil {
			_ = writeStreamErr(ws, "spec: "+err.Error())
			return
		}
		if !send(map[string]interface{}{"type": "spec", "spec": spec, "narration": narration}) {
			return
		}

		// 2) CSS (with syntax highlighting for the chosen language)
		css, err := gc.GenerateCSS(ctx, spec, req.Language)
		if err != nil {
			_ = writeStreamErr(ws, "css: "+err.Error())
			return
		}
		if !send(map[string]interface{}{"type": "css", "css": css}) {
			return
		}
		log.Info().Str("phase", "css").Dur("elapsed", time.Since(streamStart)).Msg("stream task")

		// 3) Code as segments with synced narration: each segment = code + narration, then TTS for that narration.
		// Run TTS asynchronously for all segments; deliver segment messages and audio in order.
		segments, rawSegmentsJSON, err := gc.GenerateCodeSegments(ctx, spec, req.Language)
		if err != nil {
			_ = writeStreamErr(ws, "segments: "+err.Error())
			return
		}
		log.Info().Str("phase", "segments").Int("n", len(segments)).Dur("elapsed", time.Since(streamStart)).Msg("stream task")

		type ttsResult struct {
			Index  int
			Chunks []string
			Err    error
		}
		// Start TTS for every segment that has narration (async)
		ttsResults := make(chan ttsResult, len(segments)+1)
		var wg sync.WaitGroup
		for i, seg := range segments {
			if seg.Narration == "" {
				continue
			}
			idx, narration := i, seg.Narration
			wg.Add(1)
			go func() {
				defer wg.Done()
				chunks, err := generateAudioChunks(ctx, gc, ttsModel, narration)
				ttsResults <- ttsResult{Index: idx, Chunks: chunks, Err: err}
			}()
		}
		go func() {
			wg.Wait()
			close(ttsResults)
		}()

		// Collect all async TTS results by index so we can deliver in order
		pendingAudio := make(map[int]ttsResult)
		ttsCount := 0
		for _, seg := range segments {
			if seg.Narration != "" {
				ttsCount++
			}
		}
		for n := 0; n < ttsCount; n++ {
			res, ok := <-ttsResults
			if !ok {
				break
			}
			pendingAudio[res.Index] = res
		}

		lexerLang := normalizeLexerLanguage(req.Language)
		// Precompute highlighted HTML once per segment; reused for WebSocket, fullHTML, and S3.
		segmentHTMLs := make([]string, len(segments))
		for i, seg := range segments {
			html, err := highlight.CodeToHTML(seg.Code, lexerLang)
			if err != nil {
				html = htmlEscapeCode(seg.Code)
			}
			segmentHTMLs[i] = html
		}

		var fullPlain strings.Builder
		for i, seg := range segments {
			if i > 0 {
				fullPlain.WriteString("\n")
			}
			fullPlain.WriteString(seg.Code)
			segHTML := segmentHTMLs[i]
			segCodePlain := seg.Code
			if i > 0 {
				segHTML = "\n" + segHTML
				segCodePlain = "\n" + seg.Code
			}
			if !send(map[string]interface{}{
				"type":      "segment",
				"index":     i,
				"code":      segHTML,
				"codePlain": segCodePlain,
				"narration": seg.Narration,
			}) {
				return
			}
			if seg.Narration != "" {
				res := pendingAudio[i]
				if res.Err != nil {
					log.Error().Err(res.Err).Int("segment", i).Msg("TTS error")
					_ = ws.WriteJSON(map[string]string{"type": "error", "error": "TTS: " + res.Err.Error()})
				} else {
					for _, b64 := range res.Chunks {
						if !send(map[string]interface{}{"type": "audio", "data": b64}) {
							return
						}
					}
				}
			}
		}

		// 4) Final wrapping narration: outline what was done, played without code (narration-only segment)
		var wrapAudioChunks []string
		wrapping, err := gc.GenerateWrappingNarration(ctx, spec, req.Language)
		log.Info().Str("phase", "wrapping").Dur("elapsed", time.Since(streamStart)).Msg("stream task")
		if err == nil && wrapping != "" {
			if !send(map[string]interface{}{
				"type":      "segment",
				"index":     len(segments),
				"code":      "",
				"codePlain": "",
				"narration": wrapping,
			}) {
				return
			}
			wrapCh := make(chan ttsResult, 1)
			go func() {
				chunks, err := generateAudioChunks(ctx, gc, ttsModel, wrapping)
				wrapCh <- ttsResult{Index: len(segments), Chunks: chunks, Err: err}
			}()
			res := <-wrapCh
			if res.Err != nil {
				log.Error().Err(res.Err).Msg("TTS wrapping error")
				_ = ws.WriteJSON(map[string]string{"type": "error", "error": "TTS: " + res.Err.Error()})
			} else {
				wrapAudioChunks = res.Chunks
				for _, b64 := range res.Chunks {
					if !send(map[string]interface{}{"type": "audio", "data": b64}) {
						return
					}
				}
			}
		}
		codePlain := strings.TrimSpace(fullPlain.String())
		var fullHTML strings.Builder
		for i := range segments {
			if i > 0 {
				fullHTML.WriteString("\n")
			}
			fullHTML.WriteString(segmentHTMLs[i])
		}
		if !send(map[string]interface{}{"type": "code_done", "code": fullHTML.String(), "codePlain": codePlain, "rawJson": rawSegmentsJSON}) {
			return
		}
		log.Info().Str("phase", "code_done").Dur("elapsed", time.Since(streamStart)).Msg("stream task")

		id := jobID
		if id == "" {
			id = uuid.New().String()
		}
		st.Put(&store.Session{
			ID:        id,
			CSS:       css,
			Code:      codePlain,
			Language:  req.Language,
			Spec:      spec,
			Narration: "", // full narration is segment list
		})

		if jobStore != nil && jobID != "" {
			segmentsStored := make([]jobstore.SegmentStored, 0, len(segments)+1)
			segmentAudio := make([][]byte, 0, len(segments)+1)
			for i, seg := range segments {
				segHTML := segmentHTMLs[i]
				if i > 0 {
					segHTML = "\n" + segHTML
				}
				segCodePlain := seg.Code
				if i > 0 {
					segCodePlain = "\n" + seg.Code
				}
				segmentsStored = append(segmentsStored, jobstore.SegmentStored{
					Code:      segHTML,
					CodePlain: segCodePlain,
					Narration: seg.Narration,
				})
				var pcm []byte
				if seg.Narration != "" {
					res := pendingAudio[i]
					if res.Err == nil {
						for _, b64 := range res.Chunks {
							dec, _ := base64.StdEncoding.DecodeString(b64)
							pcm = append(pcm, dec...)
						}
					}
				}
				segmentAudio = append(segmentAudio, pcm)
			}
			if wrapping != "" {
				segmentsStored = append(segmentsStored, jobstore.SegmentStored{Narration: wrapping})
				var wrapPcm []byte
				for _, b64 := range wrapAudioChunks {
					dec, _ := base64.StdEncoding.DecodeString(b64)
					wrapPcm = append(wrapPcm, dec...)
				}
				segmentAudio = append(segmentAudio, wrapPcm)
			}
			uploadCtx, cancelUpload := context.WithTimeout(context.WithoutCancel(ctx), s3UploadTimeout)
			defer cancelUpload()
			if err := jobStore.UploadJob(uploadCtx, jobID, req.Task, rawSegmentsJSON, fullHTML.String(), codePlain, segmentsStored, segmentAudio); err != nil {
				ev := log.Error().Err(err).Str("job", jobID).Dur("timeout", s3UploadTimeout)
				if errors.Is(err, context.DeadlineExceeded) {
					ev.Msg("S3 upload timed out")
				} else {
					ev.Msg("S3 upload failed")
				}
			}
		}

		_ = send(map[string]interface{}{"type": "session", "id": id})
	}
}

// normalizeLexerLanguage maps frontend language names to Chroma lexer names.
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

func htmlEscapeCode(code string) string {
	return html.EscapeString(code)
}

func writeStreamErr(ws *websocket.Conn, msg string) error {
	return ws.WriteJSON(map[string]string{"type": "error", "error": msg})
}

// voiceSetup returns Live API setup with AUDIO-only response for TTS (per Live API docs).
func voiceSetup(model string) []byte {
	setup := map[string]interface{}{
		"setup": map[string]interface{}{
			"model": model,
			"generationConfig": map[string]interface{}{
				"responseModalities": []string{"AUDIO"},
			},
			"systemInstruction": map[string]interface{}{
				"parts": []map[string]interface{}{{"text": "You are a voice narrator. Speak the user's message clearly. Output only speech, no text."}},
			},
		},
	}
	b, _ := json.Marshal(setup)
	return b
}

// generateAudioChunks runs TTS and returns all base64 PCM chunks (for async generation; client receives in order).
func generateAudioChunks(ctx context.Context, gc *gemini.Client, ttsModel, narration string) (chunks []string, err error) {
	err = gc.GenerateAudioStream(ctx, ttsModel, narration, func(base64Chunk string) error {
		chunks = append(chunks, base64Chunk)
		return nil
	})
	return chunks, err
}

// streamVoiceToClient connects to Live API, sends narration text via clientContent (per Live API docs), and forwards audio to the client. Returns the number of audio chunks sent.
func streamVoiceToClient(clientWS *websocket.Conn, apiKey, model, narration string) int {
	u, _ := url.Parse(liveAPIWS)
	q := u.Query()
	q.Set("key", apiKey)
	u.RawQuery = q.Encode()
	geminiWS, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		_ = clientWS.WriteJSON(map[string]string{"type": "error", "error": "Live API: " + err.Error()})
		return 0
	}
	defer geminiWS.Close()

	if err := geminiWS.WriteMessage(websocket.TextMessage, voiceSetup(model)); err != nil {
		log.Error().Err(err).Str("op", "live").Msg("write setup")
		return 0
	}
	// Per Live API docs: send text via clientContent with turns and turnComplete: true
	clientContent := map[string]interface{}{
		"clientContent": map[string]interface{}{
			"turns": []map[string]interface{}{
				{
					"role": "user",
					"parts": []map[string]interface{}{{"text": narration}},
				},
			},
			"turnComplete": true,
		},
	}
	payload, _ := json.Marshal(clientContent)
	if err := geminiWS.WriteMessage(websocket.TextMessage, payload); err != nil {
		log.Error().Err(err).Str("op", "live").Msg("write clientContent")
		return 0
	}
	log.Debug().Int("narration_len", len(narration)).Msg("live: sent narration, reading serverContent")

	audioChunks := 0
	for {
		mt, data, err := geminiWS.ReadMessage()
		if err != nil {
			log.Warn().Err(err).Int("audio_chunks", audioChunks).Msg("live: read error")
			break
		}
		if mt != websocket.TextMessage {
			continue
		}
		var msg map[string]interface{}
		if json.Unmarshal(data, &msg) != nil {
			continue
		}
		if audioChunks == 0 && len(msg) > 0 {
			keys := make([]string, 0, len(msg))
			for k := range msg {
				keys = append(keys, k)
			}
			log.Debug().Strs("keys", keys).Msg("live: server message keys")
		}
		// serverContent / server_content (camelCase common in JSON)
		sc, _ := msg["serverContent"].(map[string]interface{})
		if sc == nil {
			sc, _ = msg["server_content"].(map[string]interface{})
		}
		if sc == nil {
			if content, ok := msg["content"].(map[string]interface{}); ok {
				sc = content
			}
		}
		if sc == nil {
			continue
		}
		mturn, _ := sc["modelTurn"].(map[string]interface{})
		if mturn == nil {
			mturn, _ = sc["model_turn"].(map[string]interface{})
		}
		if mturn != nil {
			parts, _ := mturn["parts"].([]interface{})
			for _, p := range parts {
				if part, ok := p.(map[string]interface{}); ok {
					audioChunks += extractAndSendAudio(clientWS, part)
				}
			}
		}
		topParts, _ := sc["parts"].([]interface{})
		for _, p := range topParts {
			part, _ := p.(map[string]interface{})
			if part != nil {
				audioChunks += extractAndSendAudio(clientWS, part)
			}
		}
	}
	log.Debug().Int("audio_chunks", audioChunks).Msg("live: done")
	return audioChunks
}

func extractAndSendAudio(clientWS *websocket.Conn, part map[string]interface{}) int {
	inline, _ := part["inlineData"].(map[string]interface{})
	if inline == nil {
		inline, _ = part["inline_data"].(map[string]interface{})
	}
	if inline == nil {
		return 0
	}
	mime, _ := inline["mimeType"].(string)
	if mime == "" {
		mime, _ = inline["mime_type"].(string)
	}
	if mime != "" && mime != "audio/pcm" && mime != "audio/pcm;rate=24000" {
		return 0
	}
	dataStr, _ := inline["data"].(string)
	if dataStr != "" {
		_ = clientWS.WriteJSON(map[string]interface{}{"type": "audio", "data": dataStr})
		return 1
	}
	return 0
}

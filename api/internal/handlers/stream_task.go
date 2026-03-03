package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"code-commenter/api/internal/gemini"
	"code-commenter/api/internal/store"
)

// StreamTaskRequest is the first message from client: { "task", "language" }.
type StreamTaskRequest struct {
	Task     string `json:"task"`
	Language string `json:"language"`
}

// HandleStreamTask runs the full pipeline (spec → CSS → code segments with synced narration); each segment sends code + narration then TTS for that narration.
func HandleStreamTask(gc *gemini.Client, st *store.Store, apiKey, liveModel, ttsModel string) http.HandlerFunc {
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

		ctx := r.Context()
		send := func(v interface{}) bool {
			return ws.WriteJSON(v) == nil
		}

		// 1) Spec + narration
		spec, narration, err := gc.GenerateTaskSpec(ctx, req.Task, req.Language)
		if err != nil {
			_ = writeStreamErr(ws, "spec: "+err.Error())
			return
		}
		if !send(map[string]interface{}{"type": "spec", "spec": spec, "narration": narration}) {
			return
		}

		// 2) CSS
		css, err := gc.GenerateCSS(ctx, spec)
		if err != nil {
			_ = writeStreamErr(ws, "css: "+err.Error())
			return
		}
		if !send(map[string]interface{}{"type": "css", "css": css}) {
			return
		}

		// 3) Code as segments with synced narration: each segment = code + narration, then TTS for that narration
		segments, err := gc.GenerateCodeSegments(ctx, spec, req.Language)
		if err != nil {
			_ = writeStreamErr(ws, "segments: "+err.Error())
			return
		}
		var fullCode strings.Builder
		for i, seg := range segments {
			if i > 0 {
				fullCode.WriteString("\n")
			}
			fullCode.WriteString(seg.Code)
			segmentPayload := seg.Code
			if i > 0 {
				segmentPayload = "\n" + seg.Code
			}
			if !send(map[string]interface{}{
				"type":      "segment",
				"index":     i,
				"code":      segmentPayload,
				"narration": seg.Narration,
			}) {
				return
			}
			if seg.Narration != "" {
				streamVoiceViaTTS(ctx, gc, ws, ttsModel, seg.Narration)
			}
		}
		codeStr := strings.TrimSpace(fullCode.String())
		if !send(map[string]interface{}{"type": "code_done", "code": codeStr}) {
			return
		}

		id := uuid.New().String()
		st.Put(&store.Session{
			ID:        id,
			CSS:       css,
			Code:      codeStr,
			Language:  req.Language,
			Spec:      spec,
			Narration: "", // full narration is segment list
		})
		_ = send(map[string]interface{}{"type": "session", "id": id})
	}
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

// streamVoiceViaTTS uses REST TTS (gemini-2.5-pro-preview-tts) to generate speech and streams base64 PCM chunks to the client (same format as Live API).
func streamVoiceViaTTS(ctx context.Context, gc *gemini.Client, clientWS *websocket.Conn, ttsModel, narration string) {
	err := gc.GenerateAudioStream(ctx, ttsModel, narration, func(base64Chunk string) error {
		return clientWS.WriteJSON(map[string]interface{}{"type": "audio", "data": base64Chunk})
	})
	if err != nil {
		log.Printf("[tts] fallback TTS error: %v", err)
		_ = clientWS.WriteJSON(map[string]string{"type": "error", "error": "TTS: " + err.Error()})
	}
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
		log.Printf("[live] write setup: %v", err)
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
		log.Printf("[live] write clientContent: %v", err)
		return 0
	}
	log.Printf("[live] sent narration (%d chars), reading serverContent...", len(narration))

	audioChunks := 0
	for {
		mt, data, err := geminiWS.ReadMessage()
		if err != nil {
			log.Printf("[live] read error: %v (forwarded %d audio chunks)", err, audioChunks)
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
			log.Printf("[live] server message keys: %v", keys)
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
	log.Printf("[live] done (forwarded %d audio chunks)", audioChunks)
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

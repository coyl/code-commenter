package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

const liveAPIWS = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"

// LiveAPIModel is the default Live API model for native audio (override via config).
var LiveAPIModel = "gemini-2.5-flash-native-audio-preview-12-2025"

// HandleLive proxies the frontend WebSocket to the Gemini Live API so the API key stays on the server.
// Query param: ?key=API_KEY (or we inject from config). First message from client can be setup; we inject API key into URL when connecting to Google.
func HandleLive(apiKey, liveModel string) http.HandlerFunc {
	if liveModel != "" {
		LiveAPIModel = liveModel
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" {
			http.Error(w, "Live API not configured (missing API key)", http.StatusServiceUnavailable)
			return
		}
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		clientWS, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer clientWS.Close()

		u, _ := url.Parse(liveAPIWS)
		q := u.Query()
		q.Set("key", apiKey)
		u.RawQuery = q.Encode()

		geminiWS, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			_ = clientWS.WriteJSON(map[string]string{"error": "failed to connect to Live API: " + err.Error()})
			return
		}
		defer geminiWS.Close()

		// If client sends a setup message first, forward it; otherwise send default setup so Live API expects audio.
		go forward(clientWS, geminiWS, true, liveModel)
		forward(geminiWS, clientWS, false, "")
	}
}

// forward copies messages from src to dst. If fromClient and first message is not setup, send default setup to Gemini.
func forward(src, dst *websocket.Conn, fromClient bool, liveModel string) {
	model := LiveAPIModel
	if liveModel != "" {
		model = liveModel
	}
	sentSetup := !fromClient
	for {
		mt, data, err := src.ReadMessage()
		if err != nil {
			break
		}
		if fromClient && !sentSetup {
			// Optionally inject or rewrite setup with our model and API key (already in URL).
			var msg map[string]interface{}
			if json.Unmarshal(data, &msg) == nil {
				if _, hasSetup := msg["setup"]; hasSetup {
					sentSetup = true
				} else {
					// Client might send audio first; then we need to send setup to Gemini before any audio.
					setup := defaultSetup(model)
					if err := dst.WriteMessage(websocket.TextMessage, setup); err != nil {
						break
					}
					sentSetup = true
				}
			}
		}
		if err := dst.WriteMessage(mt, data); err != nil {
			break
		}
	}
}

func defaultSetup(model string) []byte {
	setup := map[string]interface{}{
		"setup": map[string]interface{}{
			"model":                 model,
			"generationConfig":      map[string]interface{}{"responseModalities": []string{"AUDIO", "TEXT"}},
			"systemInstruction":     map[string]interface{}{"parts": []map[string]interface{}{{"text": "You are a helpful assistant. Respond with clear speech and optional text."}}},
		},
	}
	b, _ := json.Marshal(setup)
	return b
}

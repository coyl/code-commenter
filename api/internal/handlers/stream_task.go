package handlers

import (
	"net/http"

	"github.com/gorilla/websocket"

	wsadapter "code-commenter/api/internal/adapters/ws"
	appalignment "code-commenter/api/internal/app/alignment"
	"code-commenter/api/internal/auth"
)

// StreamTaskRequest is the first message from client: { "task", "language" } or { "code", "language" } for "Your code" flow.
type StreamTaskRequest struct {
	Task              string `json:"task"`
	Language          string `json:"language"`
	Code              string `json:"code"`
	NarrationLanguage string `json:"narration_language"`
}

// HandleStreamTask runs the stream orchestrator and forwards typed events over websocket.
func HandleStreamTask(orchestrator *appalignment.StreamOrchestrator, apiKey string) http.HandlerFunc {
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
			_ = ws.WriteJSON(map[string]string{"type": "error", "error": err.Error()})
			return
		}
		// Defaults for task/language are applied in orchestrator.Run
		owner := auth.UserFromContext(r.Context())

		sink := wsadapter.Sink{Conn: ws}
		_, _ = orchestrator.Run(r.Context(), appalignment.StreamRequest{
			Task:              req.Task,
			Language:          req.Language,
			Code:              req.Code,
			NarrationLanguage: req.NarrationLanguage,
			Owner:             owner,
		}, sink)
	}
}

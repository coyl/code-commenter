package handlers

import (
	"net/http"

	"github.com/gorilla/websocket"

	wsadapter "code-commenter/api/internal/adapters/ws"
	appalignment "code-commenter/api/internal/app/alignment"
	"code-commenter/api/internal/auth"
	"code-commenter/api/internal/ports"
)

// StreamTaskRequest is the first message from client: { "task", "language" } or { "code", "language" } for "Your code" flow.
type StreamTaskRequest struct {
	Task              string `json:"task"`
	Language          string `json:"language"`
	Code              string `json:"code"`
	NarrationLanguage string `json:"narration_language"`
}

// HandleStreamTask runs the stream orchestrator and forwards typed events over websocket.
// When quota is non-nil and auth is enabled, enforces daily generation limit before run and increments after success.
func HandleStreamTask(orchestrator *appalignment.StreamOrchestrator, apiKey string, quota ports.DailyQuota) http.HandlerFunc {
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
		owner := auth.UserFromContext(r.Context())

		if owner != nil && quota != nil {
			count, err := quota.GetTodayCount(r.Context(), owner.Sub)
			if err != nil {
				_ = ws.WriteJSON(map[string]string{"type": "error", "error": "quota check failed"})
				return
			}
			if count >= ports.DailyGenerationLimit {
				_ = ws.WriteJSON(map[string]string{"type": "error", "error": "Daily limit reached. You can generate up to 3 tasks per day."})
				return
			}
		}

		sink := wsadapter.Sink{Conn: ws}
		_, runErr := orchestrator.Run(r.Context(), appalignment.StreamRequest{
			Task:              req.Task,
			Language:          req.Language,
			Code:              req.Code,
			NarrationLanguage: req.NarrationLanguage,
			Owner:             owner,
		}, sink)

		if runErr == nil && owner != nil && quota != nil {
			_ = quota.IncrementToday(r.Context(), owner.Sub)
		}
	}
}

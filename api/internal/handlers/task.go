package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"code-commenter/api/internal/ports"
)

// TaskRequest is the JSON body for POST /task.
type TaskRequest struct {
	Task     string `json:"task"`
	Language string `json:"language"`
}

// TaskResponse is the JSON response for POST /task.
type TaskResponse struct {
	ID           string `json:"id"`
	CSS          string `json:"css"`
	Code         string `json:"code"`
	Spec         string `json:"spec,omitempty"`
	Narration    string `json:"narration,omitempty"`
	VoiceoverURL string `json:"voiceoverUrl,omitempty"` // optional; can be empty if streamed later
}

// HandleTask creates a new task: Gemini spec + CSS + code; stores session; returns CSS, code, narration.
func HandleTask(gen ports.GenerationPort, sessions ports.SessionRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req TaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Task == "" {
			http.Error(w, "task is required", http.StatusBadRequest)
			return
		}
		if req.Language == "" {
			req.Language = "javascript"
		}

		spec, narration, err := gen.GenerateTaskSpec(r.Context(), req.Task, req.Language)
		if err != nil {
			http.Error(w, "task spec failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		css, err := gen.GenerateCSS(r.Context(), spec, req.Language)
		if err != nil {
			http.Error(w, "css generation failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		code, err := gen.GenerateCode(r.Context(), spec, req.Language)
		if err != nil {
			http.Error(w, "code generation failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		id := uuid.New().String()
		sessions.Put(ports.SessionData{
			ID:        id,
			CSS:       css,
			Code:      code,
			Language:  req.Language,
			Spec:      spec,
			Narration: narration,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TaskResponse{
			ID:        id,
			CSS:       css,
			Code:      code,
			Spec:      spec,
			Narration: narration,
		})
	}
}

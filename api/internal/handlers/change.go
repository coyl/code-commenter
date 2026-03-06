package handlers

import (
	"encoding/json"
	"net/http"

	"code-commenter/api/internal/ports"
)

// ChangeRequest is the JSON body for POST /task/:id/change.
type ChangeRequest struct {
	Message string `json:"message"`
}

// ChangeResponse is the JSON response for POST /task/:id/change.
type ChangeResponse struct {
	CSS         string `json:"css"`
	Code        string `json:"code"`
	UnifiedDiff string `json:"unifiedDiff"`
}

// HandleChange applies a user change request: Gemini returns new CSS and code diff; we apply diff and store.
func HandleChange(gen ports.GenerationPort, sessions ports.SessionRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "task id required", http.StatusBadRequest)
			return
		}
		sess := sessions.Get(id)
		if sess == nil {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}

		var req ChangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}

		newCSS, newCode, unifiedDiff, err := gen.GenerateChange(r.Context(), sess.CSS, sess.Code, req.Message, sess.Language)
		if err != nil {
			http.Error(w, "change generation failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if newCode == "" {
			newCode = sess.Code
		}
		if newCSS == "" {
			newCSS = sess.CSS
		}

		sessions.Put(ports.SessionData{
			ID:        sess.ID,
			CSS:       newCSS,
			Code:      newCode,
			Language:  sess.Language,
			Spec:      sess.Spec,
			Narration: sess.Narration,
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChangeResponse{
			CSS:         newCSS,
			Code:        newCode,
			UnifiedDiff: unifiedDiff,
		})
	}
}

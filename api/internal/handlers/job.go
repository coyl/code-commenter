package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"code-commenter/api/internal/jobstore"
)

// HandleGetJob returns a job by ID from S3 (GET /jobs/{id}).
func HandleGetJob(store *jobstore.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "job id required", http.StatusBadRequest)
			return
		}
		if store == nil {
			http.Error(w, "job storage not configured", http.StatusServiceUnavailable)
			return
		}
		job, err := store.GetJob(r.Context(), id)
		if err != nil {
			log.Debug().Err(err).Str("id", id).Msg("get job")
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(job)
	}
}

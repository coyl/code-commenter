package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"

	"code-commenter/api/internal/auth"
	"code-commenter/api/internal/ports"
)

// RecentJobLister is implemented by *jobstore.Client.
type RecentJobLister interface {
	ListRecent(ctx context.Context, limit int) ([]ports.JobMeta, error)
}

// HandleGetJob returns a job by ID from S3 (GET /jobs/{id}).
func HandleGetJob(store ports.JobRepository) http.HandlerFunc {
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
		if store == nil || !store.IsEnabled() {
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

// HandleListRecentJobs returns the most recently created public jobs (GET /jobs/recent).
// No auth required — only IDs and titles are exposed (no owner info).
func HandleListRecentJobs(lister RecentJobLister, limit int) http.HandlerFunc {
	if limit <= 0 {
		limit = 20
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if lister == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]ports.JobMeta{})
			return
		}
		jobs, err := lister.ListRecent(r.Context(), limit)
		if err != nil {
			log.Error().Err(err).Msg("list recent jobs")
			http.Error(w, "failed to list jobs", http.StatusInternalServerError)
			return
		}
		if jobs == nil {
			jobs = []ports.JobMeta{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jobs)
	}
}

// HandleListMyJobs returns the current user's jobs (GET /jobs/mine). Requires auth. Query: limit (default 50).
func HandleListMyJobs(index ports.JobIndex, defaultLimit int) http.HandlerFunc {
	if defaultLimit <= 0 {
		defaultLimit = 50
	}
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if index == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]ports.JobMeta{})
			return
		}
		limit := defaultLimit
		if s := r.URL.Query().Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		jobs, err := index.ListByOwner(r.Context(), u.Sub, limit)
		if err != nil {
			log.Error().Err(err).Str("sub", u.Sub).Msg("list my jobs")
			http.Error(w, "failed to list jobs", http.StatusInternalServerError)
			return
		}
		if jobs == nil {
			jobs = []ports.JobMeta{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jobs)
	}
}

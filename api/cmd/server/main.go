package main

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	appalignment "code-commenter/api/internal/app/alignment"
	geminiadapter "code-commenter/api/internal/adapters/gemini"
	highlightadapter "code-commenter/api/internal/adapters/highlight"
	jobstoreadapter "code-commenter/api/internal/adapters/jobstore"
	storeadapter "code-commenter/api/internal/adapters/store"
	"code-commenter/api/internal/auth"
	"code-commenter/api/internal/config"
	domainalignment "code-commenter/api/internal/domain/alignment"
	"code-commenter/api/internal/gemini"
	"code-commenter/api/internal/handlers"
	"code-commenter/api/internal/jobstore"
	dsindex "code-commenter/api/internal/jobstore/datastore"
	"code-commenter/api/internal/jobstore/firestore"
	"code-commenter/api/internal/ports"
	"code-commenter/api/internal/store"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"})

	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		log.Fatal().Msg("GEMINI_API_KEY or GOOGLE_API_KEY is required")
	}

	ctx := context.Background()
	gc, err := gemini.NewClient(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Fatal().Err(err).Msg("gemini client")
	}
	defer func() { _ = gc.Close() }()

	st := store.New()
	sessionRepo := &storeadapter.Adapter{Store: st}
	genAdapter := &geminiadapter.Adapter{Client: gc, TTSModel: cfg.TTSModel, TimestampModel: cfg.TimestampModel}
	rendererAdapter := highlightadapter.Adapter{}

	var jobStore *jobstore.Client
	if cfg.S3Bucket != "" {
		jobStore, err = jobstore.NewClient(ctx, jobstore.ClientOptions{
			Bucket:    cfg.S3Bucket,
			Region:    cfg.S3Region,
			Endpoint:  cfg.S3Endpoint,
			AccessKey: cfg.S3AccessKey,
			SecretKey: cfg.S3SecretKey,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("jobstore S3 client")
		}
	}
	var jobRepository ports.JobRepository = jobstoreadapter.NoopAdapter{}
	if jobStore != nil {
		jobRepository = &jobstoreadapter.Adapter{Store: jobStore}
	}
	var jobIndex ports.JobIndex
	var dailyQuota ports.DailyQuota
	switch cfg.JobIndexBackend() {
	case "datastore":
		dsIdx, err := dsindex.NewIndex(ctx, cfg.DatastoreProjectID, cfg.DatastoreDatabaseID)
		if err != nil {
			log.Warn().Err(err).Msg("datastore job index unavailable; job listing disabled")
		} else if dsIdx != nil {
			defer func() { _ = dsIdx.Close() }()
			jobIndex = dsIdx
			jobRepository = &jobstoreadapter.CompositeRepository{Repo: jobRepository, Index: jobIndex}
		}
		if cfg.AuthEnabled() {
			dsQuota, err := dsindex.NewQuota(ctx, cfg.DatastoreProjectID, cfg.DatastoreDatabaseID)
			if err != nil {
				log.Warn().Err(err).Msg("datastore quota unavailable; auth disabled")
			} else if dsQuota != nil {
				defer func() { _ = dsQuota.Close() }()
				dailyQuota = dsQuota
			}
		}
	case "firestore":
		fsIndex, err := firestore.NewIndex(ctx, cfg.FirestoreProjectID, cfg.FirestoreDatabaseID)
		if err != nil {
			log.Warn().Err(err).Msg("firestore job index unavailable; job listing disabled")
		} else if fsIndex != nil {
			defer func() { _ = fsIndex.Close() }()
			jobIndex = fsIndex
			jobRepository = &jobstoreadapter.CompositeRepository{Repo: jobRepository, Index: jobIndex}
		}
		if cfg.AuthEnabled() {
			fsQuota, err := firestore.NewQuota(ctx, cfg.FirestoreProjectID, cfg.FirestoreDatabaseID)
			if err != nil {
				log.Warn().Err(err).Msg("firestore quota unavailable; auth disabled")
			} else if fsQuota != nil {
				defer func() { _ = fsQuota.Close() }()
				dailyQuota = fsQuota
			}
		}
	}

	orchestrator := &appalignment.StreamOrchestrator{
		Generation:     genAdapter,
		Audio:          genAdapter,
		Renderer:       rendererAdapter,
		Sessions:       sessionRepo,
		Jobs:           jobRepository,
		Aligner:        domainalignment.Service{},
		TTSPerSegment:  cfg.TTSPerSegment,
	}

	mux := http.NewServeMux()
	allowedOrigins := parseAllowedOrigins(cfg.AllowedOrigins)

	var streamHandler http.Handler = http.HandlerFunc(handlers.HandleStreamTask(orchestrator, cfg.GeminiAPIKey, dailyQuota))
	if cfg.AuthEnabled() {
		oauthCfg := auth.NewOAuthConfig(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.AuthCallbackURL)
		mux.HandleFunc("GET /auth/start", handlers.HandleAuthStart(oauthCfg, cfg.SessionSecret, allowedOrigins))
		mux.HandleFunc("GET /auth/callback", handlers.HandleAuthCallback(oauthCfg, cfg.SessionSecret, allowedOrigins))
		mux.HandleFunc("GET /auth/logout", handlers.HandleLogout(cfg.SessionSecret, allowedOrigins))
		mux.Handle("GET /me", auth.WithSession(cfg.SessionSecret, handlers.HandleMe(dailyQuota)))
		streamHandler = auth.WithSession(cfg.SessionSecret, auth.RequireAuth(handlers.HandleStreamTask(orchestrator, cfg.GeminiAPIKey, dailyQuota)))
		if jobIndex != nil {
			mux.Handle("GET /jobs/mine", auth.WithSession(cfg.SessionSecret, auth.RequireAuth(handlers.HandleListMyJobs(jobIndex, 50))))
		}
	}
	mux.Handle("GET /task/stream", streamHandler)
	if jobRepository.IsEnabled() {
		mux.HandleFunc("GET /jobs/{id}", handlers.HandleGetJob(jobRepository))
	}
	if jobStore != nil {
		mux.HandleFunc("GET /jobs/recent", handlers.HandleListRecentJobs(jobStore, 20))
	}
	mux.HandleFunc("GET /live", handlers.HandleLive(cfg.GeminiAPIKey, cfg.LiveAPIModel))

	// CORS middleware
	handler := corsMiddleware(mux, cfg.AllowedOrigins)

	log.Info().Str("port", cfg.Port).Msg("listening")
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal().Err(err).Msg("server")
	}
}

func corsMiddleware(next http.Handler, origins string) http.Handler {
	allowedOrigins := parseAllowedOrigins(origins)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawOrigin := r.Header.Get("Origin")
		requestOrigin := normalizeOrigin(rawOrigin)
		allowOrigin := matchAllowedOrigin(requestOrigin, rawOrigin, allowedOrigins)
		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			if allowOrigin != "*" {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		appendVary(w.Header(), "Origin")
		if r.Method == http.MethodOptions {
			appendVary(w.Header(), "Access-Control-Request-Method")
			appendVary(w.Header(), "Access-Control-Request-Headers")
			reqMethod := strings.TrimSpace(r.Header.Get("Access-Control-Request-Method"))
			if reqMethod == "" {
				reqMethod = "GET, POST, OPTIONS"
			}
			reqHeaders := strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers"))
			if reqHeaders == "" {
				reqHeaders = "Content-Type, Accept, Accept-Language, Authorization"
			}
			w.Header().Set("Access-Control-Allow-Methods", reqMethod)
			// Echo requested headers for maximum browser compatibility.
			w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func appendVary(h http.Header, value string) {
	existing := h.Values("Vary")
	if len(existing) == 0 {
		h.Set("Vary", value)
		return
	}
	for _, line := range existing {
		for _, token := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(token), value) {
				return
			}
		}
	}
	h.Add("Vary", value)
}

// normalizeOrigin trims and strips trailing slash for consistent comparison.
func normalizeOrigin(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}
	return strings.TrimSuffix(origin, "/")
}

func parseAllowedOrigins(origins string) []string {
	if strings.TrimSpace(origins) == "" {
		return nil
	}
	parts := strings.Split(origins, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := normalizeOrigin(part)
		if origin == "" {
			continue
		}
		out = append(out, origin)
	}
	return out
}

// matchAllowedOrigin returns the origin to send in Access-Control-Allow-Origin.
// It must be the exact request origin (rawOrigin) for the browser to accept it; comparison uses normalized forms.
func matchAllowedOrigin(normalizedRequest, rawOrigin string, allowedOrigins []string) string {
	if len(allowedOrigins) == 0 || normalizedRequest == "" {
		return ""
	}
	for _, origin := range allowedOrigins {
		if origin == "*" {
			return "*"
		}
		if normalizedRequest == origin {
			return rawOrigin
		}
	}
	return ""
}

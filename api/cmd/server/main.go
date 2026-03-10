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
	"code-commenter/api/internal/config"
	domainalignment "code-commenter/api/internal/domain/alignment"
	"code-commenter/api/internal/gemini"
	"code-commenter/api/internal/handlers"
	"code-commenter/api/internal/jobstore"
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
	genAdapter := &geminiadapter.Adapter{Client: gc, TTSModel: cfg.TTSModel}
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

	orchestrator := &appalignment.StreamOrchestrator{
		Generation: genAdapter,
		Audio:      genAdapter,
		Renderer:   rendererAdapter,
		Sessions:   sessionRepo,
		Jobs:       jobRepository,
		Aligner:    domainalignment.Service{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /task", handlers.HandleTask(genAdapter, sessionRepo))
	mux.HandleFunc("GET /task/stream", handlers.HandleStreamTask(orchestrator, cfg.GeminiAPIKey))
	if jobRepository.IsEnabled() {
		mux.HandleFunc("GET /jobs/{id}", handlers.HandleGetJob(jobRepository))
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
		requestOrigin := r.Header.Get("Origin")
		if allowOrigin := matchAllowedOrigin(requestOrigin, allowedOrigins); allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		}
		// Always set Vary so caches do not serve a response (e.g. one without ACAO) for a different Origin.
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins(origins string) []string {
	if strings.TrimSpace(origins) == "" {
		return nil
	}
	parts := strings.Split(origins, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		out = append(out, origin)
	}
	return out
}

func matchAllowedOrigin(requestOrigin string, allowedOrigins []string) string {
	if len(allowedOrigins) == 0 || requestOrigin == "" {
		return ""
	}
	for _, origin := range allowedOrigins {
		if origin == "*" {
			return "*"
		}
		if requestOrigin == origin {
			return requestOrigin
		}
	}
	return ""
}

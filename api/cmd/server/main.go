package main

import (
	"context"
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"code-commenter/api/internal/config"
	"code-commenter/api/internal/gemini"
	"code-commenter/api/internal/handlers"
	"code-commenter/api/internal/jobstore"
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

	mux := http.NewServeMux()
	mux.HandleFunc("POST /task", handlers.HandleTask(gc, st))
	// jobStore may be nil when S3 is not configured; stream task skips S3 upload in that case.
	mux.HandleFunc("GET /task/stream", handlers.HandleStreamTask(gc, st, cfg.GeminiAPIKey, cfg.LiveAPIModel, cfg.TTSModel, jobStore))
	mux.HandleFunc("POST /task/{id}/change", handlers.HandleChange(gc, st))
	if jobStore != nil {
		mux.HandleFunc("GET /jobs/{id}", handlers.HandleGetJob(jobStore))
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

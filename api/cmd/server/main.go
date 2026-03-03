package main

import (
	"context"
	"log"
	"net/http"

	"code-commenter/api/internal/config"
	"code-commenter/api/internal/gemini"
	"code-commenter/api/internal/handlers"
	"code-commenter/api/internal/store"
)

func main() {
	cfg := config.Load()
	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY or GOOGLE_API_KEY is required")
	}

	ctx := context.Background()
	gc, err := gemini.NewClient(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Fatalf("gemini client: %v", err)
	}
	defer func() { _ = gc.Close() }()

	st := store.New()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /task", handlers.HandleTask(gc, st))
	mux.HandleFunc("GET /task/stream", handlers.HandleStreamTask(gc, st, cfg.GeminiAPIKey, cfg.LiveAPIModel, cfg.TTSModel))
	mux.HandleFunc("POST /task/{id}/change", handlers.HandleChange(gc, st))
	mux.HandleFunc("GET /live", handlers.HandleLive(cfg.GeminiAPIKey, cfg.LiveAPIModel))

	// CORS middleware
	handler := corsMiddleware(mux, cfg.AllowedOrigins)

	log.Printf("Listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal(err)
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

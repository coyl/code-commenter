package config

import (
	"os"
)

// Config holds application configuration from environment.
type Config struct {
	Port            string // HTTP server port
	GeminiAPIKey    string // GOOGLE_API_KEY or GEMINI_API_KEY for Gemini 3.1 and Live API
	GeminiModel     string // gemini-3-flash-preview for generation
	LiveAPIModel    string // Live API model for voice; fallback to TTS if not available
	TTSModel        string // REST TTS model when Live API unavailable (e.g. gemini-2.5-pro-preview-tts)
	AllowedOrigins  string // CORS origins (comma-separated)
}

// Load reads config from environment.
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-3-flash-preview"
	}
	liveModel := os.Getenv("GEMINI_LIVE_MODEL")
	if liveModel == "" {
		liveModel = "gemini-2.5-flash-native-audio-preview-12-2025"
	}
	ttsModel := os.Getenv("GEMINI_TTS_MODEL")
	if ttsModel == "" {
		ttsModel = "gemini-2.5-flash-preview-tts"
	}
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		origins = "http://localhost:3000"
	}
	return &Config{
		Port:            port,
		GeminiAPIKey:   apiKey,
		GeminiModel:    model,
		LiveAPIModel:   liveModel,
		TTSModel:       ttsModel,
		AllowedOrigins: origins,
	}
}

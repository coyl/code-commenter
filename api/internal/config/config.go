package config

import (
	"os"
)

// Config holds application configuration from environment.
type Config struct {
	Port           string // HTTP server port
	GeminiAPIKey   string // GOOGLE_API_KEY or GEMINI_API_KEY for Gemini 3.1 and Live API
	GeminiModel    string // gemini-3-flash-preview for generation
	LiveAPIModel   string // Live API model for voice; fallback to TTS if not available
	TTSModel       string // REST TTS model when Live API unavailable (e.g. gemini-2.5-pro-preview-tts)
	AllowedOrigins string // CORS origins (comma-separated)
	S3Bucket       string // S3 bucket for job storage (empty = disabled)
	S3Region       string // AWS region for S3 (e.g. us-east-1)
	S3Endpoint     string // Custom S3 endpoint (e.g. MinIO); empty = AWS
	S3AccessKey    string // S3 access key (optional; else default credential chain)
	S3SecretKey    string // S3 secret key (optional)
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
	s3Bucket := os.Getenv("S3_BUCKET")
	s3Region := os.Getenv("AWS_REGION")
	if s3Region == "" {
		s3Region = os.Getenv("S3_REGION")
	}
	if s3Region == "" && s3Bucket != "" {
		s3Region = "us-east-1"
	}
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	return &Config{
		Port:           port,
		GeminiAPIKey:   apiKey,
		GeminiModel:    model,
		LiveAPIModel:   liveModel,
		TTSModel:       ttsModel,
		AllowedOrigins: origins,
		S3Bucket:       s3Bucket,
		S3Region:       s3Region,
		S3Endpoint:     s3Endpoint,
		S3AccessKey:    s3AccessKey,
		S3SecretKey:    s3SecretKey,
	}
}

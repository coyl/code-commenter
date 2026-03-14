package config

import (
	"os"
	"strings"
)

// Config holds application configuration from environment.
type Config struct {
	Port           string // HTTP server port
	GeminiAPIKey   string // GOOGLE_API_KEY or GEMINI_API_KEY for Gemini 3.1 and Live API
	GeminiModel    string // gemini-3-flash-preview for generation
	LiveAPIModel   string // Live API model for voice; fallback to TTS if not available
	TTSModel       string // REST TTS model when Live API unavailable (e.g. gemini-2.5-pro-preview-tts)
	TTSPerSegment  bool   // if true (TTS_PER_SEGMENT=on), one TTS request per segment; default is single batched call
	TimestampModel string // model for audio timestamp detection in batched TTS (default gemini-2.5-flash)
	AllowedOrigins string // CORS origins (comma-separated)
	S3Bucket       string // S3 bucket for job storage (empty = disabled)
	S3Region       string // AWS region for S3 (e.g. us-east-1)
	S3Endpoint     string // Custom S3 endpoint (e.g. MinIO); empty = AWS
	S3AccessKey    string // S3 access key (optional; else default credential chain)
	S3SecretKey    string // S3 secret key (optional)

	// Google OAuth (all required for auth; if SessionSecret empty, auth is disabled)
	GoogleClientID     string // OAuth 2.0 client ID
	GoogleClientSecret string // OAuth 2.0 client secret
	AuthCallbackURL    string // e.g. https://api.example.com/auth/callback
	SessionSecret      string // HMAC key for session cookie (32+ bytes recommended)

	// Firestore (Native) for job index (owner + listing). Empty = disabled.
	FirestoreProjectID  string
	FirestoreDatabaseID string // Database name; empty = "(default)"

	// Datastore (or Firestore in Datastore mode) for job index. When set, used instead of Firestore.
	// Use this when the database is in Datastore mode (Cloud Firestore API not available).
	DatastoreProjectID  string
	DatastoreDatabaseID string // Named database (e.g. code-commenter); empty = "(default)"

	// DisableAuth when true (DISABLE_AUTH=yes): no auth required, no /jobs/mine or auth routes. Default false.
	DisableAuth bool
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
	ttsPerSegment := false
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TTS_PER_SEGMENT"))) {
	case "on", "1", "true":
		ttsPerSegment = true
	}
	timestampModel := os.Getenv("TIMESTAMP_MODEL")
	if timestampModel == "" {
		timestampModel = "gemini-2.5-flash"
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
		s3Region = ""
	}
	s3Endpoint := os.Getenv("S3_ENDPOINT")
	s3AccessKey := os.Getenv("S3_ACCESS_KEY")
	s3SecretKey := os.Getenv("S3_SECRET_KEY")
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	authCallbackURL := os.Getenv("AUTH_CALLBACK_URL")
	sessionSecret := os.Getenv("SESSION_SECRET")
	firestoreProjectID := os.Getenv("FIRESTORE_PROJECT_ID")
	firestoreDatabaseID := strings.TrimSpace(os.Getenv("FIRESTORE_DATABASE_ID"))
	datastoreProjectID := os.Getenv("DATASTORE_PROJECT_ID")
	datastoreDatabaseID := strings.TrimSpace(os.Getenv("DATASTORE_DATABASE_ID"))
	disableAuth := false
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DISABLE_AUTH"))) {
	case "yes", "1", "on", "true":
		disableAuth = true
	}
	return &Config{
		Port:           port,
		GeminiAPIKey:   apiKey,
		GeminiModel:    model,
		LiveAPIModel:   liveModel,
		TTSModel:       ttsModel,
		TTSPerSegment:  ttsPerSegment,
		TimestampModel: timestampModel,
		AllowedOrigins: origins,
		S3Bucket:       s3Bucket,
		S3Region:       s3Region,
		S3Endpoint:         s3Endpoint,
		S3AccessKey:         s3AccessKey,
		S3SecretKey:         s3SecretKey,
		GoogleClientID:     googleClientID,
		GoogleClientSecret: googleClientSecret,
		AuthCallbackURL:    authCallbackURL,
		SessionSecret:      sessionSecret,
		FirestoreProjectID:  firestoreProjectID,
		FirestoreDatabaseID: firestoreDatabaseID,
		DatastoreProjectID:   datastoreProjectID,
		DatastoreDatabaseID: datastoreDatabaseID,
		DisableAuth:         disableAuth,
	}
}

// JobIndexBackend returns "datastore", "firestore", or "" if no job index is configured.
func (c *Config) JobIndexBackend() string {
	if c.DatastoreProjectID != "" {
		return "datastore"
	}
	if c.FirestoreProjectID != "" {
		return "firestore"
	}
	return ""
}

// AuthEnabled returns true when Google OAuth and session are configured and DISABLE_AUTH is not set.
func (c *Config) AuthEnabled() bool {
	if c.DisableAuth {
		return false
	}
	return c.SessionSecret != "" && c.GoogleClientID != "" && c.GoogleClientSecret != "" && c.AuthCallbackURL != ""
}

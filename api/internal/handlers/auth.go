package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"code-commenter/api/internal/auth"
)

// HandleAuthStart redirects to Google OAuth. Query param "redirect" is the URL to send the user to after login.
func HandleAuthStart(cfg *auth.OAuthConfig, allowedOrigins []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.ClientID == "" || cfg.CallbackURL == "" {
			http.Error(w, "auth not configured", http.StatusServiceUnavailable)
			return
		}
		redirect := r.URL.Query().Get("redirect")
		redirect = auth.RedirectSafe(redirect, allowedOrigins, auth.DefaultRedirect(allowedOrigins))
		state := auth.StateEncode(redirect)
		url := cfg.AuthCodeURL(state)
		log.Debug().Str("redirect", redirect).Msg("auth start")
		http.Redirect(w, r, url, http.StatusFound)
	}
}

// HandleAuthCallback exchanges the code, sets the session cookie, and redirects to the URL from state.
func HandleAuthCallback(cfg *auth.OAuthConfig, sessionSecret string, allowedOrigins []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessionSecret == "" || cfg.ClientID == "" {
			http.Error(w, "auth not configured", http.StatusServiceUnavailable)
			return
		}
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		redirect, err := auth.StateDecode(state)
		if err != nil {
			redirect = auth.DefaultRedirect(allowedOrigins)
		} else {
			redirect = auth.RedirectSafe(redirect, allowedOrigins, auth.DefaultRedirect(allowedOrigins))
		}
		user, err := cfg.ExchangeCode(r.Context(), code)
		if err != nil {
			log.Error().Err(err).Msg("auth callback exchange")
			http.Error(w, "login failed", http.StatusUnauthorized)
			return
		}
		auth.SetSession(w, r, sessionSecret, user)
		log.Info().Str("sub", user.Sub).Str("email", user.Email).Msg("user signed in")
		auth.RedirectTo(w, r, redirect, allowedOrigins)
	}
}

// HandleLogout clears the session cookie and redirects.
func HandleLogout(sessionSecret string, allowedOrigins []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth.ClearSession(w, r)
		redirect := r.URL.Query().Get("redirect")
		redirect = auth.RedirectSafe(redirect, allowedOrigins, auth.DefaultRedirect(allowedOrigins))
		auth.RedirectTo(w, r, redirect, allowedOrigins)
	}
}

// HandleMe returns the current user as JSON or 401 if not signed in. Requires WithSession.
func HandleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"sub": u.Sub, "email": u.Email})
	}
}

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"code-commenter/api/internal/auth"
	"code-commenter/api/internal/ports"
)

// HandleAuthStart redirects to Google OAuth. Query param "redirect" is the URL to send the user to after login.
// Sets a signed state cookie (CSRF token) and includes it in the OAuth state so the callback is bound to this browser.
func HandleAuthStart(cfg *auth.OAuthConfig, sessionSecret string, allowedOrigins []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.ClientID == "" || cfg.CallbackURL == "" {
			http.Error(w, "auth not configured", http.StatusServiceUnavailable)
			return
		}
		redirect := r.URL.Query().Get("redirect")
		redirect = auth.RedirectSafe(redirect, allowedOrigins, auth.DefaultRedirect(allowedOrigins))
		csrfToken, err := auth.GenerateStateToken()
		if err != nil {
			log.Error().Err(err).Msg("auth start: generate state token")
			http.Error(w, "auth error", http.StatusInternalServerError)
			return
		}
		auth.SetStateCookie(w, r, sessionSecret, csrfToken)
		state := auth.StateEncode(csrfToken, redirect)
		url := cfg.AuthCodeURL(state)
		log.Debug().Str("redirect", redirect).Msg("auth start")
		http.Redirect(w, r, url, http.StatusFound)
	}
}

// HandleAuthCallback exchanges the code, sets the session cookie, and redirects to the URL from state.
// Verifies the state parameter contains a CSRF token that matches the state cookie (RFC 6749 §10.12).
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
		csrfToken, redirect, err := auth.StateDecode(state)
		if err != nil || !auth.VerifyStateCookie(r, sessionSecret, csrfToken) {
			auth.ClearStateCookie(w, r)
			log.Warn().Err(err).Msg("auth callback: invalid or missing state (possible CSRF)")
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}
		auth.ClearStateCookie(w, r)
		redirect = auth.RedirectSafe(redirect, allowedOrigins, auth.DefaultRedirect(allowedOrigins))
		user, err := cfg.ExchangeCode(r.Context(), code)
		if err != nil {
			log.Error().Err(err).Msg("auth callback exchange")
			http.Error(w, "login failed", http.StatusUnauthorized)
			return
		}
		auth.SetSession(w, r, sessionSecret, user)
		log.Info().Str("sub", user.Sub).Str("email", user.Email).Msg("user signed in")
		token := auth.GenerateSessionToken(sessionSecret, user)
		callbackURL := auth.BuildTokenCallbackURL(redirect, token)
		auth.RedirectTo(w, r, callbackURL, allowedOrigins)
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
// When quota is non-nil, includes quotaRemaining in the response.
func HandleMe(quota ports.DailyQuota) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		if u == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		resp := map[string]interface{}{
			"sub":   u.Sub,
			"email": u.Email,
		}
		if quota != nil {
			count, err := quota.GetTodayCount(r.Context(), u.Sub)
			if err != nil {
				log.Error().Err(err).Str("sub", u.Sub).Msg("quota check in /me")
			} else {
				remaining := ports.DailyGenerationLimit - count
				if remaining < 0 {
					remaining = 0
				}
				resp["quotaRemaining"] = remaining
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

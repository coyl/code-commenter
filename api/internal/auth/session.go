package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"code-commenter/api/internal/ports"
)

const sessionCookieName = "codecommenter_session"
const sessionMaxAge = 7 * 24 * 3600 // 7 days

// sessionPayload is stored in the cookie (base64 JSON + HMAC).
type sessionPayload struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Expires int64  `json:"exp"`
}

// isSecureRequest returns true when the request is over HTTPS (direct or behind a proxy).
func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

// SetSession writes a signed session cookie with the user.
// SameSite=None is required so the cookie is sent on cross-site requests (e.g. web app → API).
// Secure is set from the request (HTTPS or X-Forwarded-Proto).
func SetSession(w http.ResponseWriter, r *http.Request, secret string, u *ports.UserInfo) {
	if secret == "" || u == nil {
		return
	}
	payload := sessionPayload{
		Sub:     u.Sub,
		Email:   u.Email,
		Expires: time.Now().Unix() + sessionMaxAge,
	}
	raw, _ := json.Marshal(payload)
	b64 := base64.RawURLEncoding.EncodeToString(raw)
	sig := signSession(secret, b64)
	value := b64 + "." + sig
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		SameSite: http.SameSiteNoneMode, // required for cross-site send (web → API)
		Secure:   isSecureRequest(r),
		HttpOnly: true,
	})
}

// ClearSession removes the session cookie. Secure/SameSite must match SetSession so the browser clears it.
func ClearSession(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		SameSite: http.SameSiteNoneMode,
		Secure:   isSecureRequest(r),
		HttpOnly: true,
	})
}

// FromRequest returns the user from the request's session cookie, or nil.
func FromRequest(r *http.Request, secret string) *ports.UserInfo {
	if secret == "" {
		return nil
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return nil
	}
	b64, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(signSession(secret, b64)), []byte(sig)) {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(b64)
	if err != nil {
		return nil
	}
	var p sessionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil
	}
	if p.Expires < time.Now().Unix() || p.Sub == "" {
		return nil
	}
	return &ports.UserInfo{Sub: p.Sub, Email: p.Email}
}

func signSession(secret, payload string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// RedirectURL returns the URL to redirect to after login (from query or default).
func RedirectURL(r *http.Request, defaultPath string) string {
	if s := strings.TrimSpace(r.URL.Query().Get("redirect")); s != "" && !strings.HasPrefix(s, "//") {
		return s
	}
	return defaultPath
}

// RedirectTo writes a 302 to the given URL. SafeRedirect restricts to allowed origins.
func RedirectTo(w http.ResponseWriter, r *http.Request, targetURL string, allowedOrigins []string) {
	if targetURL == "" {
		targetURL = "/"
	}
	// Restrict redirect to allowed origins to avoid open redirect
	if allowedOrigins != nil {
		allowed := make(map[string]bool)
		for _, o := range allowedOrigins {
			o = strings.TrimSuffix(strings.TrimSpace(o), "/")
			if o != "" {
				allowed[o] = true
			}
		}
		// targetURL must start with one of the allowed origins
		ok := false
		for o := range allowed {
			if targetURL == o || strings.HasPrefix(targetURL, o+"/") {
				ok = true
				break
			}
		}
		if !ok {
			targetURL = allowedOrigins[0]
			if strings.HasSuffix(targetURL, "/") {
				targetURL = strings.TrimSuffix(targetURL, "/")
			}
		}
	}
	http.Redirect(w, r, targetURL, http.StatusFound)
}

// ParseAllowedOrigins returns a slice of normalized origins (caller passes strings.Split(AllowedOrigins, ",")).
func ParseAllowedOrigins(origins string) []string {
	if strings.TrimSpace(origins) == "" {
		return nil
	}
	parts := strings.Split(origins, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		o := strings.TrimSuffix(strings.TrimSpace(p), "/")
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}

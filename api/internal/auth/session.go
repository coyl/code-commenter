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
// When the request is HTTPS: SameSite=None and Secure=true so the cookie is sent on cross-site
// requests (e.g. web app on one domain → API on another). Browsers require Secure when SameSite=None.
// When the request is HTTP (e.g. local dev): SameSite=Lax and Secure=false so the cookie is
// stored and sent; same-origin (e.g. localhost:3010 → localhost:8080) still works as same site.
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
	secure := isSecureRequest(r)
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
	}
	setCrossSiteAttrs(cookie, secure)
	http.SetCookie(w, cookie)
}

// ClearSession removes the session cookie. Secure/SameSite must match SetSession so the browser clears it.
func ClearSession(w http.ResponseWriter, r *http.Request) {
	secure := isSecureRequest(r)
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	setCrossSiteAttrs(cookie, secure)
	http.SetCookie(w, cookie)
}

// GenerateSessionToken creates a signed session token for the given user.
// The format is identical to the session cookie value (base64-JSON + "." + HMAC).
func GenerateSessionToken(secret string, u *ports.UserInfo) string {
	if secret == "" || u == nil {
		return ""
	}
	payload := sessionPayload{
		Sub:     u.Sub,
		Email:   u.Email,
		Expires: time.Now().Unix() + sessionMaxAge,
	}
	raw, _ := json.Marshal(payload)
	b64 := base64.RawURLEncoding.EncodeToString(raw)
	sig := signSession(secret, b64)
	return b64 + "." + sig
}

// FromTokenString verifies a raw session token string and returns the user, or nil.
func FromTokenString(token, secret string) *ports.UserInfo {
	if secret == "" || token == "" {
		return nil
	}
	parts := strings.SplitN(token, ".", 2)
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

// FromRequest returns the user from the request. Checks (in order):
//  1. Authorization: Bearer <token> header
//  2. ?token= query parameter (for WebSocket upgrade where custom headers aren't supported)
//  3. Session cookie
func FromRequest(r *http.Request, secret string) *ports.UserInfo {
	if secret == "" {
		return nil
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if u := FromTokenString(strings.TrimPrefix(h, "Bearer "), secret); u != nil {
			return u
		}
	}
	if t := r.URL.Query().Get("token"); t != "" {
		if u := FromTokenString(t, secret); u != nil {
			return u
		}
	}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	return FromTokenString(c.Value, secret)
}

// setCrossSiteAttrs sets Secure, SameSite, and Partitioned on a cookie.
// HTTPS: SameSite=None + Secure + Partitioned (CHIPS) so the cookie works cross-site even when
// third-party cookies are blocked (e.g. Chrome incognito).
// HTTP (local dev): SameSite=Lax, no Secure/Partitioned.
func setCrossSiteAttrs(c *http.Cookie, secure bool) {
	if secure {
		c.SameSite = http.SameSiteNoneMode
		c.Secure = true
		c.Partitioned = true
	} else {
		c.SameSite = http.SameSiteLaxMode
		c.Secure = false
	}
}

func signSession(secret, payload string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
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
			if len(allowedOrigins) > 0 {
				targetURL = strings.TrimSuffix(allowedOrigins[0], "/")
			} else {
				targetURL = "/"
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

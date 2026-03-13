package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"code-commenter/api/internal/ports"
)

const oauthStateCookieName = "codecommenter_oauth_state"
const oauthStateMaxAge = 600 // 10 minutes

// GoogleUserInfo from https://www.googleapis.com/oauth2/v2/userinfo
type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
}

// OAuthConfig holds Google OAuth2 config.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	CallbackURL  string
}

// NewOAuthConfig builds config from env-style values.
func NewOAuthConfig(clientID, clientSecret, callbackURL string) *OAuthConfig {
	return &OAuthConfig{
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(clientSecret),
		CallbackURL:  strings.TrimSuffix(strings.TrimSpace(callbackURL), "/"),
	}
}

// AuthCodeURL returns the URL to redirect the user to for Google sign-in.
// state should be the redirect URL (frontend) to send the user back to after login.
func (c *OAuthConfig) AuthCodeURL(state string) string {
	cfg := c.oauth2Config()
	if state == "" {
		state = "/"
	}
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "select_account"))
}

func (c *OAuthConfig) oauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.CallbackURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// ExchangeCode exchanges the auth code for tokens and returns the user info.
func (c *OAuthConfig) ExchangeCode(ctx context.Context, code string) (*ports.UserInfo, error) {
	cfg := c.oauth2Config()
	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}
	return c.userInfoFromToken(ctx, tok)
}

func (c *OAuthConfig) userInfoFromToken(ctx context.Context, tok *oauth2.Token) (*ports.UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	tok.SetAuthHeader(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo status %d: %s", resp.StatusCode, string(body))
	}
	var u GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("userinfo decode: %w", err)
	}
	if u.ID == "" {
		return nil, fmt.Errorf("userinfo missing id")
	}
	return &ports.UserInfo{
		Sub:   u.ID,
		Email: u.Email,
	}, nil
}

// stateSeparator separates CSRF token from redirect in encoded state. Must not appear in redirect URLs.
const stateSeparator = "|"

// GenerateStateToken returns a cryptographically random token for OAuth state (CSRF binding).
func GenerateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// StateEncode encodes CSRF token and redirect URL for use as OAuth state (RFC 6749 §10.12).
// The token binds the callback to the browser that started the flow.
func StateEncode(csrfToken, redirectURL string) string {
	payload := csrfToken + stateSeparator + redirectURL
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// StateDecode decodes the OAuth state into CSRF token and redirect URL.
// Returns token, redirect, and error. Caller must verify token matches the state cookie.
func StateDecode(state string) (csrfToken, redirect string, err error) {
	b, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", "", err
	}
	s := string(b)
	idx := strings.Index(s, stateSeparator)
	if idx < 0 {
		return "", "", fmt.Errorf("invalid state format")
	}
	return s[:idx], s[idx+len(stateSeparator):], nil
}

// RedirectSafe returns redirect if it is allowed by allowedOrigins, else defaultOrigin.
func RedirectSafe(redirect string, allowedOrigins []string, defaultOrigin string) string {
	redirect = strings.TrimSpace(redirect)
	if redirect == "" {
		return defaultOrigin
	}
	for _, o := range allowedOrigins {
		o = strings.TrimSuffix(strings.TrimSpace(o), "/")
		if o != "" && (redirect == o || strings.HasPrefix(redirect, o+"/")) {
			return redirect
		}
	}
	return defaultOrigin
}

// DefaultRedirect returns the first allowed origin as default.
func DefaultRedirect(allowedOrigins []string) string {
	for _, o := range allowedOrigins {
		o = strings.TrimSuffix(strings.TrimSpace(o), "/")
		if o != "" {
			return o
		}
	}
	return "/"
}

// SetStateCookie sets a signed cookie containing the CSRF token for the OAuth flow.
// Same Secure/SameSite rules as session so the cookie is sent on the callback.
func SetStateCookie(w http.ResponseWriter, r *http.Request, secret, token string) {
	if secret == "" || token == "" {
		return
	}
	sig := signState(secret, token)
	value := token + "." + sig
	secure := isSecureRequest(r)
	cookie := &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   oauthStateMaxAge,
		HttpOnly: true,
	}
	if secure {
		cookie.SameSite = http.SameSiteNoneMode
		cookie.Secure = true
	} else {
		cookie.SameSite = http.SameSiteLaxMode
		cookie.Secure = false
	}
	http.SetCookie(w, cookie)
}

// VerifyStateCookie returns true if the request has a valid state cookie whose token matches tokenFromState.
func VerifyStateCookie(r *http.Request, secret, tokenFromState string) bool {
	if secret == "" || tokenFromState == "" {
		return false
	}
	c, err := r.Cookie(oauthStateCookieName)
	if err != nil || c.Value == "" {
		return false
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	token, sig := parts[0], parts[1]
	if !hmac.Equal([]byte(signState(secret, token)), []byte(sig)) {
		return false
	}
	return hmac.Equal([]byte(token), []byte(tokenFromState))
}

// ClearStateCookie removes the OAuth state cookie after use.
func ClearStateCookie(w http.ResponseWriter, r *http.Request) {
	secure := isSecureRequest(r)
	cookie := &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	if secure {
		cookie.SameSite = http.SameSiteNoneMode
		cookie.Secure = true
	} else {
		cookie.SameSite = http.SameSiteLaxMode
		cookie.Secure = false
	}
	http.SetCookie(w, cookie)
}

func signState(secret, payload string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

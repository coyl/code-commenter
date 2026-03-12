package auth

import (
	"context"
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

// StateEncode encodes the redirect URL for use as OAuth state (to avoid open redirect we only store path or allow origin).
func StateEncode(redirectURL string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(redirectURL))
}

// StateDecode decodes the OAuth state back to redirect URL.
func StateDecode(state string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", err
	}
	return string(b), nil
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

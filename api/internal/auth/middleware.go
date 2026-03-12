package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"code-commenter/api/internal/ports"
)

type contextKey struct{ name string }

var userContextKey = &contextKey{"user"}

// WithUser returns a context with the user attached.
func WithUser(ctx context.Context, u *ports.UserInfo) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// UserFromContext returns the user from the request context (set by WithSession).
func UserFromContext(ctx context.Context) *ports.UserInfo {
	u, _ := ctx.Value(userContextKey).(*ports.UserInfo)
	return u
}

// WithSession loads the session and sets the user in context, then calls next.
func WithSession(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := FromRequest(r, secret)
		ctx := WithUser(r.Context(), u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuth returns a handler that responds 401 if no user is in context, else calls next.
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
			return
		}
		next(w, r)
	}
}
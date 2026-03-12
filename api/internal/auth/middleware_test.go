package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code-commenter/api/internal/ports"
)

func TestRequireAuth_NoUser(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := WithSession("secret", RequireAuth(next))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireAuth_WithUser(t *testing.T) {
	secret := "test-secret-at-least-32-bytes-long"
	u := &ports.UserInfo{Sub: "sub1", Email: "a@b.com"}
	wSet := httptest.NewRecorder()
	SetSession(wSet, nil, secret, u)
	cookie := wSet.Result().Cookies()[0]

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := UserFromContext(r.Context())
		if got == nil || got.Sub != u.Sub {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := WithSession(secret, RequireAuth(next))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

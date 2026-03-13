package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"code-commenter/api/internal/ports"
)

func TestSetSessionAndFromRequest(t *testing.T) {
	secret := "test-secret-at-least-32-bytes-long"
	u := &ports.UserInfo{Sub: "user123", Email: "u@example.com"}

	w := httptest.NewRecorder()
	SetSession(w, nil, secret, u)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	got := FromRequest(req, secret)
	if got == nil {
		t.Fatal("FromRequest returned nil")
	}
	if got.Sub != u.Sub || got.Email != u.Email {
		t.Fatalf("got %+v", got)
	}
}

func TestFromRequest_NoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	got := FromRequest(req, "secret")
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestFromRequest_EmptySecret(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "x.y"})
	got := FromRequest(req, "")
	if got != nil {
		t.Fatalf("expected nil when secret empty, got %+v", got)
	}
}

func TestFromRequest_InvalidSignature(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "eyJzdWIiOiJ4In0.wrongsig"})
	got := FromRequest(req, "secret")
	if got != nil {
		t.Fatalf("expected nil for bad signature, got %+v", got)
	}
}

func TestClearSession(t *testing.T) {
	w := httptest.NewRecorder()
	ClearSession(w, nil)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Fatalf("expected MaxAge -1, got %d", cookies[0].MaxAge)
	}
}

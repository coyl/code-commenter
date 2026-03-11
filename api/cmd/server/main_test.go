package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSPreflightEchoesRequestedHeaders(t *testing.T) {
	h := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not run for preflight")
	}), "https://code-commenter-web.example.com")

	req := httptest.NewRequest(http.MethodOptions, "/jobs/123", nil)
	req.Header.Set("Origin", "https://code-commenter-web.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "accept, cache-control, pragma, priority, sec-ch-ua")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://code-commenter-web.example.com" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got != "accept, cache-control, pragma, priority, sec-ch-ua" {
		t.Fatalf("unexpected allow headers: %q", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "GET" {
		t.Fatalf("unexpected allow methods: %q", got)
	}
}

func TestCORSOriginTrailingSlashMatches(t *testing.T) {
	h := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "https://code-commenter-web.example.com/")

	req := httptest.NewRequest(http.MethodGet, "/jobs/123", nil)
	req.Header.Set("Origin", "https://code-commenter-web.example.com")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://code-commenter-web.example.com" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
}

func TestAppendVaryNoDuplicates(t *testing.T) {
	h := http.Header{}
	appendVary(h, "Origin")
	appendVary(h, "Origin")
	appendVary(h, "Access-Control-Request-Headers")

	values := h.Values("Vary")
	if len(values) != 2 {
		t.Fatalf("expected 2 vary values, got %d (%v)", len(values), values)
	}
}

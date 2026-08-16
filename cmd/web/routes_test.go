package main

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newRoutedApp() http.Handler {
	app := &application{
		logger:        slog.New(slog.DiscardHandler),
		csp:           buildCSP("http://localhost:9000/photos"),
		sessionSecret: []byte(testSecret),
	}

	return app.routes()
}

func TestPing(t *testing.T) {
	w := httptest.NewRecorder()
	ping(w, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if w.Code != http.StatusOK {
		t.Errorf("got status %d; want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "OK" {
		t.Errorf("got body %q; want OK", w.Body.String())
	}
}

func TestRoutesPingThroughTheChain(t *testing.T) {
	w := httptest.NewRecorder()
	newRoutedApp().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "img-src") {
		t.Error("the CSP is missing; commonHeaders is not wrapping the mux")
	}
	if w.Header().Get("X-Frame-Options") != "deny" {
		t.Error("X-Frame-Options is missing from a routed response")
	}
}

func TestRoutesMethodAndPathMismatch(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		want   int
	}{
		{"unknown path", http.MethodGet, "/nope", http.StatusNotFound},
		{"wrong method", http.MethodPost, "/ping", http.StatusMethodNotAllowed},
		{"root is exact", http.MethodGet, "/anything", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			newRoutedApp().ServeHTTP(w, httptest.NewRequest(tt.method, tt.target, nil))

			if w.Code != tt.want {
				t.Errorf("got status %d; want %d", w.Code, tt.want)
			}
		})
	}
}

func TestRoutesGuardEveryWriteEndpoint(t *testing.T) {
	guarded := []struct {
		method string
		target string
	}{
		{http.MethodGet, "/person/1/edit"},
		{http.MethodPost, "/person/1/edit"},
		{http.MethodGet, "/person/1/add"},
		{http.MethodPost, "/person/1/add"},
		{http.MethodGet, "/person/1/delete"},
		{http.MethodPost, "/person/1/delete"},
		{http.MethodPost, "/person/1/photos"},
		{http.MethodGet, "/person/1/relatives"},
		{http.MethodPost, "/person/1/relatives/link"},
		{http.MethodPost, "/person/1/relatives/unlink"},
		{http.MethodGet, "/info"},
	}

	for _, tt := range guarded {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			w := httptest.NewRecorder()
			newRoutedApp().ServeHTTP(w, httptest.NewRequest(tt.method, tt.target, nil))

			if w.Code != http.StatusSeeOther {
				t.Fatalf("got status %d; want %d — an anonymous request must never reach the handler", w.Code, http.StatusSeeOther)
			}
			if got := w.Header().Get("Location"); !strings.HasPrefix(got, "/login?next=") {
				t.Errorf("got redirect %q; want /login?next=...", got)
			}
		})
	}
}

func TestRoutesPublicEndpointsAreReachable(t *testing.T) {
	public := []struct {
		method string
		target string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/person/1"},
		{http.MethodGet, "/person/1/photos"},
		{http.MethodGet, "/map"},
		{http.MethodGet, "/login"},
	}

	for _, tt := range public {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			w := httptest.NewRecorder()
			newRoutedApp().ServeHTTP(w, httptest.NewRequest(tt.method, tt.target, nil))

			if w.Code == http.StatusNotFound || w.Code == http.StatusMethodNotAllowed {
				t.Errorf("got status %d; the route is not registered", w.Code)
			}
			if w.Code == http.StatusSeeOther {
				t.Errorf("got a redirect to %q; this route is public", w.Header().Get("Location"))
			}
		})
	}
}

package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

func requestWithUser(r *http.Request, user models.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

func TestBuildCSP(t *testing.T) {
	tests := []struct {
		name       string
		mediaURL   string
		wantImgSrc string
	}{
		{"dev minio", "http://localhost:9000/photos", "'self' http://localhost:9000"},
		{"supabase public path", "https://abc.supabase.co/storage/v1/object/public/photos", "'self' https://abc.supabase.co"},
		{"host only", "https://cdn.example.com", "'self' https://cdn.example.com"},
		{"empty", "", "'self'"},
		{"unparseable", "://nope", "'self'"},
		{"no scheme or host", "photos", "'self'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csp := buildCSP(tt.mediaURL)

			if !strings.Contains(csp, "img-src "+tt.wantImgSrc+" "+tileHost+";") {
				t.Errorf("got %q; want img-src %q plus the tile host", csp, tt.wantImgSrc)
			}

			for _, directive := range []string{
				"default-src 'self'",
				"script-src 'self' " + leafletHost,
				"style-src 'self' " + leafletHost,
				"frame-ancestors 'none'",
				"base-uri 'self'",
			} {
				if !strings.Contains(csp, directive) {
					t.Errorf("got %q; missing %q", csp, directive)
				}
			}
		})
	}
}

func TestCommonHeaders(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	commonHeaders("img-src 'self'")(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Error("next handler was not called")
	}

	want := map[string]string{
		"Content-Security-Policy": "img-src 'self'",
		"Referrer-Policy":         "origin-when-cross-origin",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "deny",
	}

	for header, value := range want {
		if got := w.Header().Get(header); got != value {
			t.Errorf("got %s: %q; want %q", header, got, value)
		}
	}
}

func TestRequireAuthRedirectsAnonymous(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"plain", "/person/1/edit", "/login?next=%2Fperson%2F1%2Fedit"},
		{"with query", "/person/1/photos?page=2", "/login?next=%2Fperson%2F1%2Fphotos%3Fpage%3D2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{}

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("next handler ran for an anonymous request")
			})

			w := httptest.NewRecorder()
			app.requireAuth(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.uri, nil))

			if w.Code != http.StatusSeeOther {
				t.Fatalf("got status %d; want %d", w.Code, http.StatusSeeOther)
			}
			if got := w.Header().Get("Location"); got != tt.want {
				t.Errorf("got redirect %q; want %q", got, tt.want)
			}
		})
	}
}

func TestRequireAuthAllowsAuthenticated(t *testing.T) {
	app := &application{}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	r := httptest.NewRequest(http.MethodGet, "/person/1/edit", nil)
	r = requestWithUser(r, models.User{ID: 1, Name: "admin"})

	w := httptest.NewRecorder()
	app.requireAuth(next).ServeHTTP(w, r)

	if !called {
		t.Fatal("next handler did not run for an authenticated request")
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("got Cache-Control %q; want no-store — logged-in pages must not be cached", got)
	}
}

func TestAuthenticatePassesAnonymousThrough(t *testing.T) {
	app := newSessionApp(testSecret, false)

	tests := []struct {
		name   string
		cookie *http.Cookie
	}{
		{"no cookie", nil},
		{"garbage cookie", &http.Cookie{Name: sessionCookie, Value: "not-a-session"}},
		{"forged cookie", &http.Cookie{Name: sessionCookie, Value: signedPayload(newSessionApp(otherSecret, false), 1, time.Now().Add(time.Hour))}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true

				_, ok := userFromContext(r)
				if ok {
					t.Error("got a user in context; the request was not authenticated")
				}
			})

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.cookie != nil {
				r.AddCookie(tt.cookie)
			}

			app.authenticate(next).ServeHTTP(httptest.NewRecorder(), r)

			if !called {
				t.Error("next handler did not run; an anonymous request is still a valid request")
			}
		})
	}
}

func TestAuthenticateAttachesTheUser(t *testing.T) {
	app := newTestApp(t)
	id, username := haNewUser(t, app)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		user, ok := userFromContext(r)
		if !ok {
			t.Fatal("got no user in context; a valid session must authenticate the request")
		}
		if user.ID != id {
			t.Errorf("got user %d; want %d", user.ID, id)
		}
		if user.Name != username {
			t.Errorf("got name %q; want %q", user.Name, username)
		}
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(issueCookie(t, app, id))

	w := httptest.NewRecorder()
	app.authenticate(next).ServeHTTP(w, r)

	if !called {
		t.Fatal("next handler did not run")
	}
	if cookies := (&http.Response{Header: w.Header()}).Cookies(); len(cookies) != 0 {
		t.Errorf("got cookies %v; want none — a good session must be left alone", cookies)
	}
}

func TestAuthenticateClearsAStaleCookie(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"deleted user", `DELETE FROM auth_user WHERE id = $1`},
		{"deactivated user", `UPDATE auth_user SET is_active = false WHERE id = $1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newTestApp(t)
			id, _ := haNewUser(t, app)

			cookie := issueCookie(t, app, id)

			_, err := app.users.DB.Exec(context.Background(), tt.query, id)
			if err != nil {
				t.Fatal(err)
			}

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true

				_, ok := userFromContext(r)
				if ok {
					t.Error("got a user in context; the account behind the session is gone")
				}
			})

			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.AddCookie(cookie)

			w := httptest.NewRecorder()
			app.authenticate(next).ServeHTTP(w, r)

			if !called {
				t.Fatal("next handler did not run; a stale session still browses as anonymous")
			}

			got := (&http.Response{Header: w.Header()}).Cookies()
			if len(got) != 1 || got[0].Name != sessionCookie {
				t.Fatalf("got cookies %v; want the session cookie cleared", got)
			}
			if got[0].Value != "" || got[0].MaxAge >= 0 {
				t.Errorf("got cookie %q with MaxAge %d; want an empty value and a negative MaxAge", got[0].Value, got[0].MaxAge)
			}
		})
	}
}

func TestRecoverPanic(t *testing.T) {
	app := &application{logger: slog.New(slog.DiscardHandler)}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	w := httptest.NewRecorder()
	app.recoverPanic(next).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d; want %d", w.Code, http.StatusInternalServerError)
	}
	if got := w.Header().Get("Connection"); got != "close" {
		t.Errorf("got Connection %q; want close", got)
	}
}

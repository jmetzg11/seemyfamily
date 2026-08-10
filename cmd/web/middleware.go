package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"seemyfamily.jmetzg11/internal/models"
)

func buildCSP(mediaURL string) string {
	imgSrc := "'self'"

	u, err := url.Parse(mediaURL)
	if err == nil && u.Host != "" {
		imgSrc += " " + u.Scheme + "://" + u.Host
	}

	return "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self'; " +
		"img-src " + imgSrc + "; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'"
}

func commonHeaders(csp string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", csp)
			w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "deny")

			next.ServeHTTP(w, r)
		})
	}
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := app.readSessionCookie(r)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		user, err := app.users.Get(r.Context(), id)
		if err != nil {
			if !errors.Is(err, models.ErrNoRecord) {
				app.serverError(w, r, err)
				return
			}

			app.clearSessionCookie(w)
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (app *application) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := userFromContext(r)
		if !ok {
			v := url.Values{}
			v.Set("next", r.URL.RequestURI())
			http.Redirect(w, r, "/login?"+v.Encode(), http.StatusSeeOther)
			return
		}

		w.Header().Set("Cache-Control", "no-store")

		next.ServeHTTP(w, r)
	})
}

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			pv := recover()
			if pv != nil {
				w.Header().Set("Connection", "close")
				app.serverError(w, r, fmt.Errorf("%v", pv))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

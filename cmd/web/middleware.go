package main

import (
	"fmt"
	"net/http"
	"net/url"
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

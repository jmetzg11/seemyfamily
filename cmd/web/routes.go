package main

import (
	"net/http"

	"seemyfamily.jmetzg11/ui"
)

func ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(ui.Files))

	mux.HandleFunc("GET /ping", ping)
	mux.HandleFunc("GET /{$}", app.home)
	mux.HandleFunc("GET /person/{id}", app.person)
	mux.HandleFunc("GET /person/{id}/photos", app.gallery)
	mux.HandleFunc("GET /map", app.mapPage)

	mux.HandleFunc("GET /login", app.loginForm)
	mux.HandleFunc("POST /login", app.login)
	mux.HandleFunc("POST /logout", app.logout)

	owner := func(h http.HandlerFunc) http.Handler {
		return app.requireAuth(app.requireOwner(h))
	}

	mux.Handle("GET /person/{id}/edit", owner(app.editForm))
	mux.Handle("POST /person/{id}/edit", owner(app.edit))
	mux.Handle("GET /person/{id}/add", owner(app.addRelativeForm))
	mux.Handle("POST /person/{id}/add", owner(app.addRelative))
	mux.Handle("GET /person/{id}/delete", owner(app.deleteForm))
	mux.Handle("POST /person/{id}/delete", owner(app.delete))
	mux.Handle("POST /person/{id}/photos", owner(app.upload))
	mux.Handle("GET /person/{id}/relatives", owner(app.relatives))
	mux.Handle("POST /person/{id}/relatives/link", owner(app.link))
	mux.Handle("POST /person/{id}/relatives/unlink", owner(app.unlink))

	mux.Handle("GET /info", app.requireAuth(http.HandlerFunc(app.info)))

	csrf := http.NewCrossOriginProtection()

	return app.recoverPanic(commonHeaders(app.csp)(csrf.Handler(app.authenticate(mux))))
}

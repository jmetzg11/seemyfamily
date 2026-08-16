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

	mux.HandleFunc("GET /login", app.loginForm)
	mux.HandleFunc("POST /login", app.login)
	mux.HandleFunc("POST /logout", app.logout)

	mux.Handle("GET /person/{id}/edit", app.requireAuth(http.HandlerFunc(app.editForm)))
	mux.Handle("POST /person/{id}/edit", app.requireAuth(http.HandlerFunc(app.edit)))
	mux.Handle("GET /person/{id}/add", app.requireAuth(http.HandlerFunc(app.addRelativeForm)))
	mux.Handle("POST /person/{id}/add", app.requireAuth(http.HandlerFunc(app.addRelative)))
	mux.Handle("GET /person/{id}/delete", app.requireAuth(http.HandlerFunc(app.deleteForm)))
	mux.Handle("POST /person/{id}/delete", app.requireAuth(http.HandlerFunc(app.delete)))
	mux.Handle("POST /person/{id}/photos", app.requireAuth(http.HandlerFunc(app.upload)))
	mux.Handle("GET /person/{id}/relatives", app.requireAuth(http.HandlerFunc(app.relatives)))
	mux.Handle("POST /person/{id}/relatives/link", app.requireAuth(http.HandlerFunc(app.link)))
	mux.Handle("POST /person/{id}/relatives/unlink", app.requireAuth(http.HandlerFunc(app.unlink)))

	mux.Handle("GET /info", app.requireAuth(http.HandlerFunc(app.info)))

	csrf := http.NewCrossOriginProtection()

	return app.recoverPanic(commonHeaders(app.csp)(csrf.Handler(app.authenticate(mux))))
}

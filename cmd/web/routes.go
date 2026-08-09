package main

import (
	"net/http"

	"seemyfamily.jmetzg11/ui"
)

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.FileServerFS(ui.Files))

	mux.HandleFunc("GET /ping", ping)
	mux.HandleFunc("GET /{$}", app.home)

	return app.recoverPanic(commonHeaders(app.csp)(mux))
}

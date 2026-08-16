package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"os"
)

func mustGetenv(logger *slog.Logger, key string) string {
	value := os.Getenv(key)
	if value == "" {
		logger.Error(key + " is not set")
		os.Exit(1)
	}

	return value
}

func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}

func (app *application) newTemplateData(r *http.Request) templateData {
	data := templateData{
		MediaURL: app.bucket.PublicURL,
	}

	user, ok := userFromContext(r)
	if ok {
		data.UserName = user.Name
		data.IsAuthenticated = true
	}

	return data
}

func (app *application) render(w http.ResponseWriter, r *http.Request, status int, page string, data any) {
	ts, ok := app.templateCache[page]
	if !ok {
		app.serverError(w, r, fmt.Errorf("the template %s does not exist", page))
		return
	}

	buf := new(bytes.Buffer)

	err := ts.ExecuteTemplate(buf, "base", data)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	w.WriteHeader(status)
	buf.WriteTo(w)
}

package main

import (
	"errors"
	"net/http"
	"strings"

	"seemyfamily.jmetzg11/internal/models"
)

func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/"
	}
	return next
}

func (app *application) loginForm(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData(r)
	data.Page = "login"
	data.LoginForm.Next = safeNext(r.URL.Query().Get("next"))

	app.render(w, r, http.StatusOK, "login.html", data)
}

func (app *application) login(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.PostForm.Get("name"))
	next := safeNext(r.PostForm.Get("next"))

	user, err := app.users.Authenticate(r.Context(), name, r.PostForm.Get("password"))
	if err != nil {
		if !errors.Is(err, models.ErrInvalidCredentials) {
			app.serverError(w, r, err)
			return
		}

		data := app.newTemplateData(r)
		data.Page = "login"
		data.LoginForm.Next = next
		data.LoginForm.Name = name
		data.LoginForm.Error = "Username or password is incorrect."

		app.render(w, r, http.StatusUnprocessableEntity, "login.html", data)
		return
	}

	app.setSessionCookie(w, user.ID)

	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	app.clearSessionCookie(w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

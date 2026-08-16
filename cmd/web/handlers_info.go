package main

import (
	"net/http"
)

func (app *application) info(w http.ResponseWriter, r *http.Request) {
	edits, err := app.stats.Edits(r.Context(), 100)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Page = "info"
	data.Edits = edits

	app.render(w, r, http.StatusOK, "info.html", data)
}

package main

import (
	"net/http"
)

func (app *application) mapPage(w http.ResponseWriter, r *http.Request) {
	places, err := app.locations.Places(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	mapData, err := buildMapData(places)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Page = "map"
	data.MapData = mapData
	data.Places = len(places)

	app.render(w, r, http.StatusOK, "map.html", data)
}

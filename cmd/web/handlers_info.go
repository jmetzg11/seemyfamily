package main

import (
	"net/http"
)

var visitorRanges = map[string]struct {
	days       int
	groupSize  int
	labelEvery int
}{
	"week":     {7, 1, 1},
	"month":    {30, 5, 1},
	"halfyear": {180, 30, 1},
}

func (app *application) info(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("range")

	spec, ok := visitorRanges[period]
	if !ok {
		period = "month"
		spec = visitorRanges[period]
	}

	edits, err := app.stats.Edits(r.Context(), 100)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	buckets, err := app.stats.Visitors(r.Context(), spec.days, spec.groupSize)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Page = "info"
	data.Edits = edits
	data.Chart = buildChart(buckets, spec.labelEvery)
	data.Range = period

	app.render(w, r, http.StatusOK, "info.html", data)
}

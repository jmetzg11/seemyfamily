package main

import (
	"net/http"
	"strconv"
)

const rowsPerPage = 20

func ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func (app *application) home(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	search := q.Get("q")

	sort := q.Get("sort")
	if sort == "" {
		sort = "name"
	}

	dir := q.Get("dir")
	if dir != "desc" {
		dir = "asc"
	}

	page, err := strconv.Atoi(q.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	total, err := app.people.Count(r.Context(), search)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	totalPages := (total + rowsPerPage - 1) / rowsPerPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	people, err := app.people.List(r.Context(), search, sort, dir, rowsPerPage, (page-1)*rowsPerPage)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Page = "home"
	data.People = people
	data.Search = search
	data.Sort = sort
	data.Dir = dir
	data.CurrentPage = page
	data.TotalPages = totalPages
	data.Total = total

	app.render(w, r, http.StatusOK, "home.html", data)
}

package main

import (
	"errors"
	"net/http"
	"strconv"

	"seemyfamily.jmetzg11/internal/models"
)

const rowsPerPage = 20

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

func (app *application) person(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	person, err := app.people.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	relations, err := app.people.Relations(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Page = "person"
	data.Person = person
	data.Relations = relations

	app.render(w, r, http.StatusOK, "person.html", data)
}

func (app *application) editForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	person, err := app.people.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	data := app.newTemplateData(r)
	data.Page = "person"
	data.Person = person
	data.PersonForm = newPersonForm(person)

	app.render(w, r, http.StatusOK, "edit.html", data)
}

func (app *application) edit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	user, _ := userFromContext(r)
	form := personFormFrom(r)
	person, ok := form.validate()
	person.ID = id

	if ok {
		err = app.people.Update(r.Context(), person, user.Name)
		switch {
		case err == nil:
			http.Redirect(w, r, "/person/"+strconv.Itoa(id), http.StatusSeeOther)
			return
		case errors.Is(err, models.ErrNoRecord):
			app.notFound(w)
			return
		case errors.Is(err, models.ErrDuplicateName):
			form.Errors["Name"] = "Someone with that name already exists."
		default:
			app.serverError(w, r, err)
			return
		}
	}

	data := app.newTemplateData(r)
	data.Page = "person"
	data.Person = person
	data.PersonForm = form

	app.render(w, r, http.StatusUnprocessableEntity, "edit.html", data)
}

func (app *application) deleteForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	person, err := app.people.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	relations, err := app.people.Relations(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Page = "person"
	data.Person = person
	data.Relations = relations

	app.render(w, r, http.StatusOK, "delete.html", data)
}

func (app *application) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	user, _ := userFromContext(r)

	err = app.people.Delete(r.Context(), id, user.Name)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

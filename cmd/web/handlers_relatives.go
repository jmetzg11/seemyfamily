package main

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"seemyfamily.jmetzg11/internal/models"
)

func (app *application) renderRelatives(w http.ResponseWriter, r *http.Request, id int, form linkForm, status int) {
	person, err := app.people.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	facts, err := app.people.Facts(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	names, err := app.people.Names(r.Context(), id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	data := app.newTemplateData(r)
	data.Page = "person"
	data.Person = person
	data.Facts = facts
	data.Names = names
	data.LinkForm = form

	app.render(w, r, status, "relatives.html", data)
}

func (app *application) relatives(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	app.renderRelatives(w, r, id, linkForm{}, http.StatusOK)
}

func (app *application) link(w http.ResponseWriter, r *http.Request) {
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
	form := linkForm{
		Name:     strings.TrimSpace(r.PostForm.Get("name")),
		Relation: r.PostForm.Get("relation"),
	}

	switch {
	case form.Name == "":
		form.Error = "Choose a person."
	case !slices.Contains(facts, form.Relation):
		form.Error = "Choose how they are related."
	}

	if form.Error == "" {
		err = app.people.Link(r.Context(), id, form.Name, form.Relation, user.Name)
		switch {
		case err == nil:
			http.Redirect(w, r, "/person/"+strconv.Itoa(id)+"/relatives", http.StatusSeeOther)
			return
		case errors.Is(err, models.ErrNoRecord):
			form.Error = "No one on the site is called that."
		case errors.Is(err, models.ErrSelfLink):
			form.Error = "A person cannot be their own relative."
		case errors.Is(err, models.ErrAlreadyLinked):
			form.Error = "That link already exists."
		default:
			app.serverError(w, r, err)
			return
		}
	}

	app.renderRelatives(w, r, id, form, http.StatusUnprocessableEntity)
}

func (app *application) unlink(w http.ResponseWriter, r *http.Request) {
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
	name := r.PostForm.Get("name")
	relation := r.PostForm.Get("relation")

	if !slices.Contains(facts, relation) {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	err = app.people.Unlink(r.Context(), id, name, relation, user.Name)
	if err != nil && !errors.Is(err, models.ErrNoRecord) {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/person/"+strconv.Itoa(id)+"/relatives", http.StatusSeeOther)
}

func (app *application) addRelativeForm(w http.ResponseWriter, r *http.Request) {
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

	app.render(w, r, http.StatusOK, "add.html", data)
}

func (app *application) addRelative(w http.ResponseWriter, r *http.Request) {
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

	person, err := app.people.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrNoRecord) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	user, _ := userFromContext(r)
	form := relativeFormFrom(r)
	relative, ok := form.validate()

	if ok {
		err = app.people.AddRelative(r.Context(), relative, id, form.Relation, user.Name)
		switch {
		case err == nil:
			http.Redirect(w, r, "/person/"+strconv.Itoa(id), http.StatusSeeOther)
			return
		case errors.Is(err, models.ErrDuplicateName):
			form.Person.Errors["Name"] = "Someone with that name already exists."
		default:
			app.serverError(w, r, err)
			return
		}
	}

	data := app.newTemplateData(r)
	data.Page = "person"
	data.Person = person
	data.RelativeForm = form

	app.render(w, r, http.StatusUnprocessableEntity, "add.html", data)
}

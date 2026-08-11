package main

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"seemyfamily.jmetzg11/internal/models"
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

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	app.clearSessionCookie(w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
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

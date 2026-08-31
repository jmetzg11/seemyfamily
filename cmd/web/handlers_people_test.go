package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"seemyfamily.jmetzg11/internal/models"
)

func hpSend(t *testing.T, handler http.HandlerFunc, id string, values url.Values) *httptest.ResponseRecorder {
	t.Helper()

	r := postForm(t, values)
	r.SetPathValue("id", id)

	w := httptest.NewRecorder()
	handler(w, requestWithUser(r, models.User{ID: 1, Name: testUser}))

	return w
}

func hpHome(t *testing.T, app *application, query string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	app.home(w, httptest.NewRequest(http.MethodGet, "/?"+query, nil))

	return w
}

func hpAssertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()

	if w.Code != want {
		t.Fatalf("got status %d; want %d — body: %s", w.Code, want, w.Body)
	}
}

func hpAssertBody(t *testing.T, w *httptest.ResponseRecorder, want, why string) {
	t.Helper()

	if !strings.Contains(w.Body.String(), want) {
		t.Errorf("got a page without %q; want it present — %s", want, why)
	}
}

func hpMissingID(t *testing.T, app *application) int {
	t.Helper()

	var highest int

	err := app.people.DB.QueryRow(context.Background(), `SELECT COALESCE(max(id), 0) FROM api_person`).Scan(&highest)
	if err != nil {
		t.Fatal(err)
	}

	return highest + 1000000
}

func hpExists(t *testing.T, app *application, id int) bool {
	t.Helper()

	var exists bool

	err := app.people.DB.QueryRow(context.Background(), `SELECT exists(SELECT 1 FROM api_person WHERE id = $1)`, id).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}

	return exists
}

func hpLink(t *testing.T, app *application, parentID, childID int) {
	t.Helper()

	_, err := app.people.DB.Exec(context.Background(),
		`INSERT INTO api_parentchild (parent_id, child_id) VALUES ($1, $2)`, parentID, childID)
	if err != nil {
		t.Fatal(err)
	}
}

func hpForgetHistory(t *testing.T, app *application, name string) {
	t.Helper()

	t.Cleanup(func() {
		_, err := app.people.DB.Exec(context.Background(), `DELETE FROM api_history WHERE recipient = $1`, name)
		if err != nil {
			t.Error(err)
		}
	})
}

func hpSetBirthyear(t *testing.T, app *application, id int, name string, year int) {
	t.Helper()

	err := app.people.Update(context.Background(),
		models.Person{Summary: models.Summary{ID: id, Name: name, Birthyear: year}}, testUser)
	if err != nil {
		t.Fatal(err)
	}
}

func hpSharedPrefix(a, b string) string {
	n := min(len(a), len(b))

	for i := range n {
		if a[i] != b[i] {
			return a[:i]
		}
	}

	return a[:n]
}

var hpPageRe = regexp.MustCompile(`Page (\d+) of (\d+)`)

var hpRowRe = regexp.MustCompile(`data-href="/person/(\d+)"`)

func hpPageNumbers(t *testing.T, w *httptest.ResponseRecorder) (int, int) {
	t.Helper()

	m := hpPageRe.FindStringSubmatch(w.Body.String())
	if m == nil {
		t.Fatalf("got a page with no pager; want a \"Page X of Y\" — body: %s", w.Body)
	}

	current, _ := strconv.Atoi(m[1])
	total, _ := strconv.Atoi(m[2])

	return current, total
}

func hpRowIDs(w *httptest.ResponseRecorder) []string {
	var ids []string

	for _, m := range hpRowRe.FindAllStringSubmatch(w.Body.String(), -1) {
		ids = append(ids, m[1])
	}

	return ids
}

func TestHomeListsPeople(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	name := testPersonName(t, app, id)

	w := hpHome(t, app, "q="+url.QueryEscape(name))

	hpAssertStatus(t, w, http.StatusOK)
	hpAssertBody(t, w, name, "the person matching the search must be listed")
	hpAssertBody(t, w, `href="/person/`+strconv.Itoa(id)+`"`, "every row links to the profile")
}

func TestHomeSearchFiltersTheList(t *testing.T) {
	app := newTestApp(t)
	wanted := testPersonName(t, app, newTestPerson(t, app))
	other := testPersonName(t, app, newTestPerson(t, app))

	w := hpHome(t, app, "q="+url.QueryEscape(wanted))

	hpAssertStatus(t, w, http.StatusOK)
	hpAssertBody(t, w, wanted, "the searched-for person is kept")
	hpAssertBody(t, w, `value="`+wanted+`"`, "the search box keeps what was typed")

	if strings.Contains(w.Body.String(), other) {
		t.Errorf("got %q in the list; want it filtered out by q=%q", other, wanted)
	}
}

func TestHomeSearchWithNoMatches(t *testing.T) {
	app := newTestApp(t)

	w := hpHome(t, app, "q=no-such-person-anywhere-in-this-family")

	hpAssertStatus(t, w, http.StatusOK)
	hpAssertBody(t, w, "No one found.", "an empty result set still renders the table")

	current, total := hpPageNumbers(t, w)
	if current != 1 || total != 1 {
		t.Errorf("got page %d of %d; want 1 of 1 — there is always at least one page", current, total)
	}
}

func TestHomeSortAndDirection(t *testing.T) {
	app := newTestApp(t)
	first := testPersonName(t, app, newTestPerson(t, app))
	second := testPersonName(t, app, newTestPerson(t, app))
	search := url.QueryEscape(hpSharedPrefix(first, second))

	tests := []struct {
		name    string
		query   string
		inOrder bool
		wantDir string
	}{
		{"defaults to name ascending", "", true, "asc"},
		{"name ascending", "sort=name&dir=asc", true, "asc"},
		{"name descending", "sort=name&dir=desc", false, "desc"},
		{"unknown sort falls back to name", "sort=favourite&dir=asc", true, "asc"},
		{"unknown dir falls back to ascending", "sort=name&dir=sideways", true, "asc"},
		{"unknown sort and dir fall back to name ascending", "sort=favourite&dir=sideways", true, "asc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := hpHome(t, app, tt.query+"&q="+search)
			hpAssertStatus(t, w, http.StatusOK)

			body := w.Body.String()

			a, b := strings.Index(body, first), strings.Index(body, second)
			if a < 0 || b < 0 {
				t.Fatalf("got a page missing %q or %q; both were created and both match the search", first, second)
			}
			if (a < b) != tt.inOrder {
				t.Errorf("got %q at %d and %q at %d; want %q first", first, a, second, b, map[bool]string{true: first, false: second}[tt.inOrder])
			}

			hpAssertBody(t, w, `name="dir" value="`+tt.wantDir+`"`, "the toolbar carries the direction actually used")
		})
	}
}

func TestHomeSortsByBirthyear(t *testing.T) {
	app := newTestApp(t)
	olderID, youngerID := newTestPerson(t, app), newTestPerson(t, app)
	older, younger := testPersonName(t, app, olderID), testPersonName(t, app, youngerID)
	search := url.QueryEscape(hpSharedPrefix(older, younger))

	hpSetBirthyear(t, app, youngerID, younger, 1900)
	hpSetBirthyear(t, app, olderID, older, 1800)

	tests := []struct {
		name  string
		dir   string
		first string
	}{
		{"ascending puts the oldest first", "asc", older},
		{"descending puts the youngest first", "desc", younger},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := hpHome(t, app, "sort=birthyear&dir="+tt.dir+"&q="+search)
			hpAssertStatus(t, w, http.StatusOK)
			hpAssertBody(t, w, `name="sort" value="birthyear"`, "the chosen column is carried on the page")

			body := w.Body.String()

			a, b := strings.Index(body, older), strings.Index(body, younger)
			if a < 0 || b < 0 {
				t.Fatalf("got a page missing %q or %q; both were created and both match the search", older, younger)
			}
			if (a < b) != (tt.first == older) {
				t.Errorf("got %q at %d and %q at %d; want %q first — birthyear order, not name order", older, a, younger, b, tt.first)
			}
		})
	}
}

func TestHomePageParameter(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name     string
		page     string
		wantLast bool
	}{
		{"absent", "", false},
		{"blank", "page=", false},
		{"first", "page=1", false},
		{"below one", "page=0", false},
		{"negative", "page=-3", false},
		{"not a number", "page=twelve", false},
		{"past the end", "page=99999", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := hpHome(t, app, tt.page)
			hpAssertStatus(t, w, http.StatusOK)

			current, total := hpPageNumbers(t, w)

			want := 1
			if tt.wantLast {
				want = total
			}
			if current != want {
				t.Errorf("got page %d of %d; want page %d", current, total, want)
			}
			if total < 1 {
				t.Errorf("got %d total pages; want at least 1", total)
			}
		})
	}
}

func TestHomePagesAreRowsPerPageLong(t *testing.T) {
	app := newTestApp(t)

	first := hpHome(t, app, "page=1")
	hpAssertStatus(t, first, http.StatusOK)

	ids := hpRowIDs(first)
	if len(ids) > rowsPerPage {
		t.Fatalf("got %d rows; want at most %d", len(ids), rowsPerPage)
	}

	_, total := hpPageNumbers(t, first)
	if total < 2 {
		t.Skip("the table holds less than one full page; paging cannot be observed")
	}

	if len(ids) != rowsPerPage {
		t.Errorf("got %d rows on page 1 of %d; want a full page of %d", len(ids), total, rowsPerPage)
	}

	second := hpHome(t, app, "page=2")
	hpAssertStatus(t, second, http.StatusOK)

	for _, id := range hpRowIDs(second) {
		if slices.Contains(ids, id) {
			t.Errorf("got person %s on both pages; the offset must skip page 1", id)
		}
	}
}

func TestPersonRendersProfileAndRelations(t *testing.T) {
	app := newTestApp(t)
	id, childID := newTestPerson(t, app), newTestPerson(t, app)
	hpLink(t, app, id, childID)

	name, childName := testPersonName(t, app, id), testPersonName(t, app, childID)

	w := hpSend(t, app.person, strconv.Itoa(id), nil)

	hpAssertStatus(t, w, http.StatusOK)
	hpAssertBody(t, w, "<h1>"+name+"</h1>", "the profile is headed by the person's name")
	hpAssertBody(t, w, "<h2>Children</h2>", "a linked child is grouped under Children")
	hpAssertBody(t, w, childName, "the child is named on the page")
	hpAssertBody(t, w, `href="/person/`+strconv.Itoa(childID)+`"`, "the child card links onward")
}

func TestPersonWithoutRelations(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	w := hpSend(t, app.person, strconv.Itoa(id), nil)

	hpAssertStatus(t, w, http.StatusOK)
	hpAssertBody(t, w, "No relatives recorded.", "a lone person still renders")
}

func TestPersonHandlersRejectBadIDs(t *testing.T) {
	app := newTestApp(t)
	missing := strconv.Itoa(hpMissingID(t, app))

	handlers := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"person", app.person},
		{"editForm", app.editForm},
		{"edit", app.edit},
		{"deleteForm", app.deleteForm},
		{"delete", app.delete},
	}

	ids := []struct {
		name string
		id   string
	}{
		{"not a number", "twelve"},
		{"negative", "-1"},
		{"zero", "0"},
		{"absent", ""},
		{"unknown", missing},
	}

	for _, h := range handlers {
		for _, id := range ids {
			t.Run(h.name+"/"+id.name, func(t *testing.T) {
				w := hpSend(t, h.handler, id.id, url.Values{"name": {"Nobody " + h.name + id.name}})

				hpAssertStatus(t, w, http.StatusNotFound)
			})
		}
	}
}

func TestEditFormPrefillsFromTheStoredPerson(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	name := testPersonName(t, app, id)

	err := app.people.Update(context.Background(), models.Person{
		Summary:    models.Summary{ID: id, Name: name, Birthyear: 1815},
		Birthplace: "London",
		Bio:        "a stored bio",
		Location:   "Kent",
		Lat:        ptr(51.5),
		Lng:        ptr(-0.12),
	}, testUser)
	if err != nil {
		t.Fatal(err)
	}

	w := hpSend(t, app.editForm, strconv.Itoa(id), nil)
	hpAssertStatus(t, w, http.StatusOK)

	for _, want := range []string{
		`value="` + name + `"`,
		`value="1815"`,
		`value="London"`,
		`>a stored bio</textarea>`,
		`value="Kent"`,
		`value="51.5"`,
		`value="-0.12"`,
	} {
		hpAssertBody(t, w, want, "the form is pre-filled from the stored person")
	}
}

func TestEditUpdatesAndRedirects(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	renamed := testPersonName(t, app, id) + " Renamed"
	hpForgetHistory(t, app, renamed)

	w := hpSend(t, app.edit, strconv.Itoa(id), url.Values{
		"name":       {renamed},
		"birthyear":  {"1815"},
		"birthplace": {"London"},
		"bio":        {"a new bio"},
		"location":   {"Kent"},
		"lat":        {"51.5"},
		"lng":        {"-0.12"},
	})

	hpAssertStatus(t, w, http.StatusSeeOther)

	if got := w.Header().Get("Location"); got != "/person/"+strconv.Itoa(id) {
		t.Errorf("got redirect %q; want %q", got, "/person/"+strconv.Itoa(id))
	}

	person, err := app.people.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	if person.Name != renamed || person.Birthyear != 1815 || person.Birthplace != "London" || person.Bio != "a new bio" || person.Location != "Kent" {
		t.Errorf("got %+v; want every submitted field stored", person)
	}
	if person.Lat == nil || *person.Lat != 51.5 || person.Lng == nil || *person.Lng != -0.12 {
		t.Errorf("got lat %v lng %v; want 51.5 and -0.12", person.Lat, person.Lng)
	}
}

func TestEditRejectsInvalidInput(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	name := testPersonName(t, app, id)
	attempt := name + " Renamed"

	w := hpSend(t, app.edit, strconv.Itoa(id), url.Values{
		"name":      {attempt},
		"birthyear": {"nineteen"},
	})

	hpAssertStatus(t, w, http.StatusUnprocessableEntity)
	hpAssertBody(t, w, "Birth year must be a year in the past.", "the error is shown on the re-rendered form")
	hpAssertBody(t, w, `value="`+attempt+`"`, "the form is re-rendered with what was typed")

	if got := testPersonName(t, app, id); got != name {
		t.Errorf("got name %q; want %q — a rejected post must not reach the database", got, name)
	}
}

func TestEditRejectsBlankName(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	w := hpSend(t, app.edit, strconv.Itoa(id), url.Values{"name": {"   "}})

	hpAssertStatus(t, w, http.StatusUnprocessableEntity)
	hpAssertBody(t, w, "Name cannot be blank.", "the error is shown on the re-rendered form")
}

func TestEditRejectsDuplicateName(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	name := testPersonName(t, app, id)
	taken := testPersonName(t, app, newTestPerson(t, app))

	w := hpSend(t, app.edit, strconv.Itoa(id), url.Values{"name": {taken}})

	hpAssertStatus(t, w, http.StatusUnprocessableEntity)
	hpAssertBody(t, w, "already exists", "a name clash is a form error, not a server error")

	if got := testPersonName(t, app, id); got != name {
		t.Errorf("got name %q; want %q — the rejected update must roll back", got, name)
	}
}

func TestEditRejectsMalformedBody(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	r := httptest.NewRequest(http.MethodPost, "/person/"+strconv.Itoa(id)+"/edit", strings.NewReader("name=%zz"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", strconv.Itoa(id))

	w := httptest.NewRecorder()
	app.edit(w, requestWithUser(r, models.User{ID: 1, Name: testUser}))

	hpAssertStatus(t, w, http.StatusBadRequest)
}

func TestDeleteFormShowsWhatWillBeLost(t *testing.T) {
	app := newTestApp(t)
	id, parentID := newTestPerson(t, app), newTestPerson(t, app)
	hpLink(t, app, parentID, id)

	name, parentName := testPersonName(t, app, id), testPersonName(t, app, parentID)

	w := hpSend(t, app.deleteForm, strconv.Itoa(id), nil)

	hpAssertStatus(t, w, http.StatusOK)
	hpAssertBody(t, w, "Delete "+name+"?", "the confirmation names the person")
	hpAssertBody(t, w, "Parents", "the relation group about to be broken is listed")
	hpAssertBody(t, w, parentName, "the relative that will be unlinked is named")
}

func TestDeleteRemovesThePerson(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	w := hpSend(t, app.delete, strconv.Itoa(id), nil)

	hpAssertStatus(t, w, http.StatusSeeOther)

	if got := w.Header().Get("Location"); got != "/" {
		t.Errorf("got redirect %q; want /", got)
	}
	if hpExists(t, app, id) {
		t.Error("got the person still in the table; want them gone")
	}
}

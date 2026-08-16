package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

func hrName(label string) string {
	return "HR " + label + " " + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func hrCall(h http.HandlerFunc, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h(w, r)

	return w
}

func hrGet(id string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("id", id)

	return requestWithUser(r, models.User{Name: testUser})
}

func hrPost(t *testing.T, id string, values url.Values) *http.Request {
	t.Helper()

	r := postForm(t, values)
	r.SetPathValue("id", id)

	return requestWithUser(r, models.User{Name: testUser})
}

func hrBadBody(id string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("name=%zz"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("id", id)

	return requestWithUser(r, models.User{Name: testUser})
}

func hrBadIDs() map[string]string {
	return map[string]string{"not a number": "abc", "zero": "0", "negative": "-1"}
}

func hrUnknownID(t *testing.T, app *application) string {
	t.Helper()

	var id int

	err := app.people.DB.QueryRow(context.Background(), `SELECT COALESCE(MAX(id), 0) + 1000 FROM api_person`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}

	return strconv.Itoa(id)
}

func hrCleanupPeople(t *testing.T, app *application, names ...string) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()
		queries := []string{
			`DELETE FROM api_parentchild WHERE parent_id IN (SELECT id FROM api_person WHERE name = $1)
			    OR child_id IN (SELECT id FROM api_person WHERE name = $1)`,
			`DELETE FROM api_marriage WHERE person_a_id IN (SELECT id FROM api_person WHERE name = $1)
			    OR person_b_id IN (SELECT id FROM api_person WHERE name = $1)`,
			`DELETE FROM api_location WHERE person_id IN (SELECT id FROM api_person WHERE name = $1)`,
			`DELETE FROM api_photo WHERE person_id IN (SELECT id FROM api_person WHERE name = $1)`,
			`DELETE FROM api_history WHERE recipient = $1`,
			`DELETE FROM api_person WHERE name = $1`,
		}

		for _, name := range names {
			for _, query := range queries {
				_, err := app.people.DB.Exec(ctx, query, name)
				if err != nil {
					t.Error(err)
				}
			}
		}
	})
}

func hrPersonID(t *testing.T, app *application, name string) (int, bool) {
	t.Helper()

	ids := []int{}

	rows, err := app.people.DB.Query(context.Background(), `SELECT id FROM api_person WHERE name = $1`, name)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int

		err = rows.Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}

	if len(ids) > 1 {
		t.Fatalf("got %d people called %q; want at most 1", len(ids), name)
	}
	if len(ids) == 0 {
		return 0, false
	}

	return ids[0], true
}

func hrFacts(t *testing.T, app *application, id int) []models.Fact {
	t.Helper()

	list, err := app.people.Facts(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	return list
}

func hrHasFact(list []models.Fact, relation, name string) bool {
	return slices.ContainsFunc(list, func(f models.Fact) bool {
		return f.Relation == relation && f.Person.Name == name
	})
}

func hrHasSibling(t *testing.T, app *application, id int, name string) bool {
	t.Helper()

	groups, err := app.people.Relations(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	for _, g := range groups {
		if g.Title != "Siblings" {
			continue
		}
		if slices.ContainsFunc(g.People, func(p models.Summary) bool { return p.Name == name }) {
			return true
		}
	}

	return false
}

func hrLink(t *testing.T, app *application, id int, name, relation string) {
	t.Helper()

	err := app.people.Link(context.Background(), id, name, relation, testUser)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRelativesRendersFactsAndPicker(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	childID := newTestPerson(t, app)
	strangerID := newTestPerson(t, app)
	name, childName, strangerName := testPersonName(t, app, id), testPersonName(t, app, childID), testPersonName(t, app, strangerID)

	hrLink(t, app, id, childName, "child")

	res := hrCall(app.relatives, hrGet(strconv.Itoa(id)))
	body := res.Body.String()

	if res.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusOK, body)
	}
	if !strings.Contains(body, name) {
		t.Errorf("got a page without %q; the subject's own name must be on it", name)
	}
	if !strings.Contains(body, childName) {
		t.Errorf("got a page without %q; stored facts must be listed", childName)
	}
	if !strings.Contains(body, strangerName) {
		t.Errorf("got a page without %q; every other person is an option in the picker", strangerName)
	}
}

func TestRelativesNotFound(t *testing.T) {
	app := newTestApp(t)
	ids := hrBadIDs()
	ids["unknown"] = hrUnknownID(t, app)

	for name, id := range ids {
		t.Run(name, func(t *testing.T) {
			res := hrCall(app.relatives, hrGet(id))

			if res.Code != http.StatusNotFound {
				t.Errorf("got status %d; want %d — body: %s", res.Code, http.StatusNotFound, res.Body)
			}
		})
	}
}

func TestLinkStoresEachFact(t *testing.T) {
	app := newTestApp(t)

	for _, relation := range facts {
		t.Run(relation, func(t *testing.T) {
			id := newTestPerson(t, app)
			otherID := newTestPerson(t, app)
			otherName := testPersonName(t, app, otherID)

			res := hrCall(app.link, hrPost(t, strconv.Itoa(id), url.Values{"name": {otherName}, "relation": {relation}}))

			if res.Code != http.StatusSeeOther {
				t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusSeeOther, res.Body)
			}
			if got, want := res.Header().Get("Location"), "/person/"+strconv.Itoa(id)+"/relatives"; got != want {
				t.Errorf("got redirect %q; want %q", got, want)
			}
			if !hrHasFact(hrFacts(t, app, id), relation, otherName) {
				t.Errorf("got no %s fact for %q; the link was not stored", relation, otherName)
			}
		})
	}
}

func TestLinkRejects(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	otherID := newTestPerson(t, app)
	name, otherName := testPersonName(t, app, id), testPersonName(t, app, otherID)

	tests := []struct {
		label    string
		relation string
		value    func(subject, other string) string
		want     string
	}{
		{"blank name", "parent", func(_, _ string) string { return "" }, "Choose a person."},
		{"name is only spaces", "parent", func(_, _ string) string { return "   " }, "Choose a person."},
		{"sibling is not a fact", "sibling", func(_, other string) string { return other }, "Choose how they are related."},
		{"relation not offered", "cousin", func(_, other string) string { return other }, "Choose how they are related."},
		{"blank relation", "", func(_, other string) string { return other }, "Choose how they are related."},
		{"nobody has that name", "parent", func(_, _ string) string { return hrName("ghost") }, "No one on the site is called that."},
		{"linking a person to themselves", "parent", func(subject, _ string) string { return subject }, "A person cannot be their own relative."},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			values := url.Values{"name": {tt.value(name, otherName)}, "relation": {tt.relation}}

			res := hrCall(app.link, hrPost(t, strconv.Itoa(id), values))

			if res.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusUnprocessableEntity, res.Body)
			}
			if !strings.Contains(res.Body.String(), tt.want) {
				t.Errorf("got a page without %q; the reason must be visible on the re-rendered form", tt.want)
			}
			if list := hrFacts(t, app, id); len(list) != 0 {
				t.Errorf("got facts %+v; want none — a rejected link stores nothing", list)
			}
		})
	}
}

func TestLinkRejectsDuplicate(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	otherID := newTestPerson(t, app)
	otherName := testPersonName(t, app, otherID)

	hrLink(t, app, id, otherName, "spouse")

	res := hrCall(app.link, hrPost(t, strconv.Itoa(id), url.Values{"name": {otherName}, "relation": {"spouse"}}))

	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusUnprocessableEntity, res.Body)
	}
	if !strings.Contains(res.Body.String(), "That link already exists.") {
		t.Error("got a page without the duplicate message; a repeat link is a form error, not a 500")
	}
	if list := hrFacts(t, app, id); len(list) != 1 {
		t.Errorf("got facts %+v; want exactly the one that already existed", list)
	}
}

func TestLinkMalformedBody(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	res := hrCall(app.link, hrBadBody(strconv.Itoa(id)))

	if res.Code != http.StatusBadRequest {
		t.Errorf("got status %d; want %d — body: %s", res.Code, http.StatusBadRequest, res.Body)
	}
}

func TestLinkNotFound(t *testing.T) {
	app := newTestApp(t)

	for label, id := range hrBadIDs() {
		t.Run(label, func(t *testing.T) {
			res := hrCall(app.link, hrPost(t, id, url.Values{"name": {"anyone"}, "relation": {"parent"}}))

			if res.Code != http.StatusNotFound {
				t.Errorf("got status %d; want %d — body: %s", res.Code, http.StatusNotFound, res.Body)
			}
		})
	}
}

func TestUnlinkRemovesFact(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	otherID := newTestPerson(t, app)
	otherName := testPersonName(t, app, otherID)

	hrLink(t, app, id, otherName, "child")

	res := hrCall(app.unlink, hrPost(t, strconv.Itoa(id), url.Values{"name": {otherName}, "relation": {"child"}}))

	if res.Code != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusSeeOther, res.Body)
	}
	if got, want := res.Header().Get("Location"), "/person/"+strconv.Itoa(id)+"/relatives"; got != want {
		t.Errorf("got redirect %q; want %q", got, want)
	}
	if list := hrFacts(t, app, id); len(list) != 0 {
		t.Errorf("got facts %+v; want none", list)
	}
}

func TestUnlinkIsIdempotent(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	otherID := newTestPerson(t, app)
	otherName := testPersonName(t, app, otherID)

	res := hrCall(app.unlink, hrPost(t, strconv.Itoa(id), url.Values{"name": {otherName}, "relation": {"spouse"}}))

	if res.Code != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d — removing an edge that is not there is deliberately not an error; body: %s",
			res.Code, http.StatusSeeOther, res.Body)
	}
}

func TestUnlinkRejectsNonFactRelation(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	otherID := newTestPerson(t, app)
	otherName := testPersonName(t, app, otherID)

	for _, relation := range []string{"sibling", "cousin", ""} {
		t.Run(strconv.Quote(relation), func(t *testing.T) {
			values := url.Values{"name": {otherName}, "relation": {relation}}

			res := hrCall(app.unlink, hrPost(t, strconv.Itoa(id), values))

			if res.Code != http.StatusBadRequest {
				t.Errorf("got status %d; want %d — only stored facts can be unlinked; body: %s",
					res.Code, http.StatusBadRequest, res.Body)
			}
		})
	}
}

func TestUnlinkMalformedBody(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	res := hrCall(app.unlink, hrBadBody(strconv.Itoa(id)))

	if res.Code != http.StatusBadRequest {
		t.Errorf("got status %d; want %d — body: %s", res.Code, http.StatusBadRequest, res.Body)
	}
}

func TestUnlinkNotFound(t *testing.T) {
	app := newTestApp(t)

	for label, id := range hrBadIDs() {
		t.Run(label, func(t *testing.T) {
			res := hrCall(app.unlink, hrPost(t, id, url.Values{"name": {"anyone"}, "relation": {"parent"}}))

			if res.Code != http.StatusNotFound {
				t.Errorf("got status %d; want %d — body: %s", res.Code, http.StatusNotFound, res.Body)
			}
		})
	}
}

func TestAddRelativeFormRenders(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	name := testPersonName(t, app, id)

	res := hrCall(app.addRelativeForm, hrGet(strconv.Itoa(id)))
	body := res.Body.String()

	if res.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusOK, body)
	}
	if !strings.Contains(body, name) {
		t.Errorf("got a page without %q; the form names the person being added to", name)
	}
	for _, relation := range relations {
		if !strings.Contains(body, `value="`+relation+`"`) {
			t.Errorf("got a form without a %q option", relation)
		}
	}
}

func TestAddRelativeFormNotFound(t *testing.T) {
	app := newTestApp(t)
	ids := hrBadIDs()
	ids["unknown"] = hrUnknownID(t, app)

	for label, id := range ids {
		t.Run(label, func(t *testing.T) {
			res := hrCall(app.addRelativeForm, hrGet(id))

			if res.Code != http.StatusNotFound {
				t.Errorf("got status %d; want %d — body: %s", res.Code, http.StatusNotFound, res.Body)
			}
		})
	}
}

func TestAddRelativeCreatesAndLinks(t *testing.T) {
	app := newTestApp(t)

	for _, relation := range facts {
		t.Run(relation, func(t *testing.T) {
			id := newTestPerson(t, app)
			name := hrName(relation)
			hrCleanupPeople(t, app, name)

			values := url.Values{"name": {name}, "relation": {relation}, "birthyear": {"1900"}, "location": {"London"}, "lat": {"51.5"}, "lng": {"-0.12"}}

			res := hrCall(app.addRelative, hrPost(t, strconv.Itoa(id), values))

			if res.Code != http.StatusSeeOther {
				t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusSeeOther, res.Body)
			}
			if got, want := res.Header().Get("Location"), "/person/"+strconv.Itoa(id); got != want {
				t.Errorf("got redirect %q; want %q", got, want)
			}
			if _, ok := hrPersonID(t, app, name); !ok {
				t.Fatalf("got no person called %q; the relative was not created", name)
			}
			if !hrHasFact(hrFacts(t, app, id), relation, name) {
				t.Errorf("got no %s fact for %q; the new person was created but not linked", relation, name)
			}
		})
	}
}

func TestAddRelativeSiblingCopiesParents(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	parentID := newTestPerson(t, app)
	parentName := testPersonName(t, app, parentID)
	name := hrName("sibling copied")
	hrCleanupPeople(t, app, name, "Unknown parent of "+testPersonName(t, app, id))

	hrLink(t, app, id, parentName, "parent")

	res := hrCall(app.addRelative, hrPost(t, strconv.Itoa(id), url.Values{"name": {name}, "relation": {"sibling"}}))

	if res.Code != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusSeeOther, res.Body)
	}

	newID, ok := hrPersonID(t, app, name)
	if !ok {
		t.Fatalf("got no person called %q", name)
	}

	if !hrHasFact(hrFacts(t, app, newID), "parent", parentName) {
		t.Errorf("got no parent %q for the new sibling; an existing parent is shared, not invented", parentName)
	}
	if _, invented := hrPersonID(t, app, "Unknown parent of "+testPersonName(t, app, id)); invented {
		t.Error("got an Unknown parent placeholder; one is only invented when there is no parent to copy")
	}
	if !hrHasSibling(t, app, id, name) {
		t.Errorf("got no sibling %q; a sibling is derived from the shared parent", name)
	}
}

func TestAddRelativeSiblingInventsUnknownParent(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	name := hrName("sibling invented")
	placeholder := "Unknown parent of " + testPersonName(t, app, id)
	hrCleanupPeople(t, app, name, placeholder)

	res := hrCall(app.addRelative, hrPost(t, strconv.Itoa(id), url.Values{"name": {name}, "relation": {"sibling"}}))

	if res.Code != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusSeeOther, res.Body)
	}

	newID, ok := hrPersonID(t, app, name)
	if !ok {
		t.Fatalf("got no person called %q", name)
	}

	if !hrHasFact(hrFacts(t, app, id), "parent", placeholder) {
		t.Errorf("got no %q for the subject; with no parent to copy one is invented for both", placeholder)
	}
	if !hrHasFact(hrFacts(t, app, newID), "parent", placeholder) {
		t.Errorf("got no %q for the new person; the placeholder must parent both of them", placeholder)
	}
	if !hrHasSibling(t, app, id, name) {
		t.Errorf("got no sibling %q; the shared placeholder is what makes them siblings", name)
	}
}

func TestAddRelativeRejectsInvalidForm(t *testing.T) {
	app := newTestApp(t)
	nextYear := strconv.Itoa(time.Now().Year() + 1)

	tests := []struct {
		label     string
		name      string
		relation  string
		birthyear string
		want      string
	}{
		{"blank name", "", "parent", "", "Name cannot be blank."},
		{"birth year in the future", hrName("future"), "parent", nextYear, "Birth year must be a year in the past."},
		{"relation not offered", hrName("cousin"), "cousin", "", "Choose how this person is related."},
		{"blank relation", hrName("norelation"), "", "", "Choose how this person is related."},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			id := newTestPerson(t, app)
			values := url.Values{"name": {tt.name}, "relation": {tt.relation}, "birthyear": {tt.birthyear}}

			res := hrCall(app.addRelative, hrPost(t, strconv.Itoa(id), values))

			if res.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusUnprocessableEntity, res.Body)
			}
			if !strings.Contains(res.Body.String(), tt.want) {
				t.Errorf("got a page without %q; the reason must be visible on the re-rendered form", tt.want)
			}
			if tt.name != "" {
				if _, ok := hrPersonID(t, app, tt.name); ok {
					t.Errorf("got a person called %q; a rejected form creates nobody", tt.name)
				}
			}
			if list := hrFacts(t, app, id); len(list) != 0 {
				t.Errorf("got facts %+v; want none", list)
			}
		})
	}
}

func TestAddRelativeRejectsDuplicateName(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	takenID := newTestPerson(t, app)
	taken := testPersonName(t, app, takenID)

	res := hrCall(app.addRelative, hrPost(t, strconv.Itoa(id), url.Values{"name": {taken}, "relation": {"child"}}))

	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d; want %d — a taken name is a form error, not a 500; body: %s",
			res.Code, http.StatusUnprocessableEntity, res.Body)
	}
	if !strings.Contains(res.Body.String(), "already exists") {
		t.Error("got a page without an 'already exists' message")
	}

	var n int

	err := app.people.DB.QueryRow(context.Background(), `SELECT count(*) FROM api_person WHERE name = $1`, taken).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("got %d people called %q; want 1 — the second insert must not land", n, taken)
	}

	if list := hrFacts(t, app, id); len(list) != 0 {
		t.Errorf("got facts %+v; want none — the whole insert is rolled back", list)
	}
}

func TestAddRelativeNotFound(t *testing.T) {
	app := newTestApp(t)
	ids := hrBadIDs()
	ids["unknown"] = hrUnknownID(t, app)

	for label, id := range ids {
		t.Run(label, func(t *testing.T) {
			values := url.Values{"name": {hrName("never created")}, "relation": {"child"}}

			res := hrCall(app.addRelative, hrPost(t, id, values))

			if res.Code != http.StatusNotFound {
				t.Errorf("got status %d; want %d — body: %s", res.Code, http.StatusNotFound, res.Body)
			}
		})
	}
}

func TestAddRelativeMalformedBody(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	res := hrCall(app.addRelative, hrBadBody(strconv.Itoa(id)))

	if res.Code != http.StatusBadRequest {
		t.Errorf("got status %d; want %d — body: %s", res.Code, http.StatusBadRequest, res.Body)
	}
}

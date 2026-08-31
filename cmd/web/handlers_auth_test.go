package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	haAdminHash     = "pbkdf2_sha256$1500000$xT0rOS1a20sYdLxorfNVug$IbfYM7jlW/MARo2rYhVAYSMStjGUpPQFCXnapXn2O5o="
	haAdminPassword = "admin"
)

var haHostileNext = []struct {
	name string
	next string
}{
	{"protocol relative", "//evil.example.com"},
	{"absolute url", "https://evil.example.com/"},
	{"backslash scheme", "/\\evil.example.com"},
}

const haInsertUser = `
INSERT INTO auth_user (password, is_superuser, username, first_name, last_name, email, is_staff, is_active, date_joined)
VALUES ($1, false, $2, '', '', '', false, true, now())
RETURNING id`

func haNewUser(t *testing.T, app *application) (int, string) {
	t.Helper()

	ctx := context.Background()
	username := "ha-login-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	var id int

	err := app.users.DB.QueryRow(ctx, haInsertUser, haAdminHash, username).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_, err := app.users.DB.Exec(ctx, `DELETE FROM auth_user WHERE id = $1`, id)
		if err != nil {
			t.Error(err)
		}
	})

	return id, username
}

func haSessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	cookies := (&http.Response{Header: w.Header()}).Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookie {
		t.Fatalf("got cookies %v; want exactly the session cookie", cookies)
	}

	return cookies[0]
}

func TestSafeNext(t *testing.T) {
	tests := []struct {
		name string
		next string
		want string
	}{
		{"plain path", "/person/1", "/person/1"},
		{"path with query", "/person/1/photos?page=2", "/person/1/photos?page=2"},
		{"root", "/", "/"},
		{"empty", "", "/"},
		{"protocol relative", "//evil.example.com", "/"},
		{"backslash scheme", "/\\evil.example.com", "/"},
		{"absolute url", "https://evil.example.com/", "/"},
		{"scheme only", "javascript:alert(1)", "/"},
		{"no leading slash", "person/1", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeNext(tt.next)
			if got != tt.want {
				t.Errorf("safeNext(%q) = %q; want %q", tt.next, got, tt.want)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	app := newSessionApp(testSecret, true)

	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: issueCookie(t, app, 7).Value})

	w := httptest.NewRecorder()
	app.logout(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusSeeOther)
	}
	if got := w.Header().Get("Location"); got != "/" {
		t.Errorf("got redirect %q; want /", got)
	}

	cookies := (&http.Response{Header: w.Header()}).Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookie {
		t.Fatalf("got cookies %v; want the session cookie cleared", cookies)
	}
	if cookies[0].Value != "" || cookies[0].MaxAge != -1 {
		t.Errorf("got %+v; logging out must expire the cookie, not just redirect", cookies[0])
	}
}

func TestLoginForm(t *testing.T) {
	app := newTestApp(t)

	r := httptest.NewRequest(http.MethodGet, "/login?"+url.Values{"next": {"/person/1"}}.Encode(), nil)
	w := httptest.NewRecorder()
	app.loginForm(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusOK, w.Body)
	}
	if !strings.Contains(w.Body.String(), `value="/person/1"`) {
		t.Errorf("got body %s; want ?next= carried into the hidden field", w.Body)
	}
}

func TestLoginFormHostileNext(t *testing.T) {
	app := newTestApp(t)

	for _, tt := range haHostileNext {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/login?"+url.Values{"next": {tt.next}}.Encode(), nil)
			w := httptest.NewRecorder()
			app.loginForm(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusOK, w.Body)
			}

			body := w.Body.String()

			if !strings.Contains(body, `value="/"`) {
				t.Errorf("got body %s; want next neutralised to /", body)
			}
			if strings.Contains(body, "evil.example.com") {
				t.Errorf("got body %s; an off-site next must never reach the page", body)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	app := newTestApp(t)
	id, username := haNewUser(t, app)

	w := httptest.NewRecorder()
	app.login(w, postForm(t, url.Values{"name": {username}, "password": {haAdminPassword}, "next": {"/person/1"}}))

	if w.Code != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusSeeOther, w.Body)
	}
	if got := w.Header().Get("Location"); got != "/person/1" {
		t.Errorf("got redirect %q; want /person/1", got)
	}

	c := haSessionCookie(t, w)
	if !c.HttpOnly {
		t.Error("the session cookie is not HttpOnly; JavaScript must never read the session")
	}

	got, err := readCookie(app, c.Value)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Errorf("got user %d; want %d", got, id)
	}
}

func TestLoginTrimsUsername(t *testing.T) {
	app := newTestApp(t)
	id, username := haNewUser(t, app)

	w := httptest.NewRecorder()
	app.login(w, postForm(t, url.Values{"name": {"  " + username + "\t"}, "password": {haAdminPassword}}))

	if w.Code != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d — a padded username must still authenticate (body: %s)", w.Code, http.StatusSeeOther, w.Body)
	}

	got, err := readCookie(app, haSessionCookie(t, w).Value)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Errorf("got user %d; want %d", got, id)
	}
}

func TestLoginHostileNext(t *testing.T) {
	app := newTestApp(t)
	_, username := haNewUser(t, app)

	for _, tt := range haHostileNext {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			app.login(w, postForm(t, url.Values{"name": {username}, "password": {haAdminPassword}, "next": {tt.next}}))

			if w.Code != http.StatusSeeOther {
				t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusSeeOther, w.Body)
			}
			if got := w.Header().Get("Location"); got != "/" {
				t.Errorf("got redirect %q; want / — a hostile next in the form body must not send the user off-site", got)
			}
		})
	}
}

func TestLoginRejects(t *testing.T) {
	app := newTestApp(t)
	_, username := haNewUser(t, app)

	tests := []struct {
		name     string
		user     string
		password string
	}{
		{"wrong password", username, "ha-not-the-password"},
		{"unknown username", "ha-no-such-user-" + strconv.FormatInt(time.Now().UnixNano(), 10), "ha-irrelevant-password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			app.login(w, postForm(t, url.Values{"name": {tt.user}, "password": {tt.password}}))

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusUnprocessableEntity, w.Body)
			}

			body := w.Body.String()

			if !strings.Contains(body, "Username or password is incorrect.") {
				t.Errorf("got body %s; want the failure re-rendered with the error", body)
			}
			if !strings.Contains(body, `value="`+tt.user+`"`) {
				t.Errorf("got body %s; want the submitted username echoed back", body)
			}
			if strings.Contains(body, tt.password) {
				t.Errorf("got body %s; the submitted password must never be echoed back", body)
			}

			cookies := (&http.Response{Header: w.Header()}).Cookies()
			if len(cookies) != 0 {
				t.Errorf("got cookies %v; want none — a failed login must not start a session", cookies)
			}
		})
	}
}

func TestLoginMalformedBody(t *testing.T) {
	app := newSessionApp(testSecret, false)

	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("name=%zz"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	app.login(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body)
	}
}

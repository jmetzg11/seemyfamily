package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

const (
	testSecret  = "test-session-secret-at-least-32-chars"
	otherSecret = "a-completely-different-secret-32-ch"
)

func newSessionApp(secret string, secure bool) *application {
	return &application{sessionSecret: []byte(secret), secureCookies: secure}
}

func issueCookie(t *testing.T, app *application, userID int) *http.Cookie {
	t.Helper()

	w := httptest.NewRecorder()
	app.setSessionCookie(w, userID)

	cookies := (&http.Response{Header: w.Header()}).Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies; want 1", len(cookies))
	}

	return cookies[0]
}

func readCookie(app *application, value string) (int, error) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: value})

	return app.readSessionCookie(r)
}

func signedPayload(app *application, id int, expiry time.Time) string {
	payload := strconv.Itoa(id) + "." + strconv.FormatInt(expiry.Unix(), 10)
	return payload + "." + app.sign(payload)
}

func TestSessionRoundTrip(t *testing.T) {
	app := newSessionApp(testSecret, false)

	got, err := readCookie(app, issueCookie(t, app, 42).Value)
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Errorf("got user %d; want 42", got)
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run("secureCookies="+strconv.FormatBool(secure), func(t *testing.T) {
			c := issueCookie(t, newSessionApp(testSecret, secure), 1)

			if c.Name != sessionCookie {
				t.Errorf("got name %q; want %q", c.Name, sessionCookie)
			}
			if !c.HttpOnly {
				t.Error("cookie is not HttpOnly; JavaScript must never read the session")
			}
			if c.Path != "/" {
				t.Errorf("got path %q; want /", c.Path)
			}
			if c.SameSite != http.SameSiteLaxMode {
				t.Errorf("got SameSite %v; want Lax", c.SameSite)
			}
			if c.Secure != secure {
				t.Errorf("got Secure %v; want %v", c.Secure, secure)
			}
			if c.MaxAge != int(sessionLifetime.Seconds()) {
				t.Errorf("got MaxAge %d; want %d", c.MaxAge, int(sessionLifetime.Seconds()))
			}
		})
	}
}

func TestClearSessionCookie(t *testing.T) {
	app := newSessionApp(testSecret, true)

	w := httptest.NewRecorder()
	app.clearSessionCookie(w)

	cookies := (&http.Response{Header: w.Header()}).Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies; want 1", len(cookies))
	}

	c := cookies[0]

	if c.Value != "" {
		t.Errorf("got value %q; want empty", c.Value)
	}
	if c.MaxAge != -1 {
		t.Errorf("got MaxAge %d; want -1", c.MaxAge)
	}
	if !c.Secure || !c.HttpOnly {
		t.Error("the clearing cookie must carry the same flags as the one it replaces")
	}
}

func TestReadSessionCookieRejects(t *testing.T) {
	app := newSessionApp(testSecret, false)
	other := newSessionApp(otherSecret, false)

	future := time.Now().Add(time.Hour)
	valid := signedPayload(app, 42, future)

	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"no separator", "nonsense"},
		{"payload only", "42." + strconv.FormatInt(future.Unix(), 10)},
		{"signature only", app.sign("42")},
		{"tampered user id", "43" + valid[2:]},
		{"tampered signature", valid[:len(valid)-1] + flip(valid[len(valid)-1])},
		{"truncated signature", valid[:len(valid)-1]},
		{"signed with another secret", signedPayload(other, 42, future)},
		{"expired", signedPayload(app, 42, time.Now().Add(-time.Hour))},
		{"non-numeric user id", "abc." + strconv.FormatInt(future.Unix(), 10) + "." + app.sign("abc."+strconv.FormatInt(future.Unix(), 10))},
		{"non-numeric expiry", "42.soon." + app.sign("42.soon")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readCookie(app, tt.value)
			if !errors.Is(err, errInvalidSession) {
				t.Errorf("got err %v; want errInvalidSession", err)
			}
		})
	}
}

func TestReadSessionCookieNoCookie(t *testing.T) {
	app := newSessionApp(testSecret, false)

	_, err := app.readSessionCookie(httptest.NewRequest(http.MethodGet, "/", nil))
	if !errors.Is(err, errInvalidSession) {
		t.Errorf("got err %v; want errInvalidSession", err)
	}
}

func flip(b byte) string {
	if b == 'A' {
		return "B"
	}
	return "A"
}

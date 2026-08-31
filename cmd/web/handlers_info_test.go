package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

func hiInfo(t *testing.T, app *application, query string) string {
	t.Helper()

	r := requestWithUser(httptest.NewRequest(http.MethodGet, "/info"+query, nil), models.User{ID: 1, Name: testUser})
	w := httptest.NewRecorder()

	app.info(w, r)

	res := w.Result()
	defer res.Body.Close()

	body := w.Body.String()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %d; want %d; body: %s", res.StatusCode, http.StatusOK, body)
	}

	return body
}

func TestInfoShowsRecentEdits(t *testing.T) {
	app := newTestApp(t)

	recipient := testPersonName(t, app, newTestPerson(t, app))
	action := "changed something"

	_, err := app.stats.DB.Exec(context.Background(),
		`INSERT INTO api_history (created_at, username, action, recipient) VALUES ($1, $2, $3, $4)`,
		time.Now(), testUser, action, recipient)
	if err != nil {
		t.Fatal(err)
	}

	body := hiInfo(t, app, "")

	if !strings.Contains(body, recipient) {
		t.Errorf("got no %q row; want the recent edit listed", recipient)
	}
	if !strings.Contains(body, action) {
		t.Errorf("got no %q action; want the recent edit listed", action)
	}
	if !strings.Contains(body, testUser) {
		t.Errorf("got no %q author; want the recent edit listed", testUser)
	}
}

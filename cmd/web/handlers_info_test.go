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

func hiFirstLabel(t *testing.T, app *application, days int) string {
	t.Helper()

	var start time.Time

	err := app.stats.DB.QueryRow(context.Background(),
		`SELECT CURRENT_DATE - make_interval(days => $1::int - 1)`, days).Scan(&start)
	if err != nil {
		t.Fatal(err)
	}

	return start.Format("Jan 2")
}

func hiActiveRange(body, period string) bool {
	return strings.Contains(body, `href="/info?range=`+period+`" class="active"`)
}

func TestInfoRangeSelectsSpec(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name  string
		query string
		days  int
		bars  int
	}{
		{"week", "?range=week", 7, 7},
		{"month", "?range=month", 30, 6},
		{"halfyear", "?range=halfyear", 180, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := hiInfo(t, app, tt.query)

			if !hiActiveRange(body, tt.name) {
				t.Errorf("got no active link for %q; want the requested range echoed back", tt.name)
			}
			if got := strings.Count(body, `class="bar"`); got != tt.bars {
				t.Errorf("got %d bars; want %d — %d days grouped by the %q spec", got, tt.bars, tt.days, tt.name)
			}

			label := hiFirstLabel(t, app, tt.days)
			if !strings.Contains(body, ">"+label+"</text>") {
				t.Errorf("got no %q tick; want the chart to start %d days back", label, tt.days-1)
			}
		})
	}
}

func TestInfoUnknownRangeFallsBackToMonth(t *testing.T) {
	app := newTestApp(t)

	tests := []struct {
		name  string
		query string
	}{
		{"absent", ""},
		{"empty", "?range="},
		{"unknown", "?range=decade"},
		{"wrong case", "?range=WEEK"},
		{"other param only", "?page=2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := hiInfo(t, app, tt.query)

			if !hiActiveRange(body, "month") {
				t.Errorf("got no active month link for %q; want the month default", tt.query)
			}
			if hiActiveRange(body, "week") || hiActiveRange(body, "halfyear") {
				t.Errorf("got a second active range for %q; want month alone", tt.query)
			}
			if got := strings.Count(body, `class="bar"`); got != 6 {
				t.Errorf("got %d bars; want 6 — the fallback must use the month spec", got)
			}
		})
	}
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

func TestVisitorRanges(t *testing.T) {
	want := map[string]bool{"week": true, "month": true, "halfyear": true}

	for period := range visitorRanges {
		if !want[period] {
			t.Errorf("got range %q; want only week, month and halfyear — the template links no others", period)
		}
	}

	for period := range want {
		spec, ok := visitorRanges[period]
		if !ok {
			t.Fatalf("got no %q spec; want one for every template link", period)
		}
		if spec.labelEvery < 1 {
			t.Errorf("got labelEvery %d for %q; want >= 1 — buildChart does i %% labelEvery", spec.labelEvery, period)
		}
	}
}

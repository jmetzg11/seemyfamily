package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func hmNewLocation(t *testing.T, app *application, personID int, name string) (float64, float64) {
	t.Helper()

	n := time.Now().UnixNano()
	lat := 10 + float64(n%1_000_000)/1e6
	lng := 20 + float64((n/1_000_000)%1_000_000)/1e6

	_, err := app.locations.DB.Exec(context.Background(),
		`INSERT INTO api_location (person_id, name, lat, lng) VALUES ($1, $2, $3, $4)`,
		personID, name, lat, lng)
	if err != nil {
		t.Fatal(err)
	}

	return lat, lng
}

func hmMapData(t *testing.T, body string) []mapPlace {
	t.Helper()

	_, rest, ok := strings.Cut(body, `<script id="map-data" type="application/json">`)
	if !ok {
		t.Fatalf("got body %s; want a map-data block", body)
	}

	raw, _, ok := strings.Cut(rest, "</script>")
	if !ok {
		t.Fatalf("got body %s; the map-data block is not closed", body)
	}

	var places []mapPlace

	err := json.Unmarshal([]byte(raw), &places)
	if err != nil {
		t.Fatalf("got %s; want valid JSON: %v", raw, err)
	}

	return places
}

func TestMapPage(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)
	name := testPersonName(t, app, id)
	lat, lng := hmNewLocation(t, app, id, "Test Town")

	w := httptest.NewRecorder()
	app.mapPage(w, httptest.NewRequest(http.MethodGet, "/map", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusOK, w.Body)
	}

	var got *mapPlace

	for _, p := range hmMapData(t, w.Body.String()) {
		if p.Lat == lat && p.Lng == lng {
			got = &p
		}
	}

	if got == nil {
		t.Fatalf("got body %s; want the place at %f,%f", w.Body, lat, lng)
	}
	if got.Name != "Test Town" {
		t.Errorf("got name %q; want Test Town", got.Name)
	}
	if len(got.People) != 1 || got.People[0].ID != id || got.People[0].Name != name {
		t.Errorf("got people %+v; want %d/%q so the popup can link to them", got.People, id, name)
	}
}

func TestMapPageLoadsLeafletAndTheInitScript(t *testing.T) {
	app := newTestApp(t)

	w := httptest.NewRecorder()
	app.mapPage(w, httptest.NewRequest(http.MethodGet, "/map", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusOK, w.Body)
	}

	body := w.Body.String()

	for _, want := range []string{
		leafletHost + "/leaflet@1.9.4/dist/leaflet.css",
		leafletHost + "/leaflet@1.9.4/dist/leaflet.js",
		`integrity="sha256-`,
		`<div id="map">`,
		"/static/js/map.js",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("got body %s; missing %q", body, want)
		}
	}
}

func TestMapPageWithoutLocations(t *testing.T) {
	app := newTestApp(t)

	w := httptest.NewRecorder()
	app.mapPage(w, httptest.NewRequest(http.MethodGet, "/map", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d (body: %s)", w.Code, http.StatusOK, w.Body)
	}

	places := hmMapData(t, w.Body.String())

	if !strings.Contains(w.Body.String(), strconv.Itoa(len(places))+" place") {
		t.Errorf("got body %s; want the count to match the %d places plotted", w.Body, len(places))
	}
}

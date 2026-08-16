package models

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testCoords(t *testing.T) (float64, float64) {
	t.Helper()

	n := time.Now().UnixNano()

	return 10 + float64(n%1_000_000)/1e6, 20 + float64((n/1_000_000)%1_000_000)/1e6
}

func newTestLocation(t *testing.T, pool *pgxpool.Pool, personID int, name string, lat, lng any) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO api_location (person_id, name, lat, lng) VALUES ($1, $2, $3, $4)`,
		personID, name, lat, lng)
	if err != nil {
		t.Fatal(err)
	}
}

func placeNames(p Place) []string {
	var names []string

	for _, person := range p.People {
		names = append(names, person.Name)
	}

	return names
}

func findPlace(places []Place, lat, lng float64) (Place, bool) {
	for _, p := range places {
		if p.Lat == lat && p.Lng == lng {
			return p, true
		}
	}

	return Place{}, false
}

func TestLocationPlacesGroupsByCoordinate(t *testing.T) {
	pool := newTestPool(t)
	model := &LocationModel{DB: pool}

	lat, lng := testCoords(t)

	firstID, firstName := newTestPerson(t, pool)
	secondID, secondName := newTestPerson(t, pool)

	newTestLocation(t, pool, firstID, "Shared Town", lat, lng)
	newTestLocation(t, pool, secondID, "Shared Town", lat, lng)

	places, err := model.Places(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	got, ok := findPlace(places, lat, lng)
	if !ok {
		t.Fatalf("got %d places; none at %f,%f", len(places), lat, lng)
	}
	if got.Name != "Shared Town" {
		t.Errorf("got name %q; want Shared Town", got.Name)
	}

	want := []string{firstName, secondName}
	slices.Sort(want)

	if !slices.Equal(placeNames(got), want) {
		t.Errorf("got people %v; want %v — two people at one coordinate are a single place", placeNames(got), want)
	}
}

func TestLocationPlacesSeparatesDistinctCoordinates(t *testing.T) {
	pool := newTestPool(t)
	model := &LocationModel{DB: pool}

	lat, lng := testCoords(t)

	firstID, firstName := newTestPerson(t, pool)
	secondID, secondName := newTestPerson(t, pool)

	newTestLocation(t, pool, firstID, "Here", lat, lng)
	newTestLocation(t, pool, secondID, "There", lat+0.5, lng)

	places, err := model.Places(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	here, ok := findPlace(places, lat, lng)
	if !ok {
		t.Fatalf("got no place at %f,%f", lat, lng)
	}

	there, ok := findPlace(places, lat+0.5, lng)
	if !ok {
		t.Fatalf("got no place at %f,%f", lat+0.5, lng)
	}

	if !slices.Equal(placeNames(here), []string{firstName}) {
		t.Errorf("got people %v; want just %q", placeNames(here), firstName)
	}
	if !slices.Equal(placeNames(there), []string{secondName}) {
		t.Errorf("got people %v; want just %q", placeNames(there), secondName)
	}
	if here.People[0].ID != firstID {
		t.Errorf("got id %d; want %d — the popup links to the person by id", here.People[0].ID, firstID)
	}
}

func TestLocationPlacesSkipsMissingCoordinates(t *testing.T) {
	pool := newTestPool(t)
	model := &LocationModel{DB: pool}

	id, name := newTestPerson(t, pool)
	newTestLocation(t, pool, id, "Nowhere In Particular", nil, nil)

	places, err := model.Places(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, p := range places {
		if slices.Contains(placeNames(p), name) {
			t.Fatalf("got %q at %f,%f; a location without coordinates cannot be plotted", name, p.Lat, p.Lng)
		}
	}
}

package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

func postForm(t *testing.T, values url.Values) *http.Request {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	err := r.ParseForm()
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func TestPersonFormFrom(t *testing.T) {
	f := personFormFrom(postForm(t, url.Values{
		"name":       {"  Ada Lovelace  "},
		"birthyear":  {" 1815 "},
		"birthplace": {"\tLondon\n"},
		"bio":        {"  a bio  "},
		"location":   {" London "},
		"lat":        {" 51.5 "},
		"lng":        {" -0.12 "},
	}))

	want := personForm{
		Name:       "Ada Lovelace",
		Birthyear:  "1815",
		Birthplace: "London",
		Bio:        "a bio",
		Location:   "London",
		Lat:        "51.5",
		Lng:        "-0.12",
	}

	if !reflect.DeepEqual(f, want) {
		t.Errorf("got %+v; want %+v — every field is trimmed at parse time", f, want)
	}
}

func TestPersonFormFromMissingFields(t *testing.T) {
	f := personFormFrom(postForm(t, url.Values{"name": {"Ada"}}))

	if !reflect.DeepEqual(f, personForm{Name: "Ada"}) {
		t.Errorf("got %+v; absent fields must read as empty, not panic", f)
	}
}

func TestNewPersonForm(t *testing.T) {
	lat, lng := 51.5, -0.12

	tests := []struct {
		name   string
		person models.Person
		want   personForm
	}{
		{
			"fully populated",
			models.Person{
				Summary:    models.Summary{Name: "Ada", Birthyear: 1815},
				Birthplace: "London",
				Bio:        "a bio",
				Location:   "London",
				Lat:        &lat,
				Lng:        &lng,
			},
			personForm{Name: "Ada", Birthyear: "1815", Birthplace: "London", Bio: "a bio", Location: "London", Lat: "51.5", Lng: "-0.12"},
		},
		{
			"unknown birthyear renders blank, not zero",
			models.Person{Summary: models.Summary{Name: "Ada"}},
			personForm{Name: "Ada"},
		},
		{
			"nil coordinates render blank",
			models.Person{Summary: models.Summary{Name: "Ada"}, Location: "London"},
			personForm{Name: "Ada", Location: "London"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newPersonForm(tt.person); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v; want %+v", got, tt.want)
			}
		})
	}
}

func TestNewPersonFormRoundTrip(t *testing.T) {
	f := personForm{Name: "Ada", Birthyear: "1815", Location: "London", Lat: "51.5", Lng: "-0.12"}

	p, ok := f.validate()
	if !ok {
		t.Fatalf("got errors %v", f.Errors)
	}

	got := newPersonForm(p)
	got.Errors = f.Errors

	if !reflect.DeepEqual(got, f) {
		t.Errorf("got %+v; want %+v — a valid form must survive the trip through models.Person", got, f)
	}
}

func TestRelativeFormFrom(t *testing.T) {
	f := relativeFormFrom(postForm(t, url.Values{"name": {" Ada "}, "relation": {"sibling"}}))

	if f.Person.Name != "Ada" {
		t.Errorf("got name %q; want Ada", f.Person.Name)
	}
	if f.Relation != "sibling" {
		t.Errorf("got relation %q; want sibling", f.Relation)
	}
}

func TestPersonFormValidate(t *testing.T) {
	thisYear := strconv.Itoa(time.Now().Year())
	nextYear := strconv.Itoa(time.Now().Year() + 1)
	long := strings.Repeat("a", 256)

	tests := []struct {
		name     string
		form     personForm
		wantErrs []string
	}{
		{"minimal", personForm{Name: "Ada"}, nil},
		{"blank name", personForm{}, []string{"Name"}},
		{"name at limit", personForm{Name: long[:255]}, nil},
		{"name too long", personForm{Name: long}, []string{"Name"}},
		{"birthyear this year", personForm{Name: "Ada", Birthyear: thisYear}, nil},
		{"birthyear in the future", personForm{Name: "Ada", Birthyear: nextYear}, []string{"Birthyear"}},
		{"birthyear zero", personForm{Name: "Ada", Birthyear: "0"}, []string{"Birthyear"}},
		{"birthyear not a number", personForm{Name: "Ada", Birthyear: "nineteen"}, []string{"Birthyear"}},
		{"birthplace too long", personForm{Name: "Ada", Birthplace: long}, []string{"Birthplace"}},
		{"location too long", personForm{Name: "Ada", Location: long}, []string{"Location"}},
		{"coordinates at the limit", personForm{Name: "Ada", Location: "Pole", Lat: "-90", Lng: "180"}, nil},
		{"lat out of range", personForm{Name: "Ada", Location: "Pole", Lat: "90.1"}, []string{"Lat"}},
		{"lng out of range", personForm{Name: "Ada", Location: "Pole", Lng: "-180.1"}, []string{"Lng"}},
		{"coordinates not numbers", personForm{Name: "Ada", Location: "Pole", Lat: "north", Lng: "east"}, []string{"Lat", "Lng"}},
		{"everything wrong at once", personForm{Birthyear: "x", Birthplace: long, Location: long, Lat: "x"}, []string{"Name", "Birthyear", "Birthplace", "Location", "Lat"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.form

			_, ok := f.validate()

			if ok != (len(tt.wantErrs) == 0) {
				t.Errorf("got ok %v with errors %v", ok, f.Errors)
			}
			if len(f.Errors) != len(tt.wantErrs) {
				t.Fatalf("got errors %v; want keys %v", f.Errors, tt.wantErrs)
			}

			for _, key := range tt.wantErrs {
				if f.Errors[key] == "" {
					t.Errorf("got no error for %s; have %v", key, f.Errors)
				}
			}
		})
	}
}

func TestPersonFormValidateValues(t *testing.T) {
	f := personForm{
		Name:       "Ada Lovelace",
		Birthyear:  "1815",
		Birthplace: "London",
		Bio:        "  keeps its leading spaces, trimming happens at parse time  ",
		Location:   "London",
		Lat:        "51.5",
		Lng:        "-0.12",
	}

	p, ok := f.validate()
	if !ok {
		t.Fatalf("got errors %v; want none", f.Errors)
	}

	if p.Name != f.Name || p.Birthplace != f.Birthplace || p.Bio != f.Bio || p.Location != f.Location {
		t.Errorf("got %+v; strings should pass through untouched", p)
	}
	if p.Birthyear != 1815 {
		t.Errorf("got birthyear %d; want 1815", p.Birthyear)
	}
	if p.Lat == nil || *p.Lat != 51.5 {
		t.Errorf("got lat %v; want 51.5", p.Lat)
	}
	if p.Lng == nil || *p.Lng != -0.12 {
		t.Errorf("got lng %v; want -0.12", p.Lng)
	}
}

func TestPersonFormValidateBlankCoordinatesAreNil(t *testing.T) {
	f := personForm{Name: "Ada", Location: "London"}

	p, ok := f.validate()
	if !ok {
		t.Fatalf("got errors %v; want none — blank coordinates are not an error", f.Errors)
	}
	if p.Lat != nil || p.Lng != nil {
		t.Errorf("got lat %v lng %v; want both nil", p.Lat, p.Lng)
	}
}

func TestPersonFormValidateBlankLocationDropsCoordinates(t *testing.T) {
	f := personForm{Name: "Ada", Lat: "51.5", Lng: "-0.12"}

	p, ok := f.validate()
	if !ok {
		t.Fatalf("got errors %v; want none", f.Errors)
	}
	if p.Lat != nil || p.Lng != nil {
		t.Errorf("got lat %v lng %v; a pin with no place name is not stored", p.Lat, p.Lng)
	}
}

func TestCoordinate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		limit float64
		want  *float64
	}{
		{"blank", "", 90, nil},
		{"zero", "0", 90, ptr(0)},
		{"negative", "-33.86", 90, ptr(-33.86)},
		{"at the positive limit", "180", 180, ptr(180)},
		{"at the negative limit", "-180", 180, ptr(-180)},
		{"just over", "90.000001", 90, nil},
		{"just under", "-90.000001", 90, nil},
		{"not a number", "somewhere", 90, nil},
		{"blank space", " ", 90, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := personForm{Errors: map[string]string{}}

			got := f.coordinate(tt.value, "Lat", tt.limit)

			switch {
			case tt.want == nil && got != nil:
				t.Fatalf("got %v; want nil", *got)
			case tt.want != nil && got == nil:
				t.Fatalf("got nil; want %v", *tt.want)
			case tt.want != nil && *got != *tt.want:
				t.Fatalf("got %v; want %v", *got, *tt.want)
			}

			wantErr := tt.want == nil && tt.value != ""
			if (f.Errors["Lat"] != "") != wantErr {
				t.Errorf("got errors %v; wantErr %v", f.Errors, wantErr)
			}
		})
	}
}

func TestRelativeFormValidate(t *testing.T) {
	for _, relation := range relations {
		t.Run("accepts "+relation, func(t *testing.T) {
			f := relativeForm{Person: personForm{Name: "Ada"}, Relation: relation}

			_, ok := f.validate()
			if !ok {
				t.Errorf("got errors %v; want none", f.Person.Errors)
			}
		})
	}

	for _, relation := range []string{"", "cousin", "Parent", "grandparent"} {
		t.Run("rejects "+strconv.Quote(relation), func(t *testing.T) {
			f := relativeForm{Person: personForm{Name: "Ada"}, Relation: relation}

			_, ok := f.validate()
			if ok {
				t.Fatal("got ok; want a Relation error")
			}
			if f.Person.Errors["Relation"] == "" {
				t.Errorf("got errors %v; want a Relation key", f.Person.Errors)
			}
		})
	}
}

func TestRelativeFormValidateKeepsPersonErrors(t *testing.T) {
	f := relativeForm{Relation: "cousin"}

	_, ok := f.validate()
	if ok {
		t.Fatal("got ok; want errors")
	}
	if f.Person.Errors["Name"] == "" || f.Person.Errors["Relation"] == "" {
		t.Errorf("got errors %v; want both Name and Relation — the relation check must not clear the person's", f.Person.Errors)
	}
}

func TestFactsAreASubsetOfRelations(t *testing.T) {
	for _, fact := range facts {
		if !slices.Contains(relations, fact) {
			t.Errorf("%q is linkable but not addable; the two lists have drifted", fact)
		}
	}
	if slices.Contains(facts, "sibling") {
		t.Error("sibling is a query, not a row — it must never be directly linkable")
	}
}

func ptr(f float64) *float64 {
	return &f
}

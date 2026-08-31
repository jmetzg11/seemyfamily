package main

import (
	"encoding/json"
	"strings"
	"testing"

	"seemyfamily.jmetzg11/internal/models"
)

func TestBuildMapData(t *testing.T) {
	places := []models.Place{
		{
			Name: "Shared Town",
			Lat:  40.5,
			Lng:  -74.25,
			People: []models.PlacePerson{
				{ID: 7, Name: "Ada"},
				{ID: 9, Name: "Grace"},
			},
		},
	}

	data, err := buildMapData(places)
	if err != nil {
		t.Fatal(err)
	}

	var got []mapPlace

	err = json.Unmarshal([]byte(data), &got)
	if err != nil {
		t.Fatalf("got %s; want valid JSON: %v", data, err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d places; want 1", len(got))
	}
	if got[0].Name != "Shared Town" || got[0].Lat != 40.5 || got[0].Lng != -74.25 {
		t.Errorf("got %+v; want the place unchanged", got[0])
	}
	if len(got[0].People) != 2 || got[0].People[0].ID != 7 || got[0].People[1].Name != "Grace" {
		t.Errorf("got people %+v; want both, with ids, in order", got[0].People)
	}
}

func TestBuildMapDataEmptyIsAnArray(t *testing.T) {
	data, err := buildMapData(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Errorf("got %s; want [] — the script parses it as an array either way", data)
	}
}

func TestBuildMapDataEscapesMarkup(t *testing.T) {
	places := []models.Place{
		{
			Name:   "</script><script>alert(1)</script>",
			People: []models.PlacePerson{{ID: 1, Name: "Ada"}},
		},
	}

	data, err := buildMapData(places)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "</script>") {
		t.Errorf("got %s; a place name must never close the data block", data)
	}
}

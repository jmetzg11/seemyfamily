package main

import (
	"encoding/json"
	"html/template"

	"seemyfamily.jmetzg11/internal/models"
)

const (
	defaultLat  = 40.505
	defaultLng  = -25
	defaultZoom = 3
)

type mapPerson struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type mapPlace struct {
	Name   string      `json:"name"`
	Lat    float64     `json:"lat"`
	Lng    float64     `json:"lng"`
	People []mapPerson `json:"people"`
}

func buildMapData(places []models.Place) (template.JS, error) {
	view := make([]mapPlace, 0, len(places))

	for _, p := range places {
		place := mapPlace{
			Name:   p.Name,
			Lat:    p.Lat,
			Lng:    p.Lng,
			People: make([]mapPerson, 0, len(p.People)),
		}

		for _, person := range p.People {
			place.People = append(place.People, mapPerson{ID: person.ID, Name: person.Name})
		}
		view = append(view, place)
	}

	b, err := json.Marshal(view)
	if err != nil {
		return "", err
	}

	return template.JS(b), nil
}

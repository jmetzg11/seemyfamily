package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"seemyfamily.jmetzg11/internal/models"
)

type loginForm struct {
	Name  string
	Next  string
	Error string
}

type personForm struct {
	Name       string
	Birthyear  string
	Birthplace string
	Bio        string
	Location   string
	Lat        string
	Lng        string
	Errors     map[string]string
}

func newPersonForm(p models.Person) personForm {
	f := personForm{
		Name:       p.Name,
		Birthplace: p.Birthplace,
		Bio:        p.Bio,
		Location:   p.Location,
	}

	if p.Birthyear != 0 {
		f.Birthyear = strconv.Itoa(p.Birthyear)
	}
	if p.Lat != nil {
		f.Lat = strconv.FormatFloat(*p.Lat, 'f', -1, 64)
	}
	if p.Lng != nil {
		f.Lng = strconv.FormatFloat(*p.Lng, 'f', -1, 64)
	}

	return f
}

func personFormFrom(r *http.Request) personForm {
	return personForm{
		Name:       strings.TrimSpace(r.PostForm.Get("name")),
		Birthyear:  strings.TrimSpace(r.PostForm.Get("birthyear")),
		Birthplace: strings.TrimSpace(r.PostForm.Get("birthplace")),
		Bio:        strings.TrimSpace(r.PostForm.Get("bio")),
		Location:   strings.TrimSpace(r.PostForm.Get("location")),
		Lat:        strings.TrimSpace(r.PostForm.Get("lat")),
		Lng:        strings.TrimSpace(r.PostForm.Get("lng")),
	}
}

func (f *personForm) coordinate(value, field string, limit float64) *float64 {
	if value == "" {
		return nil
	}

	n, err := strconv.ParseFloat(value, 64)
	if err != nil || n < -limit || n > limit {
		f.Errors[field] = "Must be a number between -" +
			strconv.FormatFloat(limit, 'f', -1, 64) + " and " +
			strconv.FormatFloat(limit, 'f', -1, 64) + "."
		return nil
	}

	return &n
}

func (f *personForm) validate() (models.Person, bool) {
	f.Errors = map[string]string{}

	var p models.Person

	p.Name = f.Name
	switch {
	case p.Name == "":
		f.Errors["Name"] = "Name cannot be blank."
	case len(p.Name) > 255:
		f.Errors["Name"] = "Name cannot be more than 255 characters long."
	}

	if f.Birthyear != "" {
		year, err := strconv.Atoi(f.Birthyear)
		if err != nil || year < 1 || year > time.Now().Year() {
			f.Errors["Birthyear"] = "Birth year must be a year in the past."
		}
		p.Birthyear = year
	}

	p.Birthplace = f.Birthplace
	if len(p.Birthplace) > 255 {
		f.Errors["Birthplace"] = "Birthplace cannot be more than 255 characters long."
	}

	p.Bio = f.Bio

	p.Location = f.Location
	if len(p.Location) > 255 {
		f.Errors["Location"] = "Location cannot be more than 255 characters long."
	}

	p.Lat = f.coordinate(f.Lat, "Lat", 90)
	p.Lng = f.coordinate(f.Lng, "Lng", 180)

	if p.Location == "" {
		p.Lat, p.Lng = nil, nil
	}

	return p, len(f.Errors) == 0
}

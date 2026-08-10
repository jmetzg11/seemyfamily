package main

import (
	"html/template"
	"io/fs"
	"net/url"
	"path/filepath"
	"strconv"

	"seemyfamily.jmetzg11/internal/models"
	"seemyfamily.jmetzg11/ui"
)

type loginForm struct {
	Name  string
	Next  string
	Error string
}

type templateData struct {
	Page            string
	MediaURL        string
	UserName        string
	IsAuthenticated bool

	Form loginForm

	Edits []models.Edit
	Chart chart
	Range string

	People      []models.Person
	Person      models.Person
	Relations   []models.RelationGroup
	Search      string
	Sort        string
	Dir         string
	CurrentPage int
	TotalPages  int
	Total       int
}

func (d templateData) values() url.Values {
	v := url.Values{}
	if d.Search != "" {
		v.Set("q", d.Search)
	}
	v.Set("sort", d.Sort)
	v.Set("dir", d.Dir)
	return v
}

func (d templateData) SortURL(column string) string {
	v := d.values()
	v.Set("sort", column)
	if d.Sort == column && d.Dir == "asc" {
		v.Set("dir", "desc")
	} else {
		v.Set("dir", "asc")
	}
	return "/?" + v.Encode()
}

func (d templateData) SortIcon(column string) string {
	if d.Sort != column {
		return ""
	}
	if d.Dir == "desc" {
		return " ▼"
	}
	return " ▲"
}

func (d templateData) PageURL(n int) string {
	v := d.values()
	v.Set("page", strconv.Itoa(n))
	return "/?" + v.Encode()
}

func (d templateData) HasPrev() bool {
	return d.CurrentPage > 1
}

func (d templateData) HasNext() bool {
	return d.CurrentPage < d.TotalPages
}

func (d templateData) PrevPage() int {
	return d.CurrentPage - 1
}

func (d templateData) NextPage() int {
	return d.CurrentPage + 1
}

func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	pages, err := fs.Glob(ui.Files, "html/pages/*.html")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)
		patterns := []string{
			"html/base.html",
			page,
		}

		ts, err := template.New(name).ParseFS(ui.Files, patterns...)
		if err != nil {
			return nil, err
		}
		cache[name] = ts
	}

	return cache, nil
}

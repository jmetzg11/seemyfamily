package main

import (
	"io/fs"
	"path/filepath"
	"testing"

	"seemyfamily.jmetzg11/ui"
)

func TestTemplateCache(t *testing.T) {
	cache, err := newTemplateCache()
	if err != nil {
		t.Fatal(err)
	}

	pages, err := fs.Glob(ui.Files, "html/pages/*.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("no pages found")
	}

	for _, page := range pages {
		name := filepath.Base(page)
		if cache[name] == nil {
			t.Errorf("%s missing from cache", name)
		}
	}
}

func TestSortURL(t *testing.T) {
	tests := []struct {
		name   string
		data   templateData
		column string
		want   string
	}{
		{"same column ascending flips to descending", templateData{Sort: "name", Dir: "asc"}, "name", "/?dir=desc&sort=name"},
		{"same column descending flips back", templateData{Sort: "name", Dir: "desc"}, "name", "/?dir=asc&sort=name"},
		{"a different column always starts ascending", templateData{Sort: "name", Dir: "desc"}, "birthyear", "/?dir=asc&sort=birthyear"},
		{"search rides along", templateData{Sort: "name", Dir: "asc", Search: "ada l"}, "name", "/?dir=desc&q=ada+l&sort=name"},
		{"blank search is omitted", templateData{Sort: "location", Dir: "asc"}, "location", "/?dir=desc&sort=location"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.SortURL(tt.column); got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestSortIcon(t *testing.T) {
	tests := []struct {
		name   string
		data   templateData
		column string
		want   string
	}{
		{"inactive column", templateData{Sort: "name", Dir: "asc"}, "birthyear", ""},
		{"active ascending", templateData{Sort: "name", Dir: "asc"}, "name", " ▲"},
		{"active descending", templateData{Sort: "name", Dir: "desc"}, "name", " ▼"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.SortIcon(tt.column); got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestPageURLKeepsSortState(t *testing.T) {
	d := templateData{Sort: "birthyear", Dir: "desc", Search: "jones"}

	if got, want := d.PageURL(3), "/?dir=desc&page=3&q=jones&sort=birthyear"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestPagination(t *testing.T) {
	tests := []struct {
		name     string
		data     templateData
		wantPrev bool
		wantNext bool
	}{
		{"only page", templateData{CurrentPage: 1, TotalPages: 1}, false, false},
		{"first of many", templateData{CurrentPage: 1, TotalPages: 4}, false, true},
		{"middle", templateData{CurrentPage: 2, TotalPages: 4}, true, true},
		{"last", templateData{CurrentPage: 4, TotalPages: 4}, true, false},
		{"no results", templateData{CurrentPage: 1, TotalPages: 0}, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.HasPrev(); got != tt.wantPrev {
				t.Errorf("HasPrev() = %v; want %v", got, tt.wantPrev)
			}
			if got := tt.data.HasNext(); got != tt.wantNext {
				t.Errorf("HasNext() = %v; want %v", got, tt.wantNext)
			}
			if tt.data.PrevPage() != tt.data.CurrentPage-1 || tt.data.NextPage() != tt.data.CurrentPage+1 {
				t.Error("page steps must be adjacent to the current page")
			}
		})
	}
}

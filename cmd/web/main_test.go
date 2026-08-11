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

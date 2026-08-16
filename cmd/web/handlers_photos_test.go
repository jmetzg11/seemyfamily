package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"seemyfamily.jmetzg11/internal/models"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 180, 255})
		}
	}

	buf := new(bytes.Buffer)

	err := png.Encode(buf, img)
	if err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func upload(t *testing.T, app *application, id int, description string, content []byte) *httptest.ResponseRecorder {
	t.Helper()

	buf := new(bytes.Buffer)
	mw := multipart.NewWriter(buf)

	err := mw.WriteField("description", description)
	if err != nil {
		t.Fatal(err)
	}

	if content != nil {
		part, err := mw.CreateFormFile("photo", "upload.png")
		if err != nil {
			t.Fatal(err)
		}

		_, err = part.Write(content)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = mw.Close()
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/person/"+strconv.Itoa(id)+"/photos", buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.SetPathValue("id", strconv.Itoa(id))
	r = r.WithContext(context.WithValue(r.Context(), userContextKey, models.User{Name: testUser}))

	w := httptest.NewRecorder()
	app.upload(w, r)

	return w
}

func assertNoPhotos(t *testing.T, app *application, id int) {
	t.Helper()

	photos, err := app.photos.ByPerson(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 0 {
		t.Errorf("got %d rows after a rejected upload; want 0", len(photos))
	}
}

func TestUploadStoresObjectAndRow(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	res := upload(t, app, id, "On the beach", testPNG(t, 2400, 1800))

	if res.Code != http.StatusSeeOther {
		t.Fatalf("got status %d; want %d — body: %s", res.Code, http.StatusSeeOther, res.Body)
	}
	if got, want := res.Header().Get("Location"), "/person/"+strconv.Itoa(id)+"/photos"; got != want {
		t.Errorf("got redirect %q; want %q", got, want)
	}

	photos, err := app.photos.ByPerson(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 1 {
		t.Fatalf("got %d rows; want 1", len(photos))
	}

	key := photos[0].Path

	prefix := strconv.Itoa(id) + "/"
	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, ".jpeg") {
		t.Errorf("got key %q; want %s<timestamp>.jpeg", key, prefix)
	}
	if strings.Contains(key, "photos/") || strings.Contains(key, app.bucket.Bucket) {
		t.Errorf("got key %q; keys carry no photos/ prefix and no bucket name", key)
	}

	body, contentType := fetch(t, app.bucket.PublicURL+"/"+key)

	if contentType != "image/jpeg" {
		t.Errorf("got Content-Type %q; want image/jpeg", contentType)
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if format != "jpeg" {
		t.Errorf("got stored format %q; want jpeg", format)
	}
	if cfg.Width != 800 || cfg.Height != 600 {
		t.Errorf("got stored size %dx%d; want 800x600 — the handler resizes before it uploads", cfg.Width, cfg.Height)
	}
}

func TestUploadRejectsNonImage(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	res := upload(t, app, id, "", []byte("this is not a photograph"))

	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d; want %d", res.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(res.Body.String(), "JPEG and PNG only") {
		t.Error("got no JPEG/PNG guidance on the page; a decode failure must not surface as a 500")
	}

	assertNoPhotos(t, app, id)
}

func TestUploadRejectsOversizeBody(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	res := upload(t, app, id, "", bytes.Repeat([]byte("x"), maxUploadBytes+1))

	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d; want %d", res.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(res.Body.String(), "too large") {
		t.Error("got no size guidance on the page")
	}

	assertNoPhotos(t, app, id)
}

func TestUploadRejectsMissingFile(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	res := upload(t, app, id, "no file attached", nil)

	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d; want %d", res.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(res.Body.String(), "Choose a photo") {
		t.Error("got no prompt to choose a photo on the page")
	}

	assertNoPhotos(t, app, id)
}

func TestUploadRejectsLongDescription(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	res := upload(t, app, id, strings.Repeat("a", 256), testPNG(t, 100, 100))

	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got status %d; want %d", res.Code, http.StatusUnprocessableEntity)
	}

	assertNoPhotos(t, app, id)
}

func TestGalleryRendersEmptyState(t *testing.T) {
	app := newTestApp(t)
	id := newTestPerson(t, app)

	r := httptest.NewRequest(http.MethodGet, "/person/"+strconv.Itoa(id)+"/photos", nil)
	r.SetPathValue("id", strconv.Itoa(id))

	w := httptest.NewRecorder()
	app.gallery(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "No photos yet") {
		t.Error("got no empty-state message for a person with no photos")
	}
}

func TestGalleryUnknownPerson(t *testing.T) {
	app := newTestApp(t)

	r := httptest.NewRequest(http.MethodGet, "/person/0/photos", nil)
	r.SetPathValue("id", "0")

	w := httptest.NewRecorder()
	app.gallery(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("got status %d; want %d", w.Code, http.StatusNotFound)
	}
}

func fetch(t *testing.T, url string) ([]byte, string) {
	t.Helper()

	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("got status %d fetching %s; want 200 — the bucket is public-read", res.StatusCode, url)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return body, res.Header.Get("Content-Type")
}

package images

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()

	buf := new(bytes.Buffer)

	err := png.Encode(buf, img)
	if err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func decode(t *testing.T, data []byte) (image.Image, string) {
	t.Helper()

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	return img, format
}

func rgb(img image.Image, x, y int) (int, int, int) {
	r, g, b, _ := img.At(x, y).RGBA()

	return int(r >> 8), int(g >> 8), int(b >> 8)
}

func near(got, want int) bool {
	return got-want <= 10 && want-got <= 10
}

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}

	return img
}

func TestResizeClamps(t *testing.T) {
	tests := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"landscape", 1600, 1200, 800, 600},
		{"portrait", 1200, 1600, 600, 800},
		{"square", 1000, 1000, 800, 800},
		{"already small stays put", 400, 300, 400, 300},
		{"exactly at the limit", 800, 800, 800, 800},
		{"extreme ratio keeps a pixel", 1600, 1, 800, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Resize(encodePNG(t, solid(tt.w, tt.h, color.RGBA{200, 100, 50, 255})))
			if err != nil {
				t.Fatal(err)
			}

			img, format := decode(t, out)
			if format != "jpeg" {
				t.Errorf("got format %q; want jpeg", format)
			}

			bounds := img.Bounds()
			if bounds.Dx() != tt.wantW || bounds.Dy() != tt.wantH {
				t.Errorf("got %dx%d; want %dx%d", bounds.Dx(), bounds.Dy(), tt.wantW, tt.wantH)
			}
		})
	}
}

func TestResizeAveragesRatherThanSamples(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1600, 400))
	for y := range 400 {
		for x := range 1600 {
			shade := uint8(0)
			if x%2 == 1 {
				shade = 255
			}
			src.Set(x, y, color.RGBA{shade, shade, shade, 255})
		}
	}

	out, err := Resize(encodePNG(t, src))
	if err != nil {
		t.Fatal(err)
	}

	img, _ := decode(t, out)

	for _, x := range []int{100, 400, 700} {
		r, g, b := rgb(img, x, 100)
		if !near(r, 127) || !near(g, 127) || !near(b, 127) {
			t.Errorf("at x=%d got rgb(%d,%d,%d); want mid grey — nearest-neighbour would give black or white", x, r, g, b)
		}
	}
}

func TestResizeFlattensAlphaOntoWhite(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 1000))
	for y := range 1000 {
		for x := range 1000 {
			src.Set(x, y, color.RGBA{0, 0, 0, 0})
		}
	}

	out, err := Resize(encodePNG(t, src))
	if err != nil {
		t.Fatal(err)
	}

	img, _ := decode(t, out)

	r, g, b := rgb(img, 400, 400)
	if !near(r, 255) || !near(g, 255) || !near(b, 255) {
		t.Errorf("got rgb(%d,%d,%d) for a fully transparent source; want white", r, g, b)
	}
}

func TestResizeAcceptsJPEG(t *testing.T) {
	buf := new(bytes.Buffer)

	err := jpeg.Encode(buf, solid(1200, 900, color.RGBA{20, 160, 90, 255}), nil)
	if err != nil {
		t.Fatal(err)
	}

	out, err := Resize(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	img, _ := decode(t, out)

	bounds := img.Bounds()
	if bounds.Dx() != 800 || bounds.Dy() != 600 {
		t.Errorf("got %dx%d; want 800x600", bounds.Dx(), bounds.Dy())
	}

	r, g, b := rgb(img, 400, 300)
	if !near(r, 20) || !near(g, 160) || !near(b, 90) {
		t.Errorf("got rgb(%d,%d,%d); want the source colour back", r, g, b)
	}
}

func TestResizeRejectsNonImages(t *testing.T) {
	_, err := Resize([]byte("this is not a photograph"))
	if err == nil {
		t.Fatal("got nil error for non-image bytes; want a decode failure")
	}
	t.Log(err)
}

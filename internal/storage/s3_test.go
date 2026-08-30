package storage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

func testClient(t *testing.T) *Client {
	t.Helper()

	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT is not set; run with: set -a; . ./.env; set +a; go test ./internal/storage/")
	}

	return &Client{
		Endpoint:  endpoint,
		PublicURL: os.Getenv("S3_PUBLIC_URL"),
		Region:    os.Getenv("S3_REGION"),
		Bucket:    os.Getenv("S3_BUCKET"),
		AccessKey: os.Getenv("S3_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_SECRET_KEY"),
	}
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{0x4a, 0x90, 0xd9, 0xff})

	buf := new(bytes.Buffer)

	err := jpeg.Encode(buf, img, nil)
	if err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func fetch(t *testing.T, url string) (int, []byte) {
	t.Helper()

	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return res.StatusCode, body
}

func TestPutDelete(t *testing.T) {
	c := testClient(t)
	ctx := context.Background()

	key := "test/" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".jpeg"
	want := jpegBytes(t)

	err := c.Put(ctx, key, "image/jpeg", want)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Delete(ctx, key)

	status, got := fetch(t, c.PublicURL+"/"+key)
	if status != http.StatusOK {
		t.Fatalf("got status %d after Put; want 200", status)
	}
	if string(got) != string(want) {
		t.Errorf("got body %q; want %q", got, want)
	}

	err = c.Delete(ctx, key)
	if err != nil {
		t.Fatal(err)
	}

	status, _ = fetch(t, c.PublicURL+"/"+key)
	if status != http.StatusNotFound {
		t.Errorf("got status %d after Delete; want 404", status)
	}
}

func TestPutRejectsBadSecret(t *testing.T) {
	c := testClient(t)
	c.SecretKey += "wrong"

	err := c.Put(context.Background(), "test/should-not-exist.txt", "text/plain", []byte("nope"))
	if err == nil {
		t.Fatal("got nil error from Put with a bad secret; want a signature failure")
	}
	t.Log(err)
}

func TestSignsTheRequestedPath(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{"no prefix", "", "/photos/7/1755280000.jpeg"},
		{"path prefix", "/storage/v1/s3", "/storage/v1/s3/photos/7/1755280000.jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *http.Request

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Clone(r.Context())
			}))
			defer srv.Close()

			c := &Client{
				Endpoint:  srv.URL + tt.endpoint,
				Region:    "us-east-1",
				Bucket:    "photos",
				AccessKey: "key",
				SecretKey: "secret",
			}

			body := []byte("body")

			err := c.Put(context.Background(), "7/1755280000.jpeg", "image/jpeg", body)
			if err != nil {
				t.Fatal(err)
			}

			if got.URL.EscapedPath() != tt.want {
				t.Errorf("requested path = %q; want %q", got.URL.EscapedPath(), tt.want)
			}

			now, err := time.Parse("20060102T150405Z", got.Header.Get("X-Amz-Date"))
			if err != nil {
				t.Fatal(err)
			}

			resigned, err := http.NewRequest(http.MethodPut, srv.URL+got.URL.EscapedPath(), nil)
			if err != nil {
				t.Fatal(err)
			}

			c.sign(resigned, got.URL.EscapedPath(), hashHex(body), now)

			if resigned.Header.Get("Authorization") != got.Header.Get("Authorization") {
				t.Error("signature does not cover the path the request was sent to")
			}
		})
	}
}

func TestEncodePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"unreserved", "/photos/7/1755280000.jpeg", "/photos/7/1755280000.jpeg"},
		{"space", "/photos/a b.jpeg", "/photos/a%20b.jpeg"},
		{"plus", "/photos/a+b.jpeg", "/photos/a%2Bb.jpeg"},
		{"tilde is literal", "/photos/a~b.jpeg", "/photos/a~b.jpeg"},
		{"non ascii", "/photos/é.jpeg", "/photos/%C3%A9.jpeg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodePath(tt.path)
			if got != tt.want {
				t.Errorf("encodePath(%q) = %q; want %q", tt.path, got, tt.want)
			}
		})
	}
}

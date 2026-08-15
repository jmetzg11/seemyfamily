package storage

import (
	"context"
	"io"
	"net/http"
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

	key := "test/" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".txt"
	want := []byte("seemyfamily storage round trip")

	err := c.Put(ctx, key, "text/plain", want)
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

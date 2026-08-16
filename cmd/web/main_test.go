package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"seemyfamily.jmetzg11/internal/models"
	"seemyfamily.jmetzg11/internal/storage"
	"seemyfamily.jmetzg11/ui"
)

const testUser = "go-test-uploader"

func newTestApp(t *testing.T) *application {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" || os.Getenv("S3_ENDPOINT") == "" {
		t.Skip("DATABASE_URL or S3_ENDPOINT is not set; run with: set -a; . ./.env; set +a; go test ./cmd/web/")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	templateCache, err := newTemplateCache()
	if err != nil {
		t.Fatal(err)
	}

	return &application{
		logger:        slog.New(slog.DiscardHandler),
		templateCache: templateCache,
		people:        &models.PersonModel{DB: pool},
		photos:        &models.PhotoModel{DB: pool},
		bucket: &storage.Client{
			Endpoint:  strings.TrimSuffix(os.Getenv("S3_ENDPOINT"), "/"),
			PublicURL: strings.TrimSuffix(os.Getenv("S3_PUBLIC_URL"), "/"),
			Region:    os.Getenv("S3_REGION"),
			Bucket:    os.Getenv("S3_BUCKET"),
			AccessKey: os.Getenv("S3_ACCESS_KEY"),
			SecretKey: os.Getenv("S3_SECRET_KEY"),
		},
	}
}

func newTestPerson(t *testing.T, app *application) int {
	t.Helper()

	ctx := context.Background()
	name := "Test Subject " + strconv.FormatInt(time.Now().UnixNano(), 10)

	var id int

	err := app.people.DB.QueryRow(ctx, `INSERT INTO api_person (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		photos, err := app.photos.ByPerson(ctx, id)
		if err != nil {
			t.Error(err)
		}

		for _, p := range photos {
			err = app.bucket.Delete(ctx, p.Path)
			if err != nil {
				t.Error(err)
			}
		}

		cleanup := []string{
			`DELETE FROM api_photo WHERE person_id = $1`,
			`DELETE FROM api_person WHERE id = $1`,
		}

		for _, query := range cleanup {
			_, err = app.people.DB.Exec(ctx, query, id)
			if err != nil {
				t.Error(err)
			}
		}

		_, err = app.people.DB.Exec(ctx, `DELETE FROM api_history WHERE username = $1`, testUser)
		if err != nil {
			t.Error(err)
		}
	})

	return id
}

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

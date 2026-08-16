package models

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const testUser = "go-test-uploader"

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set; run with: set -a; . ./.env; set +a; go test ./internal/models/")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func newTestPerson(t *testing.T, pool *pgxpool.Pool) (int, string) {
	t.Helper()

	ctx := context.Background()
	name := "Test Subject " + strconv.FormatInt(time.Now().UnixNano(), 10)

	var id int

	err := pool.QueryRow(ctx, `INSERT INTO api_person (name) VALUES ($1) RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanup := []string{
			`DELETE FROM api_photo WHERE person_id = $1`,
			`DELETE FROM api_person WHERE id = $1`,
		}

		for _, query := range cleanup {
			_, err := pool.Exec(ctx, query, id)
			if err != nil {
				t.Error(err)
			}
		}

		_, err := pool.Exec(ctx, `DELETE FROM api_history WHERE username = $1`, testUser)
		if err != nil {
			t.Error(err)
		}
	})

	return id, name
}

func TestPhotoInsertProfilePicOnlyOnTheFirst(t *testing.T) {
	pool := newTestPool(t)
	model := &PhotoModel{DB: pool}
	id, _ := newTestPerson(t, pool)
	ctx := context.Background()

	for _, key := range []string{"first.jpeg", "second.jpeg", "third.jpeg"} {
		err := model.Insert(ctx, id, strconv.Itoa(id)+"/"+key, "", testUser)
		if err != nil {
			t.Fatal(err)
		}
	}

	photos, err := model.ByPerson(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 3 {
		t.Fatalf("got %d photos; want 3", len(photos))
	}

	profilePics := 0
	for _, p := range photos {
		if p.ProfilePic {
			profilePics++
		}
	}
	if profilePics != 1 {
		t.Errorf("got %d profile pics; want exactly 1", profilePics)
	}

	if !photos[0].ProfilePic {
		t.Error("got the profile pic somewhere other than first; ByPerson orders it to the front")
	}
	if photos[0].Path != strconv.Itoa(id)+"/first.jpeg" {
		t.Errorf("got %q as the profile pic; want the first photo uploaded", photos[0].Path)
	}
}

func TestPhotoInsertStoresKeyVerbatim(t *testing.T) {
	pool := newTestPool(t)
	model := &PhotoModel{DB: pool}
	id, _ := newTestPerson(t, pool)
	ctx := context.Background()

	key := strconv.Itoa(id) + "/1755300000000000000.jpeg"

	err := model.Insert(ctx, id, key, "A description", testUser)
	if err != nil {
		t.Fatal(err)
	}

	photos, err := model.ByPerson(ctx, id)
	if err != nil {
		t.Fatal(err)
	}

	if photos[0].Path != key {
		t.Errorf("got path %q; want %q — no prefix may be added on the way in or out", photos[0].Path, key)
	}
	if photos[0].Description != "A description" {
		t.Errorf("got description %q; want %q", photos[0].Description, "A description")
	}
	if photos[0].Rotation != 0 {
		t.Errorf("got rotation %d; want 0", photos[0].Rotation)
	}
}

func TestPhotoInsertBlankDescriptionReadsBackEmpty(t *testing.T) {
	pool := newTestPool(t)
	model := &PhotoModel{DB: pool}
	id, _ := newTestPerson(t, pool)
	ctx := context.Background()

	err := model.Insert(ctx, id, "k.jpeg", "", testUser)
	if err != nil {
		t.Fatal(err)
	}

	var isNull bool

	err = pool.QueryRow(ctx, `SELECT description IS NULL FROM api_photo WHERE person_id = $1`, id).Scan(&isNull)
	if err != nil {
		t.Fatal(err)
	}
	if !isNull {
		t.Error("got a stored empty string; blank descriptions are written as NULL")
	}

	photos, err := model.ByPerson(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if photos[0].Description != "" {
		t.Errorf("got description %q; want an empty string back out", photos[0].Description)
	}
}

func TestPhotoInsertWritesHistory(t *testing.T) {
	pool := newTestPool(t)
	model := &PhotoModel{DB: pool}
	id, name := newTestPerson(t, pool)
	ctx := context.Background()

	err := model.Insert(ctx, id, "k.jpeg", "", testUser)
	if err != nil {
		t.Fatal(err)
	}

	var action, recipient string

	err = pool.QueryRow(ctx,
		`SELECT action, recipient FROM api_history WHERE username = $1 ORDER BY id DESC LIMIT 1`,
		testUser).Scan(&action, &recipient)
	if err != nil {
		t.Fatal(err)
	}

	if action != "added photo" {
		t.Errorf("got action %q; want %q", action, "added photo")
	}
	if recipient != name {
		t.Errorf("got recipient %q; want %q", recipient, name)
	}
}

func TestPhotoInsertUnknownPerson(t *testing.T) {
	pool := newTestPool(t)
	model := &PhotoModel{DB: pool}
	ctx := context.Background()

	err := model.Insert(ctx, 0, "0/orphan.jpeg", "", testUser)
	if !errors.Is(err, ErrNoRecord) {
		t.Fatalf("got %v; want ErrNoRecord", err)
	}

	var rows int

	err = pool.QueryRow(ctx, `SELECT count(*) FROM api_photo WHERE file_path = $1`, "0/orphan.jpeg").Scan(&rows)
	if err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Errorf("got %d rows for an unknown person; the transaction should have rolled back", rows)
	}
}

func TestPhotoByPersonEmpty(t *testing.T) {
	pool := newTestPool(t)
	model := &PhotoModel{DB: pool}
	id, _ := newTestPerson(t, pool)

	photos, err := model.ByPerson(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 0 {
		t.Errorf("got %d photos for a new person; want 0", len(photos))
	}
}

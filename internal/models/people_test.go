package models

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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
			`DELETE FROM api_parentchild WHERE parent_id = $1 OR child_id = $1`,
			`DELETE FROM api_marriage WHERE person_a_id = $1 OR person_b_id = $1`,
			`DELETE FROM api_location WHERE person_id = $1`,
			`DELETE FROM api_photo WHERE person_id = $1`,
			`DELETE FROM api_person WHERE id = $1`,
		}

		for _, query := range cleanup {
			_, err := pool.Exec(ctx, query, id)
			if err != nil {
				t.Error(err)
			}
		}

		_, err := pool.Exec(ctx, `DELETE FROM api_history WHERE recipient = $1`, name)
		if err != nil {
			t.Error(err)
		}
	})

	return id, name
}

func peopleExec(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()

	_, err := pool.Exec(context.Background(), query, args...)
	if err != nil {
		t.Fatal(err)
	}
}

func peopleCount(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()

	var count int

	err := pool.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	return count
}

func peopleHistoryCount(t *testing.T, pool *pgxpool.Pool, action, recipient string) int {
	t.Helper()

	return peopleCount(t, pool,
		`SELECT count(*) FROM api_history WHERE action = $1 AND recipient = $2`, action, recipient)
}

func peopleCleanHistory(t *testing.T, pool *pgxpool.Pool, recipient string) {
	t.Helper()

	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM api_history WHERE recipient = $1`, recipient)
		if err != nil {
			t.Error(err)
		}
	})
}

func peopleNames(people []Person) []string {
	names := make([]string, 0, len(people))
	for _, p := range people {
		names = append(names, p.Name)
	}

	return names
}

func peopleSortFixture(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()

	token := "Zsorted" + strconv.FormatInt(time.Now().UnixNano(), 10)
	fixtures := []struct {
		suffix     string
		birthyear  int
		birthplace string
		location   string
	}{
		{"Alpha", 2000, "Yuma", "Zurich"},
		{"Bravo", 1950, "Lima", "Madrid"},
		{"Cyril", 1900, "Cairo", "Ankara"},
	}

	for _, f := range fixtures {
		id, _ := newTestPerson(t, pool)
		peopleExec(t, pool, `UPDATE api_person SET name = $2, birthyear = $3, birthplace = $4 WHERE id = $1`,
			id, token+" "+f.suffix, f.birthyear, f.birthplace)
		peopleExec(t, pool, `INSERT INTO api_location (person_id, name) VALUES ($1, $2)`, id, f.location)
	}

	return token
}

func TestPersonList(t *testing.T) {
	pool := newTestPool(t)
	model := &PersonModel{DB: pool}
	token := peopleSortFixture(t, pool)

	tests := []struct {
		name   string
		sort   string
		dir    string
		limit  int
		offset int
		want   []string
	}{
		{"name ascending", "name", "asc", 10, 0, []string{"Alpha", "Bravo", "Cyril"}},
		{"name descending", "name", "desc", 10, 0, []string{"Cyril", "Bravo", "Alpha"}},
		{"birthyear ascending", "birthyear", "asc", 10, 0, []string{"Cyril", "Bravo", "Alpha"}},
		{"birthplace ascending", "birthplace", "asc", 10, 0, []string{"Cyril", "Bravo", "Alpha"}},
		{"location ascending", "location", "asc", 10, 0, []string{"Cyril", "Bravo", "Alpha"}},
		{"location descending", "location", "desc", 10, 0, []string{"Alpha", "Bravo", "Cyril"}},
		{"unknown sort falls back to name", "bio", "asc", 10, 0, []string{"Alpha", "Bravo", "Cyril"}},
		{"unknown direction falls back to ascending", "name", "sideways", 10, 0, []string{"Alpha", "Bravo", "Cyril"}},
		{"limit caps the page", "name", "asc", 2, 0, []string{"Alpha", "Bravo"}},
		{"offset skips the first page", "name", "asc", 2, 2, []string{"Cyril"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			people, err := model.List(context.Background(), token, tt.sort, tt.dir, tt.limit, tt.offset)
			if err != nil {
				t.Fatal(err)
			}

			want := make([]string, 0, len(tt.want))
			for _, suffix := range tt.want {
				want = append(want, token+" "+suffix)
			}

			got := peopleNames(people)
			if !slices.Equal(got, want) {
				t.Errorf("got %v; want %v", got, want)
			}
		})
	}
}

func TestPersonListCoalesce(t *testing.T) {
	pool := newTestPool(t)
	model := &PersonModel{DB: pool}
	ctx := context.Background()
	_, bare := newTestPerson(t, pool)

	people, err := model.List(ctx, bare, "name", "asc", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 {
		t.Fatalf("got %d people; want 1 because the fixture name is unique", len(people))
	}

	got := people[0]
	if got.Birthyear != 0 {
		t.Errorf("Birthyear = %d; want 0 for a NULL birthyear", got.Birthyear)
	}
	if got.Birthplace != "" {
		t.Errorf("Birthplace = %q; want \"\" for a NULL birthplace", got.Birthplace)
	}
	if got.Location != "" {
		t.Errorf("Location = %q; want \"\" when there is no location row", got.Location)
	}
	if got.Photo != "default.jpeg" {
		t.Errorf("Photo = %q; want \"default.jpeg\" when there is no profile photo", got.Photo)
	}
	if got.Rotation != 0 {
		t.Errorf("Rotation = %d; want 0 when there is no profile photo", got.Rotation)
	}

	id, name := newTestPerson(t, pool)
	peopleExec(t, pool,
		`INSERT INTO api_photo (person_id, file_path, profile_pic, rotation) VALUES ($1, 'family/pic.jpg', true, 90)`, id)

	people, err = model.List(ctx, name, "name", "asc", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 {
		t.Fatalf("got %d people; want 1 because the fixture name is unique", len(people))
	}
	if people[0].Photo != "family/pic.jpg" {
		t.Errorf("Photo = %q; want %q", people[0].Photo, "family/pic.jpg")
	}
	if people[0].Rotation != 90 {
		t.Errorf("Rotation = %d; want 90", people[0].Rotation)
	}
}

func TestPersonCount(t *testing.T) {
	pool := newTestPool(t)
	model := &PersonModel{DB: pool}
	ctx := context.Background()
	token := peopleSortFixture(t, pool)

	matched, err := model.Count(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if matched != 3 {
		t.Errorf("got %d; want 3 for the three fixture people sharing the token", matched)
	}

	all, err := model.Count(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if all < matched {
		t.Errorf("got %d for an empty search; want at least %d because the search narrows the table", all, matched)
	}

	none, err := model.Count(ctx, token+" Nobody")
	if err != nil {
		t.Fatal(err)
	}
	if none != 0 {
		t.Errorf("got %d; want 0 for a search matching nobody", none)
	}
}

func TestPersonGet(t *testing.T) {
	pool := newTestPool(t)
	model := &PersonModel{DB: pool}
	ctx := context.Background()

	t.Run("full record", func(t *testing.T) {
		id, name := newTestPerson(t, pool)
		peopleExec(t, pool, `UPDATE api_person SET birthyear = 1966, birthplace = 'Rome', bio = 'a bio' WHERE id = $1`, id)
		peopleExec(t, pool, `INSERT INTO api_location (person_id, name, lat, lng) VALUES ($1, 'Rome, IT', 41.9, 12.5)`, id)
		peopleExec(t, pool,
			`INSERT INTO api_photo (person_id, file_path, profile_pic, rotation) VALUES ($1, 'family/rome.jpg', true, 180)`, id)

		got, err := model.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != id || got.Name != name {
			t.Errorf("got id %d name %q; want id %d name %q", got.ID, got.Name, id, name)
		}
		if got.Birthyear != 1966 || got.Birthplace != "Rome" || got.Bio != "a bio" {
			t.Errorf("got %d %q %q; want 1966 \"Rome\" \"a bio\"", got.Birthyear, got.Birthplace, got.Bio)
		}
		if got.Location != "Rome, IT" {
			t.Errorf("Location = %q; want %q", got.Location, "Rome, IT")
		}
		if got.Lat == nil || *got.Lat != 41.9 {
			t.Errorf("Lat = %v; want 41.9", got.Lat)
		}
		if got.Lng == nil || *got.Lng != 12.5 {
			t.Errorf("Lng = %v; want 12.5", got.Lng)
		}
		if got.Photo != "family/rome.jpg" || got.Rotation != 180 {
			t.Errorf("got photo %q rotation %d; want %q 180", got.Photo, got.Rotation, "family/rome.jpg")
		}
	})

	t.Run("bare record coalesces", func(t *testing.T) {
		id, _ := newTestPerson(t, pool)

		got, err := model.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Birthyear != 0 || got.Birthplace != "" || got.Bio != "" || got.Location != "" {
			t.Errorf("got %d %q %q %q; want zero values from COALESCE", got.Birthyear, got.Birthplace, got.Bio, got.Location)
		}
		if got.Photo != "default.jpeg" || got.Rotation != 0 {
			t.Errorf("got photo %q rotation %d; want \"default.jpeg\" 0", got.Photo, got.Rotation)
		}
		if got.Lat != nil || got.Lng != nil {
			t.Errorf("got lat %v lng %v; want both nil because there is no location row", got.Lat, got.Lng)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		_, err := model.Get(ctx, -1)
		if !errors.Is(err, ErrNoRecord) {
			t.Errorf("got %v; want %v", err, ErrNoRecord)
		}
	})
}

func TestPersonUpdate(t *testing.T) {
	pool := newTestPool(t)
	model := &PersonModel{DB: pool}
	ctx := context.Background()

	t.Run("updates fields and inserts a location", func(t *testing.T) {
		id, name := newTestPerson(t, pool)
		renamed := name + " Renamed"
		peopleCleanHistory(t, pool, renamed)
		lat, lng := 41.9, 12.5

		err := model.Update(ctx, Person{
			Summary:    Summary{ID: id, Name: renamed, Birthyear: 1966},
			Birthplace: "Rome",
			Bio:        "a bio",
			Location:   "Rome, IT",
			Lat:        &lat,
			Lng:        &lng,
		}, testUser)
		if err != nil {
			t.Fatal(err)
		}

		got, err := model.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != renamed || got.Birthyear != 1966 || got.Birthplace != "Rome" || got.Bio != "a bio" {
			t.Errorf("got %q %d %q %q; want %q 1966 \"Rome\" \"a bio\"", got.Name, got.Birthyear, got.Birthplace, got.Bio, renamed)
		}
		if got.Location != "Rome, IT" || got.Lat == nil || *got.Lat != 41.9 || got.Lng == nil || *got.Lng != 12.5 {
			t.Errorf("got %q %v %v; want \"Rome, IT\" 41.9 12.5", got.Location, got.Lat, got.Lng)
		}
		if n := peopleHistoryCount(t, pool, "updated details", renamed); n != 1 {
			t.Errorf("got %d history rows; want 1 with action \"updated details\"", n)
		}
	})

	t.Run("blank fields become NULL and drop the location", func(t *testing.T) {
		id, name := newTestPerson(t, pool)
		peopleExec(t, pool, `UPDATE api_person SET birthyear = 1966, birthplace = 'Rome', bio = 'a bio' WHERE id = $1`, id)
		peopleExec(t, pool, `INSERT INTO api_location (person_id, name, lat, lng) VALUES ($1, 'Rome, IT', 41.9, 12.5)`, id)

		err := model.Update(ctx, Person{Summary: Summary{ID: id, Name: name}}, testUser)
		if err != nil {
			t.Fatal(err)
		}

		nulls := peopleCount(t, pool,
			`SELECT count(*) FROM api_person WHERE id = $1 AND birthyear IS NULL AND birthplace IS NULL AND bio IS NULL`, id)
		if nulls != 1 {
			t.Errorf("got %d rows with all-NULL optional fields; want 1 because NULLIF turns blanks into NULL", nulls)
		}

		locations := peopleCount(t, pool, `SELECT count(*) FROM api_location WHERE person_id = $1`, id)
		if locations != 0 {
			t.Errorf("got %d location rows; want 0 because a blank Location deletes the row", locations)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		err := model.Update(ctx, Person{Summary: Summary{ID: -1, Name: "No Such Person"}}, testUser)
		if !errors.Is(err, ErrNoRecord) {
			t.Errorf("got %v; want %v", err, ErrNoRecord)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		id, _ := newTestPerson(t, pool)
		_, taken := newTestPerson(t, pool)

		err := model.Update(ctx, Person{Summary: Summary{ID: id, Name: taken}}, testUser)
		if !errors.Is(err, ErrDuplicateName) {
			t.Fatalf("got %v; want %v", err, ErrDuplicateName)
		}
		if n := peopleHistoryCount(t, pool, "updated details", taken); n != 0 {
			t.Errorf("got %d history rows; want 0 because the update failed", n)
		}
	})

	t.Run("failure rolls back the whole update", func(t *testing.T) {
		id, name := newTestPerson(t, pool)
		renamed := name + " Renamed"
		peopleCleanHistory(t, pool, renamed)

		err := model.Update(ctx, Person{Summary: Summary{ID: id, Name: renamed}}, strings.Repeat("x", 101))
		if err == nil {
			t.Fatal("got nil; want an error because the username overflows api_history.username")
		}

		got, err := model.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != name {
			t.Errorf("got %q; want %q because the transaction rolled back", got.Name, name)
		}
		if n := peopleHistoryCount(t, pool, "updated details", renamed); n != 0 {
			t.Errorf("got %d history rows; want 0 because the transaction rolled back", n)
		}
	})
}

func TestPersonDelete(t *testing.T) {
	pool := newTestPool(t)
	model := &PersonModel{DB: pool}
	ctx := context.Background()

	t.Run("removes the person and its dependents", func(t *testing.T) {
		id, name := newTestPerson(t, pool)
		other, _ := newTestPerson(t, pool)
		low, high := min(id, other), max(id, other)
		peopleExec(t, pool, `INSERT INTO api_parentchild (parent_id, child_id) VALUES ($1, $2)`, id, other)
		peopleExec(t, pool, `INSERT INTO api_marriage (person_a_id, person_b_id) VALUES ($1, $2)`, low, high)
		peopleExec(t, pool, `INSERT INTO api_location (person_id, name) VALUES ($1, 'Rome, IT')`, id)
		peopleExec(t, pool,
			`INSERT INTO api_photo (person_id, file_path, profile_pic, rotation) VALUES ($1, 'family/rome.jpg', true, 0)`, id)

		err := model.Delete(ctx, id, testUser)
		if err != nil {
			t.Fatal(err)
		}

		_, err = model.Get(ctx, id)
		if !errors.Is(err, ErrNoRecord) {
			t.Errorf("got %v; want %v because the person is deleted", err, ErrNoRecord)
		}

		dependents := []string{
			`SELECT count(*) FROM api_parentchild WHERE parent_id = $1 OR child_id = $1`,
			`SELECT count(*) FROM api_marriage WHERE person_a_id = $1 OR person_b_id = $1`,
			`SELECT count(*) FROM api_location WHERE person_id = $1`,
			`SELECT count(*) FROM api_photo WHERE person_id = $1`,
		}
		for _, query := range dependents {
			if n := peopleCount(t, pool, query, id); n != 0 {
				t.Errorf("got %d rows for %q; want 0", n, query)
			}
		}

		if n := peopleHistoryCount(t, pool, "deleted profile", name); n != 1 {
			t.Errorf("got %d history rows; want 1 with action \"deleted profile\" for %q", n, name)
		}
	})

	t.Run("missing id", func(t *testing.T) {
		err := model.Delete(ctx, -1, testUser)
		if !errors.Is(err, ErrNoRecord) {
			t.Errorf("got %v; want %v", err, ErrNoRecord)
		}
	})
}

func TestAsDuplicateName(t *testing.T) {
	duplicate := &pgconn.PgError{Code: "23505"}
	foreignKey := &pgconn.PgError{Code: "23503"}
	plain := errors.New("boom")

	tests := []struct {
		name string
		err  error
		want error
	}{
		{"unique violation", duplicate, ErrDuplicateName},
		{"wrapped unique violation", fmt.Errorf("update: %w", duplicate), ErrDuplicateName},
		{"other pg error passes through", foreignKey, foreignKey},
		{"plain error passes through", plain, plain},
		{"nil passes through", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := asDuplicateName(tt.err)
			if got != tt.want {
				t.Errorf("got %v; want %v", got, tt.want)
			}
		})
	}
}

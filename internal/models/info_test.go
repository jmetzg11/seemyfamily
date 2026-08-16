package models

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func infoInsertHistory(t *testing.T, pool *pgxpool.Pool, recipient, action string, createdAt time.Time) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO api_history (created_at, username, action, recipient) VALUES ($1, $2, $3, $4)`,
		createdAt, testUser, action, recipient)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditsNewestFirst(t *testing.T) {
	pool := newTestPool(t)
	model := InfoModel{DB: pool}

	_, name := newTestPerson(t, pool)
	base := time.Now().Truncate(time.Microsecond)
	newest := base.Add(3 * time.Hour)
	middle := base.Add(2 * time.Hour)
	oldest := base.Add(1 * time.Hour)

	infoInsertHistory(t, pool, name, "middle action", middle)
	infoInsertHistory(t, pool, name, "oldest action", oldest)
	infoInsertHistory(t, pool, name, "newest action", newest)

	edits, err := model.Edits(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 3 {
		t.Fatalf("got %d edits; want 3", len(edits))
	}

	want := []Edit{
		{newest, testUser, "newest action", name},
		{middle, testUser, "middle action", name},
		{oldest, testUser, "oldest action", name},
	}

	for i, w := range want {
		got := edits[i]

		if !got.CreatedAt.Equal(w.CreatedAt) {
			t.Errorf("edit %d created_at: got %v; want %v; rows must come back newest first", i, got.CreatedAt, w.CreatedAt)
		}
		if got.Username != w.Username {
			t.Errorf("edit %d username: got %q; want %q", i, got.Username, w.Username)
		}
		if got.Action != w.Action {
			t.Errorf("edit %d action: got %q; want %q", i, got.Action, w.Action)
		}
		if got.Recipient != w.Recipient {
			t.Errorf("edit %d recipient: got %q; want %q", i, got.Recipient, w.Recipient)
		}
	}
}

func TestEditsLimit(t *testing.T) {
	pool := newTestPool(t)
	model := InfoModel{DB: pool}

	_, name := newTestPerson(t, pool)
	base := time.Now().Truncate(time.Microsecond)

	for i := 1; i <= 4; i++ {
		infoInsertHistory(t, pool, name, fmt.Sprintf("action %d", i), base.Add(time.Duration(i)*time.Hour))
	}

	tests := []struct {
		name  string
		limit int
	}{
		{"zero", 0},
		{"one", 1},
		{"two", 2},
		{"four", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edits, err := model.Edits(context.Background(), tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			if len(edits) != tt.limit {
				t.Fatalf("got %d edits; want %d; the table holds more rows than the limit", len(edits), tt.limit)
			}
		})
	}
}

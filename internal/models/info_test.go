package models

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const infoDateLayout = "2006-01-02"

func infoCurrentDate(t *testing.T, pool *pgxpool.Pool) time.Time {
	t.Helper()

	var today time.Time

	err := pool.QueryRow(context.Background(), `SELECT CURRENT_DATE`).Scan(&today)
	if err != nil {
		t.Fatal(err)
	}

	return today
}

func infoInsertHistory(t *testing.T, pool *pgxpool.Pool, recipient, action string, createdAt time.Time) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO api_history (created_at, username, action, recipient) VALUES ($1, $2, $3, $4)`,
		createdAt, testUser, action, recipient)
	if err != nil {
		t.Fatal(err)
	}
}

func infoInsertVisitors(t *testing.T, pool *pgxpool.Pool, dates ...time.Time) {
	t.Helper()

	ctx := context.Background()
	prefix := fmt.Sprintf("go-test-%d-", time.Now().UnixNano())

	for i, date := range dates {
		_, err := pool.Exec(ctx, `INSERT INTO api_visitor (ip_address, date) VALUES ($1, $2)`,
			fmt.Sprintf("%s%d", prefix, i), date)
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		_, err := pool.Exec(ctx, `DELETE FROM api_visitor WHERE ip_address LIKE $1`, prefix+"%")
		if err != nil {
			t.Error(err)
		}
	})
}

func infoCounts(t *testing.T, buckets []VisitorBucket) map[string]int {
	t.Helper()

	counts := make(map[string]int, len(buckets))

	for _, b := range buckets {
		counts[b.Start.Format(infoDateLayout)] = b.Count
	}

	return counts
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

func TestVisitorsBucketLayout(t *testing.T) {
	pool := newTestPool(t)
	model := InfoModel{DB: pool}
	today := infoCurrentDate(t, pool)

	tests := []struct {
		name      string
		days      int
		groupSize int
		want      int
	}{
		{"week by day", 7, 1, 7},
		{"month by five", 30, 5, 6},
		{"half year by month", 180, 30, 6},
		{"uneven group", 10, 3, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buckets, err := model.Visitors(context.Background(), tt.days, tt.groupSize)
			if err != nil {
				t.Fatal(err)
			}
			if len(buckets) != tt.want {
				t.Fatalf("got %d buckets; want %d; generate_series must emit every bucket", len(buckets), tt.want)
			}

			first := today.AddDate(0, 0, -(tt.days - 1))

			for i, b := range buckets {
				want := first.AddDate(0, 0, i*tt.groupSize)

				if !b.Start.Equal(want) {
					t.Errorf("bucket %d start: got %v; want %v; buckets step by groupSize from CURRENT_DATE-(days-1)", i, b.Start, want)
				}
			}
			if buckets[len(buckets)-1].Start.After(today) {
				t.Errorf("last bucket start: got %v; want no later than %v", buckets[len(buckets)-1].Start, today)
			}
		})
	}
}

func TestVisitorsCountsLandInCoveringBucket(t *testing.T) {
	tests := []struct {
		name      string
		days      int
		groupSize int
		index     int
	}{
		{"week by day", 7, 1, 1},
		{"month by five", 30, 5, 1},
		{"half year by month", 180, 30, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := newTestPool(t)
			model := InfoModel{DB: pool}
			ctx := context.Background()

			before, err := model.Visitors(ctx, tt.days, tt.groupSize)
			if err != nil {
				t.Fatal(err)
			}

			today := infoCurrentDate(t, pool)
			start := today.AddDate(0, 0, -(tt.days-1)+tt.index*tt.groupSize)
			end := start.AddDate(0, 0, tt.groupSize-1)

			infoInsertVisitors(t, pool, start, end)

			after, err := model.Visitors(ctx, tt.days, tt.groupSize)
			if err != nil {
				t.Fatal(err)
			}

			beforeCounts := infoCounts(t, before)
			target := start.Format(infoDateLayout)

			if len(after) != len(before) {
				t.Fatalf("got %d buckets after insert; want %d", len(after), len(before))
			}

			for _, b := range after {
				key := b.Start.Format(infoDateLayout)
				prev, ok := beforeCounts[key]
				if !ok {
					t.Fatalf("bucket %s appeared only after the insert; the window shifted mid-test", key)
				}

				want := 0
				if key == target {
					want = 2
				}
				if b.Count-prev != want {
					t.Errorf("bucket %s delta: got %d; want %d; visitors on %s and %s belong only to bucket %s",
						key, b.Count-prev, want, start.Format(infoDateLayout), end.Format(infoDateLayout), target)
				}
			}
		})
	}
}

func TestVisitorsEmptyBucketIsZero(t *testing.T) {
	pool := newTestPool(t)
	model := InfoModel{DB: pool}
	ctx := context.Background()

	var empty time.Time

	err := pool.QueryRow(ctx, `
		SELECT d::date
		FROM generate_series(CURRENT_DATE - 179, CURRENT_DATE, '1 day') d
		WHERE NOT EXISTS (SELECT 1 FROM api_visitor v WHERE v.date = d::date)
		LIMIT 1`).Scan(&empty)
	if err != nil {
		t.Skipf("no visitor-free day in the last 180: %v", err)
	}

	buckets, err := model.Visitors(ctx, 180, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 180 {
		t.Fatalf("got %d buckets; want 180", len(buckets))
	}

	counts := infoCounts(t, buckets)
	key := empty.Format(infoDateLayout)

	count, ok := counts[key]
	if !ok {
		t.Fatalf("bucket %s is missing; empty buckets must still be returned", key)
	}
	if count != 0 {
		t.Errorf("bucket %s: got %d; want 0; no visitor rows exist on that date", key, count)
	}
}

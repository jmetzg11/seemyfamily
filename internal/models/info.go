package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Edit struct {
	CreatedAt time.Time
	Username  string
	Action    string
	Recipient string
}

type VisitorBucket struct {
	Start time.Time
	Count int
}

type InfoModel struct {
	DB *pgxpool.Pool
}

const editsQuery = `
SELECT created_at, username, action, recipient
FROM api_history
ORDER BY created_at DESC
LIMIT $1`

func (m *InfoModel) Edits(ctx context.Context, limit int) ([]Edit, error) {
	rows, err := m.DB.Query(ctx, editsQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edits []Edit

	for rows.Next() {
		var e Edit

		err = rows.Scan(&e.CreatedAt, &e.Username, &e.Action, &e.Recipient)
		if err != nil {
			return nil, err
		}
		edits = append(edits, e)
	}

	return edits, rows.Err()
}

const visitorsQuery = `
WITH buckets AS (
    SELECT generate_series(
        CURRENT_DATE - make_interval(days => $1::int - 1),
        CURRENT_DATE,
        make_interval(days => $2::int)
    )::date AS start
)
SELECT b.start,
       (SELECT count(*)
        FROM api_visitor v
        WHERE v.date >= b.start
          AND v.date < b.start + make_interval(days => $2::int))
FROM buckets b
ORDER BY b.start`

func (m *InfoModel) Visitors(ctx context.Context, days, groupSize int) ([]VisitorBucket, error) {
	rows, err := m.DB.Query(ctx, visitorsQuery, days, groupSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []VisitorBucket

	for rows.Next() {
		var b VisitorBucket

		err = rows.Scan(&b.Start, &b.Count)
		if err != nil {
			return nil, err
		}
		buckets = append(buckets, b)
	}

	return buckets, rows.Err()
}

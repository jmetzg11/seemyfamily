package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Edit struct {
	CreatedAt time.Time
	Username  string
	Action    Action
	Recipient string
}

type InfoModel struct {
	DB *pgxpool.Pool
}

const editsQuery = `
SELECT created_at, username, action, recipient
FROM api_history
ORDER BY created_at DESC
LIMIT $1`

const recentByUserQuery = `
SELECT created_at, username, action, recipient
FROM api_history
WHERE username = $1 AND ` + recentWindow + `
ORDER BY created_at DESC, id DESC
LIMIT $2`

func (m *InfoModel) Edits(ctx context.Context, limit int) ([]Edit, error) {
	return m.edits(ctx, editsQuery, limit)
}

func (m *InfoModel) RecentByUser(ctx context.Context, username string, limit int) ([]Edit, error) {
	return m.edits(ctx, recentByUserQuery, username, limit)
}

func (m *InfoModel) edits(ctx context.Context, query string, args ...any) ([]Edit, error) {
	rows, err := m.DB.Query(ctx, query, args...)
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

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

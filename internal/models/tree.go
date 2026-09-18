package models

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Node struct {
	Name string
}

type TreeModel struct {
	DB *pgxpool.Pool
}

const treeQuery = `
SELECT p.name
FROM api_person p
WHERE p.id = $1`

func (m *TreeModel) GetTree(ctx context.Context, id int) (Node, error) {
	var n Node
	err := m.DB.QueryRow(ctx, treeQuery, id).Scan(
		&n.Name,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Node{}, ErrNoRecord
	}

	return n, err
}

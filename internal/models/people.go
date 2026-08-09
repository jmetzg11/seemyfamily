package models

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Person struct {
	ID         int
	Name       string
	Birthyear  int
	Birthplace string
	Location   string
	Photo      string
	Rotation   int
}

func (p Person) Degrees() int {
	return p.Rotation * 90
}

type PersonModel struct {
	DB *pgxpool.Pool
}

var sortColumns = map[string]string{
	"name":       "p.name",
	"location":   "l.name",
	"birthyear":  "p.birthyear",
	"birthplace": "p.birthplace",
}

const listQuery = `
SELECT p.id,
       p.name,
       COALESCE(p.birthyear, 0),
       COALESCE(p.birthplace, ''),
       COALESCE(l.name, ''),
       COALESCE(ph.file_path, 'default.jpeg'),
       COALESCE(ph.rotation, 0)
FROM api_person p
LEFT JOIN api_location l ON l.person_id = p.id
LEFT JOIN LATERAL (
    SELECT file_path, rotation
    FROM api_photo
    WHERE person_id = p.id AND profile_pic
    ORDER BY id
    LIMIT 1
) ph ON true
WHERE p.name ILIKE $1
ORDER BY %s %s, p.id
LIMIT $2 OFFSET $3`

const countQuery = `SELECT count(*) FROM api_person WHERE name ILIKE $1`

func (m *PersonModel) List(ctx context.Context, search, sort, dir string, limit, offset int) ([]Person, error) {
	column, ok := sortColumns[sort]
	if !ok {
		column = sortColumns["name"]
	}
	if dir != "desc" {
		dir = "asc"
	}

	query := fmt.Sprintf(listQuery, column, dir)

	rows, err := m.DB.Query(ctx, query, "%"+search+"%", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var people []Person
	for rows.Next() {
		var p Person
		err = rows.Scan(&p.ID, &p.Name, &p.Birthyear, &p.Birthplace, &p.Location, &p.Photo, &p.Rotation)
		if err != nil {
			return nil, err
		}
		people = append(people, p)
	}

	return people, rows.Err()
}

func (m *PersonModel) Count(ctx context.Context, search string) (int, error) {
	var total int
	err := m.DB.QueryRow(ctx, countQuery, "%"+search+"%").Scan(&total)
	return total, err
}

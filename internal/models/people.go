package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Summary struct {
	ID        int
	Name      string
	Birthyear int
	Photo     string
	Rotation  int
}

type Person struct {
	Summary
	Birthplace string
	Bio        string
	Location   string
	Lat        *float64
	Lng        *float64
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

const getQuery = `
SELECT p.id,
       p.name,
       COALESCE(p.birthyear, 0),
       COALESCE(p.birthplace, ''),
       COALESCE(p.bio, ''),
       COALESCE(l.name, ''),
       l.lat,
       l.lng,
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
WHERE p.id = $1`

func (m *PersonModel) Get(ctx context.Context, id int) (Person, error) {
	var p Person

	err := m.DB.QueryRow(ctx, getQuery, id).Scan(
		&p.ID, &p.Name, &p.Birthyear, &p.Birthplace, &p.Bio,
		&p.Location, &p.Lat, &p.Lng, &p.Photo, &p.Rotation)
	if errors.Is(err, pgx.ErrNoRows) {
		return Person{}, ErrNoRecord
	}

	return p, err
}

const updateQuery = `
UPDATE api_person
SET name = $2,
    birthyear = NULLIF($3::int, 0),
    birthplace = NULLIF($4, ''),
    bio = NULLIF($5, '')
WHERE id = $1`

const upsertLocationQuery = `
INSERT INTO api_location (person_id, name, lat, lng)
VALUES ($1, $2, $3, $4)
ON CONFLICT (person_id) DO UPDATE
SET name = EXCLUDED.name, lat = EXCLUDED.lat, lng = EXCLUDED.lng`

const deleteLocationQuery = `DELETE FROM api_location WHERE person_id = $1`

const historyQuery = `
INSERT INTO api_history (created_at, username, action, recipient)
VALUES (now(), $1, $2, $3)`

func (m *PersonModel) Update(ctx context.Context, p Person, username string) error {
	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, updateQuery, p.ID, p.Name, p.Birthyear, p.Birthplace, p.Bio)
	if err != nil {
		return asDuplicateName(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRecord
	}

	if p.Location == "" {
		_, err = tx.Exec(ctx, deleteLocationQuery, p.ID)
	} else {
		_, err = tx.Exec(ctx, upsertLocationQuery, p.ID, p.Location, p.Lat, p.Lng)
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, historyQuery, username, "updated details", p.Name)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

const idByNameQuery = `SELECT id FROM api_person WHERE name = $1`

const nameByIDQuery = `SELECT name FROM api_person WHERE id = $1`

var deleteDependentQueries = []string{
	`DELETE FROM api_parentchild WHERE parent_id = $1 OR child_id = $1`,
	`DELETE FROM api_marriage WHERE person_a_id = $1 OR person_b_id = $1`,
	`DELETE FROM api_location WHERE person_id = $1`,
	`DELETE FROM api_photo WHERE person_id = $1`,
}

const deletePersonQuery = `DELETE FROM api_person WHERE id = $1 RETURNING name`

func (m *PersonModel) Delete(ctx context.Context, id int, username string) error {
	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, query := range deleteDependentQueries {
		_, err = tx.Exec(ctx, query, id)
		if err != nil {
			return err
		}
	}

	var name string

	err = tx.QueryRow(ctx, deletePersonQuery, id).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoRecord
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, historyQuery, username, "deleted profile", name)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

const uniqueViolation = "23505"

func asDuplicateName(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return ErrDuplicateName
	}
	return err
}

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

type RelationGroup struct {
	Title  string
	People []Summary
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

var relationGroups = []struct {
	kind  string
	title string
}{
	{"parent", "Parents"},
	{"sibling", "Siblings"},
	{"half sibling", "Half siblings"},
	{"spouse", "Spouses"},
	{"child", "Children"},
}

const relationsQuery = `
WITH sibs AS (
    SELECT pc.child_id AS id,
           count(*) AS shared,
           (SELECT count(*) FROM api_parentchild WHERE child_id = pc.child_id) AS their_parents
    FROM api_parentchild pc
    WHERE pc.parent_id IN (SELECT parent_id FROM api_parentchild WHERE child_id = $1)
      AND pc.child_id <> $1
    GROUP BY pc.child_id
),
rel AS (
        SELECT 'parent' AS kind, parent_id AS id
        FROM api_parentchild WHERE child_id = $1
    UNION ALL
        SELECT 'child', child_id
        FROM api_parentchild WHERE parent_id = $1
    UNION ALL
        SELECT 'spouse', CASE WHEN person_a_id = $1 THEN person_b_id ELSE person_a_id END
        FROM api_marriage WHERE person_a_id = $1 OR person_b_id = $1
    UNION ALL
        SELECT CASE WHEN shared = 1
                     AND (SELECT count(*) FROM api_parentchild WHERE child_id = $1) >= 2
                     AND their_parents >= 2
                    THEN 'half sibling'
                    ELSE 'sibling'
               END,
               id
        FROM sibs
)
SELECT rel.kind,
       p.id,
       p.name,
       COALESCE(p.birthyear, 0),
       COALESCE(ph.file_path, 'default.jpeg'),
       COALESCE(ph.rotation, 0)
FROM rel
JOIN api_person p ON p.id = rel.id
LEFT JOIN LATERAL (
    SELECT file_path, rotation
    FROM api_photo
    WHERE person_id = p.id AND profile_pic
    ORDER BY id
    LIMIT 1
) ph ON true
ORDER BY p.birthyear NULLS LAST, p.name`

func (m *PersonModel) Relations(ctx context.Context, id int) ([]RelationGroup, error) {
	rows, err := m.DB.Query(ctx, relationsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := map[string][]Summary{}

	for rows.Next() {
		var kind string
		var r Summary

		err = rows.Scan(&kind, &r.ID, &r.Name, &r.Birthyear, &r.Photo, &r.Rotation)
		if err != nil {
			return nil, err
		}
		found[kind] = append(found[kind], r)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	var groups []RelationGroup

	for _, g := range relationGroups {
		if len(found[g.kind]) > 0 {
			groups = append(groups, RelationGroup{Title: g.title, People: found[g.kind]})
		}
	}

	return groups, nil
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

const insertPersonQuery = `
INSERT INTO api_person (name, birthyear, birthplace, bio)
VALUES ($1, NULLIF($2::int, 0), NULLIF($3, ''), NULLIF($4, ''))
RETURNING id`

const insertParentChildQuery = `
INSERT INTO api_parentchild (parent_id, child_id)
VALUES ($1, $2)`

const insertMarriageQuery = `
INSERT INTO api_marriage (person_a_id, person_b_id)
VALUES (LEAST($1, $2), GREATEST($1, $2))`

const copyParentsQuery = `
INSERT INTO api_parentchild (parent_id, child_id)
SELECT parent_id, $2
FROM api_parentchild
WHERE child_id = $1`

const insertUnknownParentQuery = `
INSERT INTO api_person (name)
SELECT left('Unknown parent of ' || name, 255)
FROM api_person
WHERE id = $1
RETURNING id`

func linkUnknownParent(ctx context.Context, tx pgx.Tx, siblings ...int) error {
	var parentID int

	err := tx.QueryRow(ctx, insertUnknownParentQuery, siblings[0]).Scan(&parentID)
	if err != nil {
		return err
	}

	for _, child := range siblings {
		_, err = tx.Exec(ctx, insertParentChildQuery, parentID, child)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *PersonModel) AddRelative(ctx context.Context, p Person, relativeID int, relation, username string) error {
	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var id int

	err = tx.QueryRow(ctx, insertPersonQuery, p.Name, p.Birthyear, p.Birthplace, p.Bio).Scan(&id)
	if err != nil {
		return asDuplicateName(err)
	}

	if p.Location != "" {
		_, err = tx.Exec(ctx, upsertLocationQuery, id, p.Location, p.Lat, p.Lng)
		if err != nil {
			return err
		}
	}

	switch relation {
	case "parent":
		_, err = tx.Exec(ctx, insertParentChildQuery, id, relativeID)
	case "child":
		_, err = tx.Exec(ctx, insertParentChildQuery, relativeID, id)
	case "spouse":
		_, err = tx.Exec(ctx, insertMarriageQuery, relativeID, id)
	case "sibling":
		var tag pgconn.CommandTag
		tag, err = tx.Exec(ctx, copyParentsQuery, relativeID, id)
		if err == nil && tag.RowsAffected() == 0 {
			err = linkUnknownParent(ctx, tx, relativeID, id)
		}
	default:
		return fmt.Errorf("models: unknown relation %q", relation)
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, historyQuery, username, "created", p.Name)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

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

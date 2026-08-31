package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type RelationGroup struct {
	Title  string
	People []Summary
}

type Fact struct {
	Relation string
	Person   Summary
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

const factsQuery = `
SELECT rel.kind,
       p.id,
       p.name,
       COALESCE(p.birthyear, 0),
       COALESCE(ph.file_path, 'default.jpeg'),
       COALESCE(ph.rotation, 0)
FROM (
        SELECT 'parent' AS kind, parent_id AS id
        FROM api_parentchild WHERE child_id = $1
    UNION ALL
        SELECT 'child', child_id
        FROM api_parentchild WHERE parent_id = $1
    UNION ALL
        SELECT 'spouse', CASE WHEN person_a_id = $1 THEN person_b_id ELSE person_a_id END
        FROM api_marriage WHERE person_a_id = $1 OR person_b_id = $1
) rel
JOIN api_person p ON p.id = rel.id
LEFT JOIN LATERAL (
    SELECT file_path, rotation
    FROM api_photo
    WHERE person_id = p.id AND profile_pic
    ORDER BY id
    LIMIT 1
) ph ON true
ORDER BY p.birthyear NULLS LAST, p.name`

func (m *PersonModel) Facts(ctx context.Context, id int) ([]Fact, error) {
	rows, err := m.DB.Query(ctx, factsQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []Fact

	for rows.Next() {
		var f Fact

		err = rows.Scan(&f.Relation, &f.Person.ID, &f.Person.Name,
			&f.Person.Birthyear, &f.Person.Photo, &f.Person.Rotation)
		if err != nil {
			return nil, err
		}
		facts = append(facts, f)
	}

	return facts, rows.Err()
}

const namesQuery = `SELECT name FROM api_person WHERE id <> $1 ORDER BY name`

func (m *PersonModel) Names(ctx context.Context, exclude int) ([]string, error) {
	rows, err := m.DB.Query(ctx, namesQuery, exclude)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string

	for rows.Next() {
		var name string

		err = rows.Scan(&name)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}

	return names, rows.Err()
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
VALUES (LEAST($1::bigint, $2::bigint), GREATEST($1::bigint, $2::bigint))`

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

const deleteParentChildQuery = `
DELETE FROM api_parentchild WHERE parent_id = $1 AND child_id = $2`

const deleteMarriageQuery = `
DELETE FROM api_marriage
WHERE person_a_id = LEAST($1::bigint, $2::bigint)
  AND person_b_id = GREATEST($1::bigint, $2::bigint)`

func (m *PersonModel) Link(ctx context.Context, id int, name, relation, username string) error {
	return m.relate(ctx, id, name, relation, username, true)
}

func (m *PersonModel) Unlink(ctx context.Context, id int, name, relation, username string) error {
	return m.relate(ctx, id, name, relation, username, false)
}

func (m *PersonModel) relate(ctx context.Context, id int, name, relation, username string, link bool) error {
	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var otherID int

	err = tx.QueryRow(ctx, idByNameQuery, name).Scan(&otherID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoRecord
	}
	if err != nil {
		return err
	}
	if otherID == id {
		return ErrSelfLink
	}

	verb := "removed"

	if link {
		verb = "added"
		err = insertFact(ctx, tx, id, otherID, relation)
	} else {
		err = deleteFact(ctx, tx, id, otherID, relation)
	}
	if err != nil {
		return err
	}

	var subject string

	err = tx.QueryRow(ctx, nameByIDQuery, id).Scan(&subject)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, historyQuery, username, verb+" "+relation, subject)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func insertFact(ctx context.Context, tx pgx.Tx, id, otherID int, relation string) error {
	var err error

	switch relation {
	case "parent":
		_, err = tx.Exec(ctx, insertParentChildQuery, otherID, id)
	case "child":
		_, err = tx.Exec(ctx, insertParentChildQuery, id, otherID)
	case "spouse":
		_, err = tx.Exec(ctx, insertMarriageQuery, id, otherID)
	default:
		return fmt.Errorf("models: unknown relation %q", relation)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return ErrAlreadyLinked
	}

	return err
}

func deleteFact(ctx context.Context, tx pgx.Tx, id, otherID int, relation string) error {
	var tag pgconn.CommandTag
	var err error

	switch relation {
	case "parent":
		tag, err = tx.Exec(ctx, deleteParentChildQuery, otherID, id)
	case "child":
		tag, err = tx.Exec(ctx, deleteParentChildQuery, id, otherID)
	case "spouse":
		tag, err = tx.Exec(ctx, deleteMarriageQuery, id, otherID)
	default:
		return fmt.Errorf("models: unknown relation %q", relation)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRecord
	}

	return nil
}

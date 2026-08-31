package models

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Photo struct {
	ID          int
	Description string
	Path        string
	ProfilePic  bool
	Rotation    int
}

type PhotoModel struct {
	DB *pgxpool.Pool
}

const photosByPersonQuery = `
SELECT id,
       COALESCE(description, ''),
       COALESCE(file_path, 'default.jpeg'),
       profile_pic,
       rotation
FROM api_photo
WHERE person_id = $1
ORDER BY profile_pic DESC, id`

func (m *PhotoModel) ByPerson(ctx context.Context, personID int) ([]Photo, error) {
	rows, err := m.DB.Query(ctx, photosByPersonQuery, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []Photo

	for rows.Next() {
		var p Photo

		err = rows.Scan(&p.ID, &p.Description, &p.Path, &p.ProfilePic, &p.Rotation)
		if err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}

	return photos, rows.Err()
}

const insertPhotoQuery = `
INSERT INTO api_photo (person_id, description, file_path, profile_pic, rotation)
VALUES ($1, NULLIF($2, ''), $3,
        NOT EXISTS (SELECT 1 FROM api_photo WHERE person_id = $1),
        0)`

func (m *PhotoModel) Insert(ctx context.Context, personID int, key, description, username string) error {
	tx, err := m.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var name string

	err = tx.QueryRow(ctx, nameByIDQuery, personID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoRecord
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, insertPhotoQuery, personID, description, key)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, historyQuery, username, "added photo", name)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

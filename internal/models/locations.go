package models

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PlacePerson struct {
	ID   int
	Name string
}

type Place struct {
	Name   string
	Lat    float64
	Lng    float64
	People []PlacePerson
}

type LocationModel struct {
	DB *pgxpool.Pool
}

const placesQuery = `
SELECT min(l.name),
       l.lat,
       l.lng,
       array_agg(p.id ORDER BY p.name, p.id),
       array_agg(p.name ORDER BY p.name, p.id)
FROM api_location l
JOIN api_person p ON p.id = l.person_id
WHERE l.lat IS NOT NULL AND l.lng IS NOT NULL
GROUP BY l.lat, l.lng
ORDER BY l.lat, l.lng`

func (m *LocationModel) Places(ctx context.Context) ([]Place, error) {
	rows, err := m.DB.Query(ctx, placesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var places []Place

	for rows.Next() {
		var (
			p     Place
			ids   []int
			names []string
		)

		err = rows.Scan(&p.Name, &p.Lat, &p.Lng, &ids, &names)
		if err != nil {
			return nil, err
		}

		for i, id := range ids {
			p.People = append(p.People, PlacePerson{ID: id, Name: names[i]})
		}
		places = append(places, p)
	}

	return places, rows.Err()
}

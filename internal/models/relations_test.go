package models

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var relCounter atomic.Int64

func relModel(pool *pgxpool.Pool) *PersonModel {
	return &PersonModel{DB: pool}
}

func relUnique(prefix string) string {
	return fmt.Sprintf("%s %d-%d", prefix, time.Now().UnixNano(), relCounter.Add(1))
}

func relCleanupName(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()

		var id int

		err := pool.QueryRow(ctx, `SELECT id FROM api_person WHERE name = $1`, name).Scan(&id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			t.Error(err)
		}
		if err == nil {
			queries := []string{
				`DELETE FROM api_parentchild WHERE parent_id = $1 OR child_id = $1`,
				`DELETE FROM api_marriage WHERE person_a_id = $1 OR person_b_id = $1`,
				`DELETE FROM api_location WHERE person_id = $1`,
				`DELETE FROM api_photo WHERE person_id = $1`,
				`DELETE FROM api_person WHERE id = $1`,
			}
			for _, query := range queries {
				_, err := pool.Exec(ctx, query, id)
				if err != nil {
					t.Error(err)
				}
			}
		}

		_, err = pool.Exec(ctx, `DELETE FROM api_history WHERE recipient = $1`, name)
		if err != nil {
			t.Error(err)
		}
	})
}

func relName(t *testing.T, pool *pgxpool.Pool, prefix string) string {
	t.Helper()
	name := relUnique(prefix)
	relCleanupName(t, pool, name)

	return name
}

func relRename(t *testing.T, pool *pgxpool.Pool, id int, prefix string) string {
	t.Helper()
	name := relUnique(prefix)

	_, err := pool.Exec(context.Background(), `UPDATE api_person SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		t.Fatal(err)
	}

	return name
}

func relBirthyear(t *testing.T, pool *pgxpool.Pool, id, year int) {
	t.Helper()

	_, err := pool.Exec(context.Background(), `UPDATE api_person SET birthyear = $2 WHERE id = $1`, id, year)
	if err != nil {
		t.Fatal(err)
	}
}

func relParent(t *testing.T, pool *pgxpool.Pool, parentID, childID int) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO api_parentchild (parent_id, child_id) VALUES ($1, $2)`, parentID, childID)
	if err != nil {
		t.Fatal(err)
	}
}

func relMarry(t *testing.T, pool *pgxpool.Pool, a, b int) {
	t.Helper()

	lo, hi := a, b
	if lo > hi {
		lo, hi = hi, lo
	}

	_, err := pool.Exec(context.Background(),
		`INSERT INTO api_marriage (person_a_id, person_b_id) VALUES ($1, $2)`, lo, hi)
	if err != nil {
		t.Fatal(err)
	}
}

func relPersonWithID(t *testing.T, pool *pgxpool.Pool, id int) string {
	t.Helper()
	name := relUnique("Rel Fixed")
	relCleanupName(t, pool, name)

	_, err := pool.Exec(context.Background(),
		`INSERT INTO api_person (id, name) VALUES ($1, $2)`, id, name)
	if err != nil {
		t.Fatal(err)
	}

	return name
}

func relPersonID(t *testing.T, pool *pgxpool.Pool, name string) (int, bool) {
	t.Helper()

	var id int

	err := pool.QueryRow(context.Background(), `SELECT id FROM api_person WHERE name = $1`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatal(err)
	}

	return id, true
}

func relParentIDs(t *testing.T, pool *pgxpool.Pool, childID int) []int {
	t.Helper()

	rows, err := pool.Query(context.Background(),
		`SELECT parent_id FROM api_parentchild WHERE child_id = $1 ORDER BY parent_id`, childID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var ids []int

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return ids
}

func relChildIDs(t *testing.T, pool *pgxpool.Pool, parentID int) []int {
	t.Helper()

	rows, err := pool.Query(context.Background(),
		`SELECT child_id FROM api_parentchild WHERE parent_id = $1 ORDER BY child_id`, parentID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var ids []int

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return ids
}

func relMarriageRow(t *testing.T, pool *pgxpool.Pool, x, y int) (int, int, bool) {
	t.Helper()

	var a, b int

	err := pool.QueryRow(context.Background(),
		`SELECT person_a_id, person_b_id FROM api_marriage
		 WHERE (person_a_id = $1 AND person_b_id = $2) OR (person_a_id = $2 AND person_b_id = $1)`,
		x, y).Scan(&a, &b)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, false
	}
	if err != nil {
		t.Fatal(err)
	}

	return a, b, true
}

func relHistoryCount(t *testing.T, pool *pgxpool.Pool, action, recipient string) int {
	t.Helper()

	var n int

	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM api_history WHERE username = $1 AND action = $2 AND recipient = $3`,
		testUser, action, recipient).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}

	return n
}

func relTitles(groups []RelationGroup) []string {
	titles := make([]string, len(groups))
	for i, g := range groups {
		titles[i] = g.Title
	}

	return titles
}

func relGroupNames(t *testing.T, groups []RelationGroup, title string) []string {
	t.Helper()

	for _, g := range groups {
		if g.Title == title {
			names := make([]string, len(g.People))
			for i, p := range g.People {
				names[i] = p.Name
			}
			return names
		}
	}
	t.Fatalf("got no %q group; want one in %v", title, relTitles(groups))

	return nil
}

func relTitleOf(groups []RelationGroup, id int) string {
	for _, g := range groups {
		for _, p := range g.People {
			if p.ID == id {
				return g.Title
			}
		}
	}

	return ""
}

func relEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func TestRelationsGroupOrderAndSorting(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)
	p1ID, _ := newTestPerson(t, pool)
	p2ID, _ := newTestPerson(t, pool)
	p3ID, _ := newTestPerson(t, pool)
	sibID, _ := newTestPerson(t, pool)
	halfID, _ := newTestPerson(t, pool)
	spouseID, _ := newTestPerson(t, pool)
	olderChildID, _ := newTestPerson(t, pool)
	unknownChildID, _ := newTestPerson(t, pool)

	p1Name := relRename(t, pool, p1ID, "Rel ZParent")
	p2Name := relRename(t, pool, p2ID, "Rel AParent")
	olderChildName := relRename(t, pool, olderChildID, "Rel ZChild")
	unknownChildName := relRename(t, pool, unknownChildID, "Rel AChild")
	relBirthyear(t, pool, p1ID, 1930)
	relBirthyear(t, pool, p2ID, 1930)
	relBirthyear(t, pool, olderChildID, 1980)

	relParent(t, pool, p1ID, subjectID)
	relParent(t, pool, p2ID, subjectID)
	relParent(t, pool, p1ID, sibID)
	relParent(t, pool, p2ID, sibID)
	relParent(t, pool, p1ID, halfID)
	relParent(t, pool, p3ID, halfID)
	relMarry(t, pool, subjectID, spouseID)
	relParent(t, pool, subjectID, olderChildID)
	relParent(t, pool, subjectID, unknownChildID)

	groups, err := m.Relations(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}

	wantTitles := []string{"Parents", "Siblings", "Half siblings", "Spouses", "Children"}
	if !relEqual(relTitles(groups), wantTitles) {
		t.Fatalf("got %v; want %v", relTitles(groups), wantTitles)
	}

	gotParents := relGroupNames(t, groups, "Parents")
	if !relEqual(gotParents, []string{p2Name, p1Name}) {
		t.Errorf("got %v; want %v, equal birthyears must fall back to name order", gotParents, []string{p2Name, p1Name})
	}

	gotChildren := relGroupNames(t, groups, "Children")
	wantChildren := []string{olderChildName, unknownChildName}
	if !relEqual(gotChildren, wantChildren) {
		t.Errorf("got %v; want %v, a null birthyear must sort last despite the earlier name", gotChildren, wantChildren)
	}

	if got := relTitleOf(groups, sibID); got != "Siblings" {
		t.Errorf("got %q; want %q for a person sharing both parents", got, "Siblings")
	}
	if got := relTitleOf(groups, halfID); got != "Half siblings" {
		t.Errorf("got %q; want %q for a person sharing exactly one of two parents", got, "Half siblings")
	}
}

func TestRelationsOmitsEmptyGroups(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)
	spouseID, spouseName := newTestPerson(t, pool)
	relMarry(t, pool, subjectID, spouseID)

	groups, err := m.Relations(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if !relEqual(relTitles(groups), []string{"Spouses"}) {
		t.Fatalf("got %v; want [Spouses], groups with no people must be omitted", relTitles(groups))
	}
	if got := relGroupNames(t, groups, "Spouses"); !relEqual(got, []string{spouseName}) {
		t.Errorf("got %v; want %v", got, []string{spouseName})
	}
}

func TestRelationsNoEdges(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)

	groups, err := m.Relations(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Errorf("got %d groups; want 0", len(groups))
	}
}

func TestRelationsHalfSiblingRule(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	t.Run("one shared parent and both have two", func(t *testing.T) {
		subjectID, _ := newTestPerson(t, pool)
		otherID, _ := newTestPerson(t, pool)
		sharedID, _ := newTestPerson(t, pool)
		subjectOnlyID, _ := newTestPerson(t, pool)
		otherOnlyID, _ := newTestPerson(t, pool)

		relParent(t, pool, sharedID, subjectID)
		relParent(t, pool, subjectOnlyID, subjectID)
		relParent(t, pool, sharedID, otherID)
		relParent(t, pool, otherOnlyID, otherID)

		groups, err := m.Relations(ctx, subjectID)
		if err != nil {
			t.Fatal(err)
		}
		if got := relTitleOf(groups, otherID); got != "Half siblings" {
			t.Errorf("got %q; want %q", got, "Half siblings")
		}
	})

	t.Run("both parents shared", func(t *testing.T) {
		subjectID, _ := newTestPerson(t, pool)
		otherID, _ := newTestPerson(t, pool)
		momID, _ := newTestPerson(t, pool)
		dadID, _ := newTestPerson(t, pool)

		relParent(t, pool, momID, subjectID)
		relParent(t, pool, dadID, subjectID)
		relParent(t, pool, momID, otherID)
		relParent(t, pool, dadID, otherID)

		groups, err := m.Relations(ctx, subjectID)
		if err != nil {
			t.Fatal(err)
		}
		if got := relTitleOf(groups, otherID); got != "Siblings" {
			t.Errorf("got %q; want %q", got, "Siblings")
		}
	})

	t.Run("subject has one recorded parent", func(t *testing.T) {
		subjectID, _ := newTestPerson(t, pool)
		otherID, _ := newTestPerson(t, pool)
		sharedID, _ := newTestPerson(t, pool)
		otherOnlyID, _ := newTestPerson(t, pool)

		relParent(t, pool, sharedID, subjectID)
		relParent(t, pool, sharedID, otherID)
		relParent(t, pool, otherOnlyID, otherID)

		groups, err := m.Relations(ctx, subjectID)
		if err != nil {
			t.Fatal(err)
		}
		if got := relTitleOf(groups, otherID); got != "Siblings" {
			t.Errorf("got %q; want %q, one recorded parent is not enough to know they are half", got, "Siblings")
		}
	})

	t.Run("other has one recorded parent", func(t *testing.T) {
		subjectID, _ := newTestPerson(t, pool)
		otherID, _ := newTestPerson(t, pool)
		sharedID, _ := newTestPerson(t, pool)
		subjectOnlyID, _ := newTestPerson(t, pool)

		relParent(t, pool, sharedID, subjectID)
		relParent(t, pool, subjectOnlyID, subjectID)
		relParent(t, pool, sharedID, otherID)

		groups, err := m.Relations(ctx, subjectID)
		if err != nil {
			t.Fatal(err)
		}
		if got := relTitleOf(groups, otherID); got != "Siblings" {
			t.Errorf("got %q; want %q, one recorded parent is not enough to know they are half", got, "Siblings")
		}
	})
}

func TestFactsExcludeDerivedSiblings(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)
	momID, momName := newTestPerson(t, pool)
	dadID, dadName := newTestPerson(t, pool)
	sibID, sibName := newTestPerson(t, pool)
	spouseID, spouseName := newTestPerson(t, pool)
	childID, childName := newTestPerson(t, pool)

	relParent(t, pool, momID, subjectID)
	relParent(t, pool, dadID, subjectID)
	relParent(t, pool, momID, sibID)
	relParent(t, pool, dadID, sibID)
	relMarry(t, pool, subjectID, spouseID)
	relParent(t, pool, subjectID, childID)

	groups, err := m.Relations(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if got := relTitleOf(groups, sibID); got != "Siblings" {
		t.Fatalf("got %q; want %q before checking that Facts omits them", got, "Siblings")
	}

	facts, err := m.Facts(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 4 {
		t.Fatalf("got %d facts; want 4, siblings are derived and must not be stored edges", len(facts))
	}

	byName := map[string]string{}
	for _, f := range facts {
		byName[f.Person.Name] = f.Relation
	}
	if _, ok := byName[sibName]; ok {
		t.Errorf("got %q in Facts; want it absent, a sibling is never a recorded fact", sibName)
	}

	want := map[string]string{momName: "parent", dadName: "parent", spouseName: "spouse", childName: "child"}
	for name, relation := range want {
		if byName[name] != relation {
			t.Errorf("got %q for %q; want %q", byName[name], name, relation)
		}
	}
}

func TestFactsNoEdges(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)

	facts, err := m.Facts(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 0 {
		t.Errorf("got %d facts; want 0", len(facts))
	}
}

func TestNamesExcludeTheGivenID(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)
	aID, _ := newTestPerson(t, pool)
	bID, _ := newTestPerson(t, pool)
	cID, _ := newTestPerson(t, pool)

	subjectName := relRename(t, pool, subjectID, "Rel Names Subject")
	aName := relRename(t, pool, aID, "Rel Names A")
	bName := relRename(t, pool, bID, "Rel Names B")
	cName := relRename(t, pool, cID, "Rel Names C")

	names, err := m.Names(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}

	index := map[string]int{}
	for i, name := range names {
		index[name] = i
	}
	if _, ok := index[subjectName]; ok {
		t.Errorf("got %q in the list; want it excluded", subjectName)
	}

	for _, name := range []string{aName, bName, cName} {
		if _, ok := index[name]; !ok {
			t.Fatalf("got %q missing from the list; want it present", name)
		}
	}
	if !(index[aName] < index[bName] && index[bName] < index[cName]) {
		t.Errorf("got positions %d, %d, %d; want them ascending, names must come back ordered by name",
			index[aName], index[bName], index[cName])
	}
}

func TestAddRelativeCreatesAndLinks(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	tests := []struct {
		name     string
		relation string
	}{
		{"parent", "parent"},
		{"child", "child"},
		{"spouse", "spouse"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subjectID, _ := newTestPerson(t, pool)
			name := relName(t, pool, "Rel Added")

			p := Person{Summary: Summary{Name: name, Birthyear: 1955}, Birthplace: "Berlin", Bio: "a life"}
			err := m.AddRelative(ctx, p, subjectID, tt.relation, testUser)
			if err != nil {
				t.Fatal(err)
			}

			facts, err := m.Facts(ctx, subjectID)
			if err != nil {
				t.Fatal(err)
			}
			if len(facts) != 1 {
				t.Fatalf("got %d facts; want 1", len(facts))
			}
			if facts[0].Person.Name != name {
				t.Errorf("got %q; want %q", facts[0].Person.Name, name)
			}
			if facts[0].Relation != tt.relation {
				t.Errorf("got %q; want %q", facts[0].Relation, tt.relation)
			}
			if facts[0].Person.Birthyear != 1955 {
				t.Errorf("got %d; want %d", facts[0].Person.Birthyear, 1955)
			}
			if got := relHistoryCount(t, pool, "created", name); got != 1 {
				t.Errorf("got %d history rows; want 1", got)
			}
		})
	}
}

func TestAddRelativeSiblingCopiesParents(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)
	momID, _ := newTestPerson(t, pool)
	dadID, _ := newTestPerson(t, pool)
	relParent(t, pool, momID, subjectID)
	relParent(t, pool, dadID, subjectID)

	name := relName(t, pool, "Rel Added Sibling")

	err := m.AddRelative(ctx, Person{Summary: Summary{Name: name}}, subjectID, "sibling", testUser)
	if err != nil {
		t.Fatal(err)
	}

	newID, ok := relPersonID(t, pool, name)
	if !ok {
		t.Fatalf("got no person named %q; want one", name)
	}

	got := relParentIDs(t, pool, newID)
	want := relParentIDs(t, pool, subjectID)
	if len(got) != 2 || fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got parents %v; want %v copied from the existing person", got, want)
	}

	groups, err := m.Relations(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if title := relTitleOf(groups, newID); title != "Siblings" {
		t.Errorf("got %q; want %q", title, "Siblings")
	}

	facts, err := m.Facts(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range facts {
		if f.Person.ID == newID {
			t.Errorf("got %q in Facts as %q; want it absent, a sibling is only ever derived", name, f.Relation)
		}
	}
}

func TestAddRelativeSiblingCreatesUnknownParent(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, subjectName := newTestPerson(t, pool)
	name := relName(t, pool, "Rel Added Orphan Sibling")
	placeholder := "Unknown parent of " + subjectName
	relCleanupName(t, pool, placeholder)

	err := m.AddRelative(ctx, Person{Summary: Summary{Name: name}}, subjectID, "sibling", testUser)
	if err != nil {
		t.Fatal(err)
	}

	placeholderID, ok := relPersonID(t, pool, placeholder)
	if !ok {
		t.Fatalf("got no person named %q; want the placeholder parent", placeholder)
	}

	newID, ok := relPersonID(t, pool, name)
	if !ok {
		t.Fatalf("got no person named %q; want one", name)
	}

	children := relChildIDs(t, pool, placeholderID)
	want := []int{subjectID, newID}
	if subjectID > newID {
		want = []int{newID, subjectID}
	}
	if fmt.Sprint(children) != fmt.Sprint(want) {
		t.Fatalf("got children %v; want %v attached to the placeholder", children, want)
	}

	groups, err := m.Relations(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if title := relTitleOf(groups, newID); title != "Siblings" {
		t.Errorf("got %q; want %q, sharing the sole placeholder parent makes them plain siblings", title, "Siblings")
	}
}

func TestAddRelativeWritesLocation(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)
	name := relName(t, pool, "Rel Added Located")
	lat, lng := 52.52, 13.405

	p := Person{Summary: Summary{Name: name}, Location: "Berlin", Lat: &lat, Lng: &lng}
	err := m.AddRelative(ctx, p, subjectID, "child", testUser)
	if err != nil {
		t.Fatal(err)
	}

	var gotName string
	var gotLat, gotLng float64

	err = pool.QueryRow(ctx,
		`SELECT l.name, l.lat, l.lng FROM api_location l
		 JOIN api_person p ON p.id = l.person_id WHERE p.name = $1`, name).Scan(&gotName, &gotLat, &gotLng)
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "Berlin" || gotLat != lat || gotLng != lng {
		t.Errorf("got %q %v %v; want %q %v %v", gotName, gotLat, gotLng, "Berlin", lat, lng)
	}
}

func TestAddRelativeDuplicateName(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, subjectName := newTestPerson(t, pool)

	err := m.AddRelative(ctx, Person{Summary: Summary{Name: subjectName}}, subjectID, "child", testUser)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("got %v; want %v", err, ErrDuplicateName)
	}
}

func TestAddRelativeUnknownRelation(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, _ := newTestPerson(t, pool)
	name := relName(t, pool, "Rel Added Unknown")

	p := Person{Summary: Summary{Name: name}, Location: "Berlin"}
	err := m.AddRelative(ctx, p, subjectID, "cousin", testUser)
	if err == nil {
		t.Fatal("got nil; want an error for an unknown relation")
	}
	if _, ok := relPersonID(t, pool, name); ok {
		t.Errorf("got a person named %q; want none, the transaction must roll back", name)
	}
	if got := relHistoryCount(t, pool, "created", name); got != 0 {
		t.Errorf("got %d history rows; want 0", got)
	}
}

func TestAddRelativeRollsBackOnFailure(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	var missingID int

	err := pool.QueryRow(ctx, `SELECT coalesce(max(id), 0) + 1000 FROM api_person`).Scan(&missingID)
	if err != nil {
		t.Fatal(err)
	}

	name := relName(t, pool, "Rel Added Doomed")

	err = m.AddRelative(ctx, Person{Summary: Summary{Name: name}}, missingID, "parent", testUser)
	if err == nil {
		t.Fatal("got nil; want an error for a relative that does not exist")
	}
	if _, ok := relPersonID(t, pool, name); ok {
		t.Errorf("got a person named %q; want none", name)
	}
	if got := relHistoryCount(t, pool, "created", name); got != 0 {
		t.Errorf("got %d history rows; want 0", got)
	}

	var edges int

	err = pool.QueryRow(ctx,
		`SELECT count(*) FROM api_parentchild WHERE parent_id = $1 OR child_id = $1`, missingID).Scan(&edges)
	if err != nil {
		t.Fatal(err)
	}
	if edges != 0 {
		t.Errorf("got %d edges; want 0", edges)
	}
}

func TestLinkByName(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	tests := []struct {
		name     string
		relation string
	}{
		{"parent", "parent"},
		{"child", "child"},
		{"spouse", "spouse"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subjectID, subjectName := newTestPerson(t, pool)
			_, otherName := newTestPerson(t, pool)

			err := m.Link(ctx, subjectID, otherName, tt.relation, testUser)
			if err != nil {
				t.Fatal(err)
			}

			facts, err := m.Facts(ctx, subjectID)
			if err != nil {
				t.Fatal(err)
			}
			if len(facts) != 1 {
				t.Fatalf("got %d facts; want 1", len(facts))
			}
			if facts[0].Relation != tt.relation || facts[0].Person.Name != otherName {
				t.Errorf("got %q %q; want %q %q", facts[0].Relation, facts[0].Person.Name, tt.relation, otherName)
			}
			if got := relHistoryCount(t, pool, "added "+tt.relation, subjectName); got != 1 {
				t.Errorf("got %d history rows for %q; want 1, the subject is the recipient", got, subjectName)
			}
		})
	}
}

func TestUnlinkByName(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	tests := []struct {
		name     string
		relation string
	}{
		{"parent", "parent"},
		{"child", "child"},
		{"spouse", "spouse"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subjectID, subjectName := newTestPerson(t, pool)
			otherID, otherName := newTestPerson(t, pool)

			switch tt.relation {
			case "parent":
				relParent(t, pool, otherID, subjectID)
			case "child":
				relParent(t, pool, subjectID, otherID)
			case "spouse":
				relMarry(t, pool, subjectID, otherID)
			}

			err := m.Unlink(ctx, subjectID, otherName, tt.relation, testUser)
			if err != nil {
				t.Fatal(err)
			}

			facts, err := m.Facts(ctx, subjectID)
			if err != nil {
				t.Fatal(err)
			}
			if len(facts) != 0 {
				t.Fatalf("got %d facts; want 0", len(facts))
			}
			if got := relHistoryCount(t, pool, "removed "+tt.relation, subjectName); got != 1 {
				t.Errorf("got %d history rows for %q; want 1, the subject is the recipient", got, subjectName)
			}
		})
	}
}

func TestLinkErrors(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, subjectName := newTestPerson(t, pool)
	parentID, parentName := newTestPerson(t, pool)
	childID, childName := newTestPerson(t, pool)
	spouseID, spouseName := newTestPerson(t, pool)

	relParent(t, pool, parentID, subjectID)
	relParent(t, pool, subjectID, childID)
	relMarry(t, pool, subjectID, spouseID)

	tests := []struct {
		name     string
		target   string
		relation string
		want     error
	}{
		{"unknown name", relUnique("Rel Nobody"), "parent", ErrNoRecord},
		{"self link", subjectName, "parent", ErrSelfLink},
		{"parent already linked", parentName, "parent", ErrAlreadyLinked},
		{"child already linked", childName, "child", ErrAlreadyLinked},
		{"spouse already linked", spouseName, "spouse", ErrAlreadyLinked},
		{"unknown relation", parentName, "cousin", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Link(ctx, subjectID, tt.target, tt.relation, testUser)

			if tt.want == nil {
				if err == nil {
					t.Fatalf("got nil; want an error for relation %q", tt.relation)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v; want %v", err, tt.want)
			}
		})
	}

	if got := relHistoryCount(t, pool, "added parent", subjectName); got != 0 {
		t.Errorf("got %d history rows; want 0, a failed link must write nothing", got)
	}
}

func TestUnlinkErrors(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	subjectID, subjectName := newTestPerson(t, pool)
	_, otherName := newTestPerson(t, pool)

	tests := []struct {
		name     string
		target   string
		relation string
		want     error
	}{
		{"unknown name", relUnique("Rel Nobody"), "parent", ErrNoRecord},
		{"self unlink", subjectName, "parent", ErrSelfLink},
		{"parent edge missing", otherName, "parent", ErrNoRecord},
		{"child edge missing", otherName, "child", ErrNoRecord},
		{"spouse edge missing", otherName, "spouse", ErrNoRecord},
		{"unknown relation", otherName, "cousin", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Unlink(ctx, subjectID, tt.target, tt.relation, testUser)

			if tt.want == nil {
				if err == nil {
					t.Fatalf("got nil; want an error for relation %q", tt.relation)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v; want %v", err, tt.want)
			}
		})
	}

	if got := relHistoryCount(t, pool, "removed parent", subjectName); got != 0 {
		t.Errorf("got %d history rows; want 0, a failed unlink must write nothing", got)
	}
}

func TestLinkSpouseNormalizesIDOrder(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	m := relModel(pool)

	offset := int(time.Now().UnixNano() % 90000000)
	lowID, highID := 900000000+offset, 1000000000+offset
	if fmt.Sprint(lowID) < fmt.Sprint(highID) {
		t.Fatalf("got %d < %d as text; want the text order reversed so a missing ::bigint cast shows up", lowID, highID)
	}

	lowName := relPersonWithID(t, pool, lowID)
	highName := relPersonWithID(t, pool, highID)

	tests := []struct {
		name    string
		subject int
		target  string
	}{
		{"subject id below target", lowID, highName},
		{"subject id above target", highID, lowName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Link(ctx, tt.subject, tt.target, "spouse", testUser)
			if err != nil {
				t.Fatal(err)
			}

			a, b, ok := relMarriageRow(t, pool, lowID, highID)
			if !ok {
				t.Fatal("got no marriage row; want one")
			}
			if a != lowID || b != highID {
				t.Errorf("got (%d, %d); want (%d, %d), LEAST/GREATEST must normalize on the numeric ids",
					a, b, lowID, highID)
			}

			err = m.Unlink(ctx, tt.subject, tt.target, "spouse", testUser)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, ok := relMarriageRow(t, pool, lowID, highID); ok {
				t.Error("got a marriage row; want none after unlinking")
			}
		})
	}
}

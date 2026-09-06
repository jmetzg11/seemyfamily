package models

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestActionKind(t *testing.T) {
	tests := []struct {
		action Action
		want   Kind
	}{
		{ActionCreated, KindCreate},
		{ActionPhoto, KindCreate},
		{ActionAdded("parent"), KindCreate},
		{ActionAdded("child"), KindCreate},
		{ActionAdded("spouse"), KindCreate},
		{ActionAdded("sibling"), KindCreate},
		{ActionUpdated, KindEdit},
		{ActionDeleted, KindDelete},
		{ActionRemoved("parent"), KindDelete},
		{ActionRemoved("child"), KindDelete},
		{ActionRemoved("spouse"), KindDelete},
		{ActionRemoved("sibling"), KindDelete},
		{"", ""},
		{"wandered off", ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.Kind(); got != tt.want {
				t.Errorf("Action(%q).Kind() = %q; want %q", tt.action, got, tt.want)
			}
		})
	}
}

// TestActionsWrittenByMutations pins the strings the models actually store. The
// tally splits on the first word, so an action that stops starting with a known
// verb would drop out of every kind and quietly stop notifying.
func TestActionsWrittenByMutations(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()

	id, name := newTestPerson(t, pool)
	peopleCleanHistory(t, pool, name)

	people := PersonModel{DB: pool}

	err := people.Update(ctx, Person{Summary: Summary{ID: id, Name: name}}, testUser)
	if err != nil {
		t.Fatal(err)
	}

	var action Action

	err = pool.QueryRow(ctx,
		`SELECT action FROM api_history WHERE recipient = $1 ORDER BY id DESC LIMIT 1`, name).Scan(&action)
	if err != nil {
		t.Fatal(err)
	}

	if action != ActionUpdated {
		t.Errorf("Update stored action %q; want %q", action, ActionUpdated)
	}
	if action.Kind() != KindEdit {
		t.Errorf("action %q is kind %q; want %q", action, action.Kind(), KindEdit)
	}
}

func tallyInsert(t *testing.T, pool *pgxpool.Pool, username string, action Action, recipient string, createdAt time.Time) {
	t.Helper()

	_, err := pool.Exec(context.Background(),
		`INSERT INTO api_history (created_at, username, action, recipient) VALUES ($1, $2, $3, $4)`,
		createdAt, username, action, recipient)
	if err != nil {
		t.Fatal(err)
	}
}

// TestRecentCount also pins the window to a rolling 24 hours. A calendar day
// would put the same edit inside or outside the count depending on where the
// person making it lives.
func TestRecentCount(t *testing.T) {
	pool := newTestPool(t)
	model := InfoModel{DB: pool}

	_, name := newTestPerson(t, pool)
	peopleCleanHistory(t, pool, name)

	now := time.Now()

	tallyInsert(t, pool, testUser, ActionUpdated, name, now.Add(-23*time.Hour))
	tallyInsert(t, pool, testUser, ActionUpdated, name, now.Add(-2*time.Hour))
	tallyInsert(t, pool, testUser, ActionCreated, name, now.Add(-time.Hour))
	tallyInsert(t, pool, testUser, ActionPhoto, name, now.Add(-30*time.Minute))
	tallyInsert(t, pool, testUser, ActionRemoved("spouse"), name, now.Add(-10*time.Minute))

	tallyInsert(t, pool, testUser, ActionUpdated, name, now.Add(-25*time.Hour))
	tallyInsert(t, pool, "someone-else", ActionUpdated, name, now.Add(-5*time.Minute))

	tests := []struct {
		name string
		kind Kind
		want int
	}{
		{"edits", KindEdit, 2},
		{"creates", KindCreate, 2},
		{"deletes", KindDelete, 1},
		{"unknown kind", Kind("nonsense"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := model.RecentCount(context.Background(), testUser, tt.kind)
			if err != nil {
				t.Fatal(err)
			}

			if count != tt.want {
				t.Errorf("got %d; want %d; rows older than 24 hours and other users must not count", count, tt.want)
			}
		})
	}
}

func TestRecentCountNoRows(t *testing.T) {
	pool := newTestPool(t)
	model := InfoModel{DB: pool}

	count, err := model.RecentCount(context.Background(), "nobody-has-this-username", KindEdit)
	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Errorf("got %d; want 0", count)
	}
}

// TestRecentByUser covers the digest the notification email carries: one user,
// a rolling 24 hours rather than the calendar day the count uses, newest first.
func TestRecentByUser(t *testing.T) {
	pool := newTestPool(t)
	model := InfoModel{DB: pool}

	_, name := newTestPerson(t, pool)
	peopleCleanHistory(t, pool, name)

	now := time.Now()

	tallyInsert(t, pool, testUser, ActionUpdated, name, now.Add(-2*time.Hour))
	tallyInsert(t, pool, testUser, ActionPhoto, name, now.Add(-time.Hour))
	tallyInsert(t, pool, testUser, ActionDeleted, name, now.Add(-20*time.Hour))
	tallyInsert(t, pool, testUser, ActionCreated, name, now.Add(-26*time.Hour))
	tallyInsert(t, pool, "someone-else", ActionUpdated, name, now.Add(-time.Minute))

	edits, err := model.RecentByUser(context.Background(), testUser, 50)
	if err != nil {
		t.Fatal(err)
	}

	want := []Action{ActionPhoto, ActionUpdated, ActionDeleted}

	if len(edits) != len(want) {
		t.Fatalf("got %d edits; want %d; anything older than 24 hours or by another user must be left out", len(edits), len(want))
	}

	for i, w := range want {
		if edits[i].Action != w {
			t.Errorf("edit %d: got %q; want %q; rows must come back newest first", i, edits[i].Action, w)
		}
		if edits[i].Recipient != name {
			t.Errorf("edit %d recipient: got %q; want %q", i, edits[i].Recipient, name)
		}
	}
}

func TestRecentByUserLimit(t *testing.T) {
	pool := newTestPool(t)
	model := InfoModel{DB: pool}

	_, name := newTestPerson(t, pool)
	peopleCleanHistory(t, pool, name)

	now := time.Now()

	for i := range 5 {
		tallyInsert(t, pool, testUser, ActionUpdated, name, now.Add(-time.Duration(i)*time.Minute))
	}

	edits, err := model.RecentByUser(context.Background(), testUser, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(edits) != 3 {
		t.Errorf("got %d edits; want 3", len(edits))
	}
}

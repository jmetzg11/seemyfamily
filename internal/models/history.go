package models

import (
	"context"
	"slices"
	"strings"
)

const (
	verbCreated = "created"
	verbAdded   = "added"
	verbUpdated = "updated"
	verbDeleted = "deleted"
	verbRemoved = "removed"
)

type Action string

const (
	ActionCreated Action = verbCreated
	ActionUpdated Action = verbUpdated + " details"
	ActionDeleted Action = verbDeleted + " profile"
	ActionPhoto   Action = verbAdded + " photo"
)

func ActionAdded(relation string) Action {
	return Action(verbAdded + " " + relation)
}

func ActionRemoved(relation string) Action {
	return Action(verbRemoved + " " + relation)
}

type Kind string

const (
	KindCreate Kind = "create"
	KindEdit   Kind = "edit"
	KindDelete Kind = "delete"
)

var kindVerbs = map[Kind][]string{
	KindCreate: {verbCreated, verbAdded},
	KindEdit:   {verbUpdated},
	KindDelete: {verbDeleted, verbRemoved},
}

func (a Action) Kind() Kind {
	verb, _, _ := strings.Cut(string(a), " ")

	for kind, verbs := range kindVerbs {
		if slices.Contains(verbs, verb) {
			return kind
		}
	}

	return ""
}

const historyQuery = `
INSERT INTO api_history (created_at, username, action, recipient)
VALUES (now(), $1, $2, $3)`

const recentWindow = `created_at >= now() - interval '24 hours'`

const recentCountQuery = `
SELECT count(*)
FROM api_history
WHERE username = $1
  AND split_part(action, ' ', 1) = ANY($2)
  AND ` + recentWindow

func (m *InfoModel) RecentCount(ctx context.Context, username string, kind Kind) (int, error) {
	verbs, ok := kindVerbs[kind]
	if !ok {
		return 0, nil
	}

	var count int

	err := m.DB.QueryRow(ctx, recentCountQuery, username, verbs).Scan(&count)

	return count, err
}

# Family tree

Server-rendered SVG family tree on the person page. Branch `tree`.

## How to work on this

**Build it in small increments.** One reviewable step at a time, each one
compiling and visible in the browser before the next starts. Do not write the
whole feature and hand it over. Explain what is wrong with code that does not
work rather than silently rewriting it — the point is to learn the Go, not to
receive it.

## Constraints

- Zero new dependencies. No d3, no JS graph library, no client-side layout.
- Geometry computed in Go, handed to `html/template` as a slice of structs with
  x/y. Nodes are `<a href>` links so it works with JavaScript off.
- `style-src` has no `'unsafe-inline'` (`middleware.go:17`). Every SVG attribute
  must be a presentation attribute or a class in a stylesheet, never `style="…"`.
- Public read, login for writes. The tree reads what `/person` already exposes,
  so nothing new to guard.

## Data

`api_parentchild (parent_id, child_id)` and `api_marriage (person_a_id,
person_b_id, normalized a < b)` are the only facts. Siblings and half-siblings
are derived by query, never stored. No schema change, no migration.

## Where it is now

Works end to end, rendering exactly one name:

| | |
|---|---|
| `internal/models/tree.go` | `TreeModel`, `Node{Name}`, `GetTree(ctx, id) (Node, error)` |
| `cmd/web/main.go:22,93` | `tree *models.TreeModel` field, wired to the pool |
| `cmd/web/handlers_people.go:95` | `app.person` calls `GetTree`, sets `data.Tree` |
| `cmd/web/templates.go:34` | `Tree models.Node` |
| `ui/html/pages/person.html:49` | renders `{{.Tree.Name}}` |

The tree lives on `/person/{id}`, below the existing relation cards. The
duplication is deliberate for now — it lets the tree be checked against a
known-good list. Those card sections get deleted once the tree is good, which
also retires `Relations`, `RelationGroup` and `relationsQuery` in
`internal/models/relations.go`. Note `deleteForm` is a second caller of
`Relations`, so that is not a free deletion.

## Next step

One generation out: parents, spouses, children. Siblings are *not* one hop —
they are up to a shared parent and back down, so they appear at depth 2 along
with half-siblings.

One query, not several. `factsQuery` (`relations.go:115`) is already a
`UNION ALL` of the parent/spouse/child arms against a single `$1`; the sibling
arm is the `sibs` CTE (`relations.go:34`) with the half-sibling counting
stripped. Several small queries means a round trip per hop and merging
duplicates in Go afterwards.

`GetTree` returns `(Node, error)` today and becomes `([]Node, error)` or a
struct of slices. The `ErrNoRows` check goes away — zero rows stops being an
error — and `QueryRow` becomes `Query` with a scan loop like `Facts`.

## Undecided: the return shape

Not settled. Both are defensible, pick one and commit.

**Buckets.** Simple, renders immediately, no duplication, does everything one
generation out needs. Breaks at grandparents, where it needs `Parents[i].Parents`
and the same person starts appearing in two places.

```go
type Tree struct {
    Focus    Node
    Parents  []Node
    Siblings []Node
    Spouses  []Node
    Children []Node
}
```

**Graph.** Flat node list plus edge lists. One person is one row with one x/y,
which is what an SVG needs. Handles remarriage, cousin marriage and a person
reachable by two paths without special cases.

```go
type Node struct{ ID, Gen int; Name string }   // Gen: -1 parent, 0 focus, +1 child
type Edge struct{ A, B int }
type Graph struct {
    Nodes   []Node
    Parents []Edge   // A parent of B
    Couples []Edge   // a marriage
}
```

Spouse, sibling and half-sibling are not fields on a node — they are read off
the edges. Spouse: a `Couples` row with the focus on one side. Full sibling:
shares both `Parents` edges. Half sibling: shares one.

Buckets if the tree stays one generation up and down. Graph if it goes to two or
more. Going buckets-now-graph-later is a real option; the rewrite is small.

## Decisions already made

- **Draw an edge from each child to each recorded parent**, never from a child to
  a couple. Half-siblings often come from a pairing with no `api_marriage` row,
  so there is no bar for them to hang from and they would vanish or attach to the
  wrong parent. Per-parent edges handle full and half with one rule. Rendering
  a shared fork for two children of the same two parents is a later nicety, not
  the data model.
- **No half-sibling labelling.** A card list prints a word and needs the counting
  CTE. A tree prints a shape: a half-sibling drops from one parent instead of the
  fork. The geometry is the fact.
- **Unknown parents are real rows, not placeholders.** `insertUnknownParentQuery`
  (`relations.go:212`) inserts an `api_person` named `Unknown parent of X`, so the
  tree draws them like anyone else with no extra code. Optional polish: lift the
  prefix into a constant and dim those nodes. Never invent a phantom node for a
  genuinely missing parent — one recorded parent means one line and a gap.
- **Generation is the row**, `y = Gen * rowHeight`. Assigning x within a row is
  the hard part once someone appears in two marriages.
- **Cycles must be handled.** Nothing in the schema stops a person being their own
  ancestor. Any traversal needs a visited set or it hangs the request. Also worth
  a hard node cap.

## Gotchas found the hard way

- `GET /person/temp/{id}` panics the mux at startup — it conflicts with
  `GET /person/{id}/photos`, since both match `/person/temp/photos` and neither
  is more specific. Sub-pages go at the end: `GET /person/{id}/tree`.
- Model methods called from `cmd/web` must be exported. Lowercase `tree` is
  invisible outside package `models`.
- A struct field added to `application` but left out of the literal at
  `main.go:89` compiles fine and is `nil` at request time.

## Testing

Model tests hit a real local Postgres — no mocks, no interfaces anywhere in the
codebase. `TestMain` in `internal/models/guard_test.go` calls
`testutil.MustBeLocal`, which aborts the binary if `DATABASE_URL` points anywhere
but localhost or a compose service.

```
set -a; . ./.env; set +a; go test ./internal/models/
```

Reuse the helpers in `relations_test.go`: `newTestPool`, `newTestPerson`,
`relParent`, `relMarry`, `relRename`, `relBirthyear`. They register their own
cleanup. Assertion style is `got %v; want %v` with the reason on the end of the
message.

Once geometry exists, the layout function should be a pure function tested with
no database: no two nodes overlap, output is deterministic (sort inputs — row
order out of Postgres is not a contract), a cycle terminates, the node cap holds.

## Loose end

`internal/models/people.go` has an unrelated move of `countQuery` below `List`.
Either keep it as a tidy-up or drop it from the branch — it is not part of this
feature.

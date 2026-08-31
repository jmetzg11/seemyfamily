# Dev fixtures

Synthetic data for local development, split one file per model and numbered in
dependency order. `make run` loads them with:

    cd admin && uv run python manage.py loaddata api/fixtures/[0-9]*.json

The shell expands that glob in sorted order and `loaddata` installs the files in
the order given, so `0002_person.json` is in place before anything that points
at a person. Adding a new fixture means adding a file with the next number — the
Makefile picks it up with no change.

| File | Model | Rows | Holds |
| --- | --- | --- | --- |
| `0001_user.json` | `auth.user` | 2 | The `admin` superuser and an ordinary `guest` user. Passwords are hashed here; the root README has the plaintext. |
| `0002_person.json` | `api.person` | 12 | The people themselves. Everything else hangs off these primary keys. |
| `0003_parentchild.json` | `api.parentchild` | 12 | One row per parent→child edge, so a child with two known parents has two rows. |
| `0004_marriage.json` | `api.marriage` | 5 | Spouse pairs, unordered in meaning but stored canonically. |
| `0005_location.json` | `api.location` | 11 | Where a person lives now, with coordinates for the map. |
| `0006_photo.json` | `api.photo` | 13 | Photo metadata. The images themselves live in `media/photos/`. |
| `0007_history.json` | `api.history` | 6 | The audit feed. Stores names as plain strings, not foreign keys, so entries survive a deletion. |

## The family

Three generations, plus one person deliberately left dangling:

    Mira Vance (1938) ── married ── Otto Vance (1936)
      └── Boris Vance (1962)
            ├── married Ada Kovac (1964)
            │     ├── Cleo Vance (1988) ── married ── Marko Ilic (1991)
            │     │     └── Nils Vance (2018)
            │     └── Dmitri Vance (1990)
            └── married Elena Roth (1968)
                  └── Fyodor Vance (1996)

    Elena Roth (1968) ── married ── Pavel Roth (1965)
      └── Greta Roth (1992)

The shape is deliberate — it exercises the cases the tree and map code has to
survive:

- **Boris is married twice** (to Ada, then Elena), so his children come from two
  different pairings.
- **Elena also appears in a second marriage** to Pavel, so she is reachable from
  two branches.
- **Marko has no `parentchild` rows at all** — a person who enters the tree by
  marriage with no recorded ancestry.
- **Pavel has no `location` row**, so the map has to cope with a person it cannot
  place.
- **Nils is a child** (born 2018); everyone else is an adult.

## Rules a new fixture has to respect

The database constraints in `api/models.py` are checked at load time:

- `api.marriage` requires `person_a` < `person_b`. This is the
  `marriage_canonical_order` check constraint, which keeps a pair from being
  stored twice in opposite orders. A row with the two swapped will fail to load.
- `api.parentchild` is unique per `(parent, child)`, and a person cannot be their
  own parent.
- `api.location` is a **one-to-one** with person — at most one row per person.

One rule is *not* enforced at load time and is the fixture's responsibility:
`Photo.save()` clears the other `profile_pic` flags for a person, but `loaddata`
writes rows directly and never calls it. If two photo rows for the same person
both say `"profile_pic": true`, both will load and the person will have two.

## Photos

`file_path` is an object key in the storage bucket, not a path on disk. Seed
images live in `media/photos/seed-*.png` and are mirrored into MinIO by the
`createbucket` service in `docker-compose.yml`; runtime uploads land in the same
bucket under `<person_id>/<timestamp>.jpeg`.

Every person in the fixture has a profile photo. `default.jpeg` is the fallback
in `COALESCE(file_path, 'default.jpeg')` for a person added through the app who
has no photo yet, which is why no fixture row points at it.

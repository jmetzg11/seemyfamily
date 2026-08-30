# seemyfamily

[photo]
[photo]

## Development

```
make run     # postgres + minio, migrations, dev fixtures, then both servers
make down    # stop everything
```

| | |
|---|---|
| app | http://localhost:4000 |
| admin | http://localhost:8000/admin |
| minio | http://localhost:9001 — `seemyfamily` / `donkey6donkey6` |

The fixtures ship two accounts:

| username | password | |
|---|---|---|
| `admin` | `admin` | superuser, can reach the Django admin |
| `guest` | `guest` | ordinary app user |

Needs docker, [uv](https://docs.astral.sh/uv/), and [air](https://github.com/air-verse/air).

`make run` is dev-only and loads synthetic fixtures data.
To wipe the database and object store and start over: `make down`.

## 
- Django admin is just for content management. Can connect to prod locally via `make admin-prod`. Django admin also handles DB migrations 
- Only Parent <-> Child and Marriages are recorded. Everything else is inferred through DB queries (e.g. adding a sibling means adding the shared parent)


## Production

Production is a single Go binary with no Python in it, so migrations to the prod
database are applied by the Django admin running locally:

```
make admin-prod
```

It runs `migrate`, then the Django admin on 8000 and the app on 4000, both pointed at
prod, so it doubles as a way to curate and view real content. It is a separate target
from `run` so that `loaddata` can never execute against real data.

Connection details come from `.env.prod`, which holds the same keys as `.env` pointed
at Supabase instead of docker. It is gitignored, and `admin-prod` refuses to start
without it rather than falling back to the local database.


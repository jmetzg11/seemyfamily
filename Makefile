.PHONY: services services-down migrate fixtures superuser admin run down reset

DB ?= dev

services:
	docker compose up -d
	@until docker compose exec -T db pg_isready -U postgres -d seemyfamily; do sleep 1; done

services-down:
	docker compose down

migrate:
	cd admin && DB=$(DB) uv run python manage.py migrate

fixtures:
	cd admin && DB=$(DB) uv run python manage.py loaddata dev.json

superuser:
	cd admin && DB=$(DB) DJANGO_SUPERUSER_PASSWORD=admin uv run python manage.py createsuperuser --noinput --username admin --email admin@example.com || true

admin:
	cd admin && DB=$(DB) uv run python manage.py runserver 8000

run: services migrate fixtures superuser
	cd admin && uv run python manage.py runserver 8000 &
	air

down:
	@pkill -f 'tmp/web' || true
	@pkill -f 'manage.py runserver' || true
	@fuser -k 4000/tcp 2>/dev/null || true

reset: down
	docker compose down -v

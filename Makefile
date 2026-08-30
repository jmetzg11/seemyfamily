.PHONY: run down admin-prod hooks

hooks:
	@chmod +x .githooks/*
	@git config core.hooksPath .githooks
	@echo "pre-commit hook installed"

run:
	docker compose up -d
	@until docker compose exec -T db pg_isready -U postgres -d seemyfamily >/dev/null 2>&1; do sleep 1; done
	@cd admin && uv run python manage.py migrate
	@cd admin && uv run python manage.py loaddata api/fixtures/[0-9]*.json
	@trap 'pkill -f "[m]anage.py runserver" >/dev/null 2>&1 || true' EXIT; \
	set -a; . ./.env; set +a; \
	(cd admin && uv run python manage.py runserver 8000 &); \
	air

admin-prod:
	@test -f .env.prod || { echo "admin-prod: .env.prod is missing"; exit 1; }
	@trap 'pkill -f "[m]anage.py runserver" >/dev/null 2>&1 || true' EXIT; \
	set -a; . ./.env.prod; set +a; \
	(cd admin && uv run python manage.py migrate) || exit 1; \
	(cd admin && uv run python manage.py runserver 8000 &); \
	go run ./cmd/web

down:
	@pkill -f '[t]mp/web' 2>/dev/null || true
	@pkill -f '[m]anage.py runserver' 2>/dev/null || true
	@docker compose down -v 
	

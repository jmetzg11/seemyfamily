.PHONY: run down admin-prod shellplus hooks

hooks:
	@chmod +x .githooks/*
	@git config core.hooksPath .githooks
	@echo "pre-commit hook installed"

run:
	docker compose up -d
	@until docker compose exec -T db pg_isready -U postgres -d seemyfamily >/dev/null 2>&1; do sleep 1; done
	@trap 'pkill -f "[m]anage.py runserver" >/dev/null 2>&1 || true' EXIT; \
	set -a; . ./.env; set +a; \
	case "$$DATABASE_URL" in \
		*@localhost:*|*@127.0.0.1:*) ;; \
		*) echo "run: .env DATABASE_URL points at $${DATABASE_URL##*@}, not localhost; refusing to migrate and load fixtures"; exit 1;; \
	esac; \
	(cd admin && uv run python manage.py migrate) || exit 1; \
	(cd admin && uv run python manage.py loaddata api/fixtures/[0-9]*.json) || exit 1; \
	(cd admin && uv run python manage.py runserver 8000 &); \
	air

admin-prod:
	@test -f .env.prod || { echo "admin-prod: .env.prod is missing"; exit 1; }
	@trap 'pkill -f "[m]anage.py runserver" >/dev/null 2>&1 || true' EXIT; \
	set -a; . ./.env.prod; set +a; \
	(cd admin && uv run python manage.py migrate) || exit 1; \
	(cd admin && uv run python manage.py runserver 8000 &); \
	go build -o ./tmp/web ./cmd/web || exit 1; \
	./tmp/web

shellplus:
	@url=; \
	for pid in $$(pgrep -f '[m]anage.py runserver'); do \
		url=$$(tr '\0' '\n' </proc/$$pid/environ 2>/dev/null | sed -n 's/^DATABASE_URL=//p'); \
		test -n "$$url" && break; \
	done; \
	test -n "$$url" || { echo "shellplus: no admin server with DATABASE_URL running; start 'make run' or 'make admin-prod' first"; exit 1; }; \
	echo "shellplus: $${url##*@}"; \
	export DATABASE_URL=$$url; \
	cd admin && uv run python manage.py shell

down:
	@pkill -f '[t]mp/web' 2>/dev/null || true
	@pkill -f '[g]o run ./cmd/web' 2>/dev/null || true
	@pkill -f '[g]o-build.*/exe/web' 2>/dev/null || true
	@pkill -f '[m]anage.py runserver' 2>/dev/null || true
	@docker compose down -v

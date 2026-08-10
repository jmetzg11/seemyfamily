.PHONY: run down admin-prod

run:
	docker compose up -d
	@until docker compose exec -T db pg_isready -U postgres -d seemyfamily >/dev/null 2>&1; do sleep 1; done
	@cd admin && uv run python manage.py migrate
	@cd admin && uv run python manage.py loaddata dev.json
	@trap 'pkill -f "[m]anage.py runserver" >/dev/null 2>&1 || true' EXIT; \
	set -a; . ./.env; set +a; \
	(cd admin && uv run python manage.py runserver 8000 &); \
	air

admin-prod:
	@echo "TODO: prompt for the prod database and storage connection strings,"
	@echo "then run migrate and the Django admin against prod. Never loaddata."
	@exit 1

down:
	@pkill -f '[t]mp/web' 2>/dev/null || true
	@pkill -f '[m]anage.py runserver' 2>/dev/null || true
	@docker compose down -v 
	

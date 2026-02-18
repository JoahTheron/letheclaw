.PHONY: build start stop restart logs test clean

# Build all services
build:
	@echo "Building letheClaw services..."
	cd api && go mod tidy && go build -o letheclaw-api .
	@echo "Build complete."

# Start infrastructure only (for local development)
infra:
	@echo "Starting infrastructure (PostgreSQL, Qdrant, Redis, Embeddings)..."
	docker compose up -d postgres qdrant redis letheclaw-embeddings
	@echo "Waiting for services to be healthy..."
	sleep 5
	@echo "Infrastructure ready."

# Start all services
start:
	@echo "Starting letheClaw..."
	docker compose up -d
	@echo "letheClaw is running."

# Stop all services
stop:
	@echo "Stopping letheClaw..."
	docker compose down
	@echo "letheClaw stopped."

# Restart all services
restart: stop start

# View logs
logs:
	docker compose logs -f

# Test health (only API is exposed on 51234; internal services checked via exec)
test:
	@echo "Testing letheClaw API (only exposed port 51234)..."
	curl -s http://localhost:51234/health | json_pp || echo "API not responding"
	@echo "\nInternal services (via Docker):"
	@docker compose exec -T postgres pg_isready -U letheclaw 2>/dev/null && echo "  PostgreSQL OK" || echo "  PostgreSQL not responding"
	@docker compose exec -T redis redis-cli ping 2>/dev/null | grep -q PONG && echo "  Redis OK" || echo "  Redis not responding"

# Clean up (removes volumes!)
clean:
	@echo "WARNING: This will delete all data!"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		docker compose down -v; \
		echo "Cleanup complete."; \
	fi

# Run schema migrations
migrate:
	@echo "Running database migrations..."
	docker compose exec postgres psql -U letheclaw -d letheclaw -f /docker-entrypoint-initdb.d/001_init.sql
	@echo "Migrations complete."

# View database tables
db-tables:
	docker compose exec postgres psql -U letheclaw -d letheclaw -c "\dt"

# PostgreSQL shell
db-shell:
	docker compose exec postgres psql -U letheclaw -d letheclaw

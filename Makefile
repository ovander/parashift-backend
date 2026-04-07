.PHONY: run build test lint migrate-up migrate-down migrate-create docker-up docker-down

# Application
run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/parashift ./cmd/server

# Testing
test:
	go test -race ./...

test-verbose:
	go test -race -v ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

# Migrations
MIGRATE=migrate -database "${DATABASE_URL}" -path migrations

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

migrate-reset:
	$(MIGRATE) drop -f && $(MIGRATE) up

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

migrate-version:
	$(MIGRATE) version

# Docker
docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-build:
	docker compose build

docker-logs:
	docker compose logs -f app

# Database (local dev)
db-up:
	docker compose up -d db

db-shell:
	docker compose exec db psql -U parashift -d parashift

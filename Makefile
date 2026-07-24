COMPOSE := docker compose -f docker-compose.yml

.PHONY: down up build clean test smoke swagger migrate-up migrate-down migrate-status

down:
	${COMPOSE} down

up:
	${COMPOSE} up -d

build:
	${COMPOSE} up -d --build

clean:
	${COMPOSE} down -v --rmi all

test:
	go test -v ./...

smoke:
	sh scripts/smoke.sh

swagger:
	swag init -g cmd/main.go -o docs --parseDependency --parseInternal

migrate-up:
	sh migration/migrations.sh --up

migrate-down:
	sh migration/migrations.sh --down

migrate-status:
	sh migration/migrations.sh --status

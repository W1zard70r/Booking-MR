ifneq (,$(wildcard ./.env))
    include .env
    export
endif

DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: up down run psql

up:
	docker-compose up -d --build

down:
	docker-compose down

run:
	go run cmd/server/main.go

psql:
	docker exec -it booking_db psql -U $(DB_USER) -d $(DB_NAME)

migrate:
	docker run --rm -v $(CURDIR)/migrations:/migrations --network host migrate/migrate \
		-path=/migrations/ -database "$(DB_URL)" up

migrate-down:
	docker run --rm -v $(CURDIR)/migrations:/migrations --network host migrate/migrate \
		-path=/migrations/ -database "$(DB_URL)" down -all

migrate-down1:
	docker run --rm -v $(CURDIR)/migrations:/migrations --network host migrate/migrate \
		-path=/migrations/ -database "$(DB_URL)" down 1

seed:
	docker exec -i booking_db psql -U $(DB_USER) -d $(DB_NAME) < scripts/seed.sql

test:
	go test -v ./...

cover:
	go test -v -coverpkg=./internal/... -coverprofile=cover.out ./tests/...
	go tool cover -func=cover.out
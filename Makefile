# Подгружаем переменные из .env
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Собираем строку подключения
# Обрати внимание: для работы внутри docker (network host) используем переменные из .env
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: up down run psql

# Поднять базу данных в фоновом режиме
up:
	docker-compose up -d

# Остановить базу данных
down:
	docker-compose down

# Запустить наш бэкенд локально
run:
	go run cmd/server/main.go

# Заход в базу 
psql:
	docker exec -it booking_db psql -U $(DB_USER) -d $(DB_NAME)

# миграции
migrate:
	docker run --rm -v $(CURDIR)/migrations:/migrations --network host migrate/migrate \
		-path=/migrations/ -database "$(DB_URL)" up

seed:
	docker exec -i booking_db psql -U $(DB_USER) -d $(DB_NAME) < scripts/seed.sql

# Тесты
test:
	go test -v ./...

# Покрытие тестами
cover:
	go test -v -coverpkg=./internal/... -coverprofile=cover.out ./tests/...
	go tool cover -func=cover.out
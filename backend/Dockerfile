FROM golang:1.25-alpine AS builder 
WORKDIR /app

# Копируем go.mod и go.sum из папки backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Копируем исходники из папки backend в рабочую директорию /app
COPY backend/ ./

# Копируем папки миграций и скриптов из корня
COPY migrations/ ./migrations
COPY scripts/ ./scripts

# Собираем приложение
RUN go build -o main cmd/server/main.go

# Финальный образ
FROM alpine:latest
WORKDIR /app

# Копируем бинарник и необходимые папки
COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/scripts ./scripts

EXPOSE 8080
CMD ["./main"]
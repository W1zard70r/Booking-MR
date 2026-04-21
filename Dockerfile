# Берем любую свежую версию, она сама скачает 1.25
FROM golang:1.23-alpine AS builder 

WORKDIR /app

# МАГИЯ ЗДЕСЬ: разрешаем Go скачать версию 1.25.0, которую просит твой go.mod
ENV GOTOOLCHAIN=auto

# Копируем файлы зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код
COPY . .

# Собираем приложение
RUN go build -o main cmd/server/main.go

# Этап запуска
FROM alpine:latest

WORKDIR /app

# Копируем только необходимые артефакты из этапа сборки
COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/scripts ./scripts

# Открываем порт
EXPOSE 8080

# Запуск
CMD ["./main"]
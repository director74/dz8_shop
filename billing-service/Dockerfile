FROM golang:1.23-alpine as builder

WORKDIR /app

# Копируем go.mod и go.sum из корневой директории
COPY /src/go.mod /src/go.sum ./

# Устанавливаем зависимости
RUN go mod download

# Копируем исходный код
COPY /src/pkg ./pkg
COPY /src/billing-service ./billing-service

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o /billing-service /app/billing-service/cmd/app

# Финальный образ
FROM alpine:3.18

WORKDIR /

# Копируем бинарный файл из builder
COPY --from=builder /billing-service .
# Копируем конфигурацию
COPY /src/billing-service/config /config

# Устанавливаем переменные окружения
ENV APP_ENV=production

# Указываем порт
EXPOSE 8081

# Запускаем приложение
CMD ["/billing-service"] 
FROM golang:1.23-alpine as builder

WORKDIR /app

# Копируем go.mod и go.sum из корневой директории
COPY /src/go.mod /src/go.sum ./

# Устанавливаем зависимости
RUN go mod download

# Копируем исходный код
COPY /src/pkg ./pkg
COPY /src/payment-service ./payment-service

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o /payment-service /app/payment-service/cmd/app

# Финальный образ
FROM alpine:3.18

WORKDIR /

# Копируем бинарный файл из builder
COPY --from=builder /payment-service .
# Копируем конфигурацию
COPY /src/payment-service/config /config

# Устанавливаем переменные окружения
ENV APP_ENV=production
ENV INTERNAL_API_KEY=internal-api-key-for-services

# Указываем порт
EXPOSE 8083

# Запускаем приложение
CMD ["/payment-service"] 
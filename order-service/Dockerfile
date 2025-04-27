FROM golang:1.23-alpine as builder

WORKDIR /app

# Копируем go.mod и go.sum из корневой директории
COPY /src/go.mod /src/go.sum ./

# Устанавливаем зависимости
RUN go mod download

# Копируем исходный код
COPY /src/pkg ./pkg
COPY /src/order-service ./order-service

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o /order-service /app/order-service/cmd/app

# Финальный образ
FROM alpine:3.18

WORKDIR /

# Копируем бинарный файл из builder
COPY --from=builder /order-service .
# Копируем конфигурацию
COPY /src/order-service/config /config

# Устанавливаем переменные окружения
ENV APP_ENV=production

# Указываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["/order-service"] 
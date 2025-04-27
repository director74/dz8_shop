FROM golang:1.23-alpine as builder

WORKDIR /app

# Копируем go.mod и go.sum из корневой директории
COPY /src/go.mod /src/go.sum ./

# Устанавливаем зависимости
RUN go mod download

# Копируем исходный код
COPY /src/pkg ./pkg
COPY /src/warehouse-service ./warehouse-service

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o /warehouse-service /app/warehouse-service/cmd/app

# Финальный образ
FROM alpine:3.18

WORKDIR /

# Копируем бинарный файл из builder
COPY --from=builder /warehouse-service .
# Копируем конфигурацию
COPY /src/warehouse-service/config /config

# Устанавливаем переменные окружения
ENV APP_ENV=production
ENV GIN_MODE=release

# Указываем порт
EXPOSE 8084

# Запускаем приложение
CMD ["/warehouse-service"]
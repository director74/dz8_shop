FROM golang:1.23-alpine as builder

WORKDIR /app

COPY ../go.mod go.sum ./
RUN go mod download

COPY .. .

ARG SERVICE_NAME
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/app /app/${SERVICE_NAME}/cmd/app/main.go

FROM alpine:3.21.3

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/app /app/

ENTRYPOINT ["/app/app"]
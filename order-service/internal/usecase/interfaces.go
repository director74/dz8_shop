package usecase

import (
	"context"
)

// BillingService интерфейс для работы с сервисом биллинга
type BillingService interface {
	CreateAccount(ctx context.Context, userID uint) error
	WithdrawMoney(ctx context.Context, userID uint, amount float64, email string, token string) (bool, error)
}

// RabbitMQClient интерфейс для работы с RabbitMQ
type RabbitMQClient interface {
	PublishMessage(exchange, routingKey string, message interface{}) error
	PublishMessageWithRetry(exchange, routingKey string, message interface{}, retries int) error
}

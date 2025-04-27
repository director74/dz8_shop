package rabbitmq

import (
	"fmt"
	"log"

	"github.com/director74/dz8_shop/payment-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
)

// PaymentConsumer обработчик сообщений для платежей
type PaymentConsumer struct {
	paymentUseCase *usecase.PaymentUseCase
	rabbitMQ       *rabbitmq.RabbitMQ
}

// NewPaymentConsumer создает новый обработчик сообщений для платежей
func NewPaymentConsumer(paymentUseCase *usecase.PaymentUseCase, rabbitMQ *rabbitmq.RabbitMQ) *PaymentConsumer {
	return &PaymentConsumer{
		paymentUseCase: paymentUseCase,
		rabbitMQ:       rabbitMQ,
	}
}

// Setup настраивает обработчик событий
func (c *PaymentConsumer) Setup() error {
	// Объявляем exchange для заказов
	err := c.rabbitMQ.DeclareExchange("order_events", "topic")
	if err != nil {
		return fmt.Errorf("ошибка при объявлении exchange для заказов: %w", err)
	}

	// Объявляем exchange для платежей
	err = c.rabbitMQ.DeclareExchange("payment_events", "topic")
	if err != nil {
		return fmt.Errorf("ошибка при объявлении exchange для платежей: %w", err)
	}

	// Объявляем очередь для обработки событий заказов
	err = c.rabbitMQ.DeclareQueue("order_payment_queue")
	if err != nil {
		return fmt.Errorf("ошибка при объявлении очереди для заказов: %w", err)
	}

	// Привязываем очередь заказов к exchange с ключами
	err = c.rabbitMQ.BindQueue("order_payment_queue", "order_events", "order.created")
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди к ключу order.created: %w", err)
	}

	err = c.rabbitMQ.BindQueue("order_payment_queue", "order_events", "order.cancelled")
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди к ключу order.cancelled: %w", err)
	}

	// Настраиваем обработчик сообщений из очереди
	err = c.rabbitMQ.ConsumeMessages("order_payment_queue", "payment-service", func(data []byte) error {
		return c.paymentUseCase.HandleOrderEvent(data)
	})

	if err != nil {
		return fmt.Errorf("ошибка при настройке обработчика сообщений: %w", err)
	}

	log.Println("Настроена обработка сообщений в платежном сервисе")
	return nil
}

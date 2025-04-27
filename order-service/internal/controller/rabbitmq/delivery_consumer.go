package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/director74/dz8_shop/order-service/internal/entity"
	"github.com/director74/dz8_shop/order-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
)

// DeliveryConsumer обработчик сообщений от сервиса доставки
type DeliveryConsumer struct {
	orderUseCase *usecase.OrderUseCase
	orderRepo    usecase.OrderRepository
	rabbitMQ     *rabbitmq.RabbitMQ
	logger       *log.Logger
}

// NewDeliveryConsumer создает новый обработчик
func NewDeliveryConsumer(
	orderUseCase *usecase.OrderUseCase,
	orderRepo usecase.OrderRepository,
	rabbitMQ *rabbitmq.RabbitMQ,
	logger *log.Logger,
) *DeliveryConsumer {
	if logger == nil {
		logger = log.New(log.Writer(), "[DeliveryConsumer] ", log.LstdFlags)
	}
	return &DeliveryConsumer{
		orderUseCase: orderUseCase,
		orderRepo:    orderRepo,
		rabbitMQ:     rabbitMQ,
		logger:       logger,
	}
}

// DeliveryCompletedMessage структура сообщения об успешной доставке (копируем из delivery-service)
type DeliveryCompletedMessage struct {
	OrderID     uint      `json:"order_id"`
	DeliveryID  uint      `json:"delivery_id"`
	Status      string    `json:"status"`
	CompletedAt time.Time `json:"completed_at"`
}

// HandleDeliveryCompleted обрабатывает сообщение о завершении доставки
func (c *DeliveryConsumer) HandleDeliveryCompleted(data []byte) error {
	var msg DeliveryCompletedMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Printf("[ERROR] OrderID=%d: Не удалось десериализовать сообщение delivery.completed: %v", msg.OrderID, err)
		return fmt.Errorf("ошибка десериализации delivery.completed: %w", err)
	}

	// Сравниваем статус из сообщения со строковым представлением ожидаемого статуса
	if msg.Status != "completed" { // Сравниваем со строкой
		c.logger.Printf("[WARN] OrderID=%d: Получено сообщение delivery.completed с неверным статусом '%s', ожидался 'completed', игнорируем.", msg.OrderID, msg.Status)
		return nil // Не ошибка, просто игнорируем
	}

	c.logger.Printf("[INFO] OrderID=%d: Получено событие delivery.completed.", msg.OrderID)

	// Здесь логика завершения саги или перехода к следующему шагу
	// Т.к. доставка - последний шаг перед завершением, попробуем завершить заказ

	// Обновляем статус заказа напрямую (сага могла уже быть очищена)
	err := c.orderRepo.UpdateOrderStatus(context.Background(), msg.OrderID, entity.OrderStatusCompleted)
	if err != nil {
		c.logger.Printf("[ERROR] OrderID=%d: Ошибка обновления статуса заказа на Completed после delivery.completed: %v", msg.OrderID, err)
		// Не возвращаем ошибку, чтобы сообщение не переобрабатывалось бесконечно,
		// но нужно мониторить такие логи.
		return nil
	}

	c.logger.Printf("[INFO] OrderID=%d: Статус заказа успешно обновлен на Completed.", msg.OrderID)

	// Попытка найти и завершить сагу (может не существовать, если уже очищена)
	// В текущей реализации SagaOrchestrator нет простого способа найти SagaID по OrderID
	// и напрямую завершить. Обновления статуса заказа может быть достаточно.
	// sagaOrchestrator := c.orderUseCase.GetSagaOrchestrator() // Гипотетический вызов
	// sagaOrchestrator.CompleteSagaByOrderID(msg.OrderID) // Гипотетический вызов

	return nil
}

// Setup настраивает консьюмера
func (c *DeliveryConsumer) Setup() error {
	exchangeName := "delivery_events" // Убедитесь, что имя exchange совпадает с тем, что в delivery-service
	queueName := "delivery_order_queue"
	routingKey := "delivery.completed"

	// Объявляем exchange
	err := c.rabbitMQ.DeclareExchange(exchangeName, "topic")
	if err != nil {
		c.logger.Printf("[ERROR] Ошибка при объявлении exchange %s: %v", exchangeName, err)
		return fmt.Errorf("ошибка при объявлении exchange %s: %w", exchangeName, err)
	}

	// Объявляем очередь
	err = c.rabbitMQ.DeclareQueue(queueName)
	if err != nil {
		c.logger.Printf("[ERROR] Ошибка при объявлении очереди %s: %v", queueName, err)
		return fmt.Errorf("ошибка при объявлении очереди %s: %w", queueName, err)
	}

	// Привязываем очередь к exchange
	err = c.rabbitMQ.BindQueue(queueName, exchangeName, routingKey)
	if err != nil {
		c.logger.Printf("[ERROR] Ошибка при привязке очереди %s к ключу %s: %v", queueName, routingKey, err)
		return fmt.Errorf("ошибка при привязке очереди %s к ключу %s: %w", queueName, routingKey, err)
	}

	// Настраиваем обработчик сообщений
	err = c.rabbitMQ.ConsumeMessages(queueName, "order-service-delivery-handler", func(data []byte) error {
		return c.HandleDeliveryCompleted(data)
	})
	if err != nil {
		c.logger.Printf("[ERROR] Ошибка при настройке обработчика сообщений для %s: %v", queueName, err)
		return fmt.Errorf("ошибка при настройке обработчика сообщений для %s: %w", queueName, err)
	}

	c.logger.Printf("[INFO] Настроена обработка сообщений из очереди %s", queueName)
	return nil
}

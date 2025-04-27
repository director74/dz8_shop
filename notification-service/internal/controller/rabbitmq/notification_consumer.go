package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/director74/dz8_shop/notification-service/internal/entity"
	"github.com/director74/dz8_shop/notification-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
	"github.com/director74/dz8_shop/pkg/sagahandler"
)

type NotificationConsumer struct {
	notificationUseCase *usecase.NotificationUseCase
	rabbitMQ            *rabbitmq.RabbitMQ
	publisher           *rabbitmq.RabbitMQ
	logger              *log.Logger
}

func NewNotificationConsumer(notificationUseCase *usecase.NotificationUseCase, rabbitMQ *rabbitmq.RabbitMQ) *NotificationConsumer {
	logger := log.New(log.Writer(), "[NotificationService] [Saga] ", log.LstdFlags)
	return &NotificationConsumer{
		notificationUseCase: notificationUseCase,
		rabbitMQ:            rabbitMQ,
		publisher:           rabbitMQ,
		logger:              logger,
	}
}

// publishSagaResult отправляет результат шага саги
func (c *NotificationConsumer) publishSagaResult(sagaExch, sagaID, stepName, status string, sagaData []byte, errorMsg string) error {
	routingKey := fmt.Sprintf("saga.%s.result", stepName)

	message := sagahandler.SagaMessage{
		SagaID:    sagaID,
		StepName:  stepName,
		Operation: sagahandler.OperationExecute,
		Status:    sagahandler.SagaStatus(status),
		Data:      sagaData,
		Error:     errorMsg,
		Timestamp: time.Now().Unix(),
	}

	err := c.publisher.PublishMessage(sagaExch, routingKey, message)
	if err != nil {
		c.logger.Printf("[ERROR] SagaID=%s: Не удалось опубликовать результат (%s) шага %s: %v", sagaID, status, stepName, err)
	} else {
		c.logger.Printf("SagaID=%s: Результат (%s) шага %s успешно опубликован.", sagaID, status, stepName)
	}
	return err
}

// publishSuccessResult упрощает отправку успешного результата
func (c *NotificationConsumer) publishSuccessResult(sagaExch, sagaID, stepName string, data []byte) error {
	return c.publishSagaResult(sagaExch, sagaID, stepName, string(sagahandler.StatusCompleted), data, "")
}

// publishFailureResult упрощает отправку неудачного результата
func (c *NotificationConsumer) publishFailureResult(sagaExch, sagaID, stepName, errorMsg string, data []byte) error {
	return c.publishSagaResult(sagaExch, sagaID, stepName, string(sagahandler.StatusFailed), data, errorMsg)
}

// SetupSagaConsumer настраивает очередь и привязку для шага notify_customer саги
func (c *NotificationConsumer) SetupSagaConsumer(sagaExch string) error {
	queueName := "notification_saga_queue"
	routingKey := "saga.notify_customer.execute"

	err := c.rabbitMQ.DeclareQueue(queueName)
	if err != nil {
		return fmt.Errorf("ошибка при создании очереди %s: %w", queueName, err)
	}

	err = c.rabbitMQ.BindQueue(queueName, sagaExch, routingKey)
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди %s к обмену %s с ключом %s: %w", queueName, sagaExch, routingKey, err)
	}

	c.logger.Printf("Настроен обработчик для шага notify_customer (очередь %s)", queueName)
	return nil
}

// handleNotifyCustomer обрабатывает сообщение саги для шага notify_customer
func (c *NotificationConsumer) handleNotifyCustomer(data []byte) error {
	sagaExch := "saga_exchange"

	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		c.logger.Printf("[ERROR] Ошибка парсинга сообщения саги: %v", err)
		return err
	}

	c.logger.Printf("SagaID=%s: Получено сообщение саги для уведомления клиента, StepName=%s",
		message.SagaID, message.StepName)

	var sagaData sagahandler.SagaData
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		c.logger.Printf("SagaID=%s: [ERROR] Ошибка десериализации данных саги: %v", message.SagaID, err)
		_ = c.publishFailureResult(sagaExch, message.SagaID, message.StepName, fmt.Sprintf("ошибка десериализации данных саги: %v", err), message.Data)
		return fmt.Errorf("ошибка десериализации данных саги: %w", err)
	}

	c.logger.Printf("SagaID=%s: Вызываем SendSagaNotification для OrderID=%d, UserID=%d", message.SagaID, sagaData.OrderID, sagaData.UserID)

	err = c.notificationUseCase.SendSagaNotification(context.Background(), sagaData)
	if err != nil {
		c.logger.Printf("SagaID=%s: [ERROR] Ошибка при отправке уведомления для OrderID=%d: %v", message.SagaID, sagaData.OrderID, err)
		_ = c.publishFailureResult(sagaExch, message.SagaID, message.StepName, fmt.Sprintf("ошибка отправки уведомления: %v", err), message.Data)
		return err
	}

	c.logger.Printf("SagaID=%s: Уведомление для OrderID=%d успешно отправлено.", message.SagaID, sagaData.OrderID)
	_ = c.publishSuccessResult(sagaExch, message.SagaID, message.StepName, message.Data)

	return nil
}

// Setup настраивает все необходимые очереди и привязки для сервиса уведомлений
func (c *NotificationConsumer) Setup(orderExch, billingExch, sagaExch string) error {
	// Объявляем exchanges
	err := c.rabbitMQ.DeclareExchange(orderExch, "topic")
	if err != nil {
		return fmt.Errorf("ошибка при объявлении exchange %s: %w", orderExch, err)
	}
	err = c.rabbitMQ.DeclareExchange(billingExch, "topic")
	if err != nil {
		return fmt.Errorf("ошибка при объявлении exchange %s: %w", billingExch, err)
	}
	err = c.rabbitMQ.DeclareExchange(sagaExch, "topic")
	if err != nil {
		return fmt.Errorf("ошибка при объявлении exchange %s: %w", sagaExch, err)
	}

	// --- Очередь для order.notification ---
	orderQueueName := "order_notifications"
	err = c.rabbitMQ.DeclareQueue(orderQueueName)
	if err != nil {
		return fmt.Errorf("ошибка при объявлении очереди %s: %w", orderQueueName, err)
	}
	err = c.rabbitMQ.BindQueue(orderQueueName, orderExch, "order.notification")
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди %s к exchange %s: %w", orderQueueName, orderExch, err)
	}

	// --- Очередь для billing.deposit ---
	depositQueueName := "deposit_notifications"
	err = c.rabbitMQ.DeclareQueue(depositQueueName)
	if err != nil {
		return fmt.Errorf("ошибка при объявлении очереди %s: %w", depositQueueName, err)
	}
	err = c.rabbitMQ.BindQueue(depositQueueName, billingExch, "billing.deposit")
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди %s к exchange %s: %w", depositQueueName, billingExch, err)
	}

	// --- Очередь для billing.insufficient_funds ---
	insufficientFundsQueueName := "insufficient_funds_notifications"
	err = c.rabbitMQ.DeclareQueue(insufficientFundsQueueName)
	if err != nil {
		return fmt.Errorf("ошибка при объявлении очереди %s: %w", insufficientFundsQueueName, err)
	}
	err = c.rabbitMQ.BindQueue(insufficientFundsQueueName, billingExch, "billing.insufficient_funds")
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди %s к exchange %s: %w", insufficientFundsQueueName, billingExch, err)
	}

	// --- Очередь для order.cancelled, order.failed ---
	cancellationQueueName := "order_cancellation_notifications"
	err = c.rabbitMQ.DeclareQueue(cancellationQueueName)
	if err != nil {
		return fmt.Errorf("ошибка при объявлении очереди %s: %w", cancellationQueueName, err)
	}
	err = c.rabbitMQ.BindQueue(cancellationQueueName, orderExch, "order.cancelled")
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди %s к ключу order.cancelled в %s: %w", cancellationQueueName, orderExch, err)
	}
	err = c.rabbitMQ.BindQueue(cancellationQueueName, orderExch, "order.failed")
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди %s к ключу order.failed в %s: %w", cancellationQueueName, orderExch, err)
	}

	// Настройка consumer'а для шага саги
	if err := c.SetupSagaConsumer(sagaExch); err != nil {
		return fmt.Errorf("ошибка настройки saga consumer: %w", err)
	}

	c.logger.Println("Настроены очереди и привязки для notification consumer")
	return nil
}

// StartConsuming начинает обработку сообщений для всех настроенных очередей
func (c *NotificationConsumer) StartConsuming() error {
	var err error

	err = c.rabbitMQ.ConsumeMessages("order_notifications", "notification_service_orders", c.handleOrderNotification)
	if err != nil {
		return fmt.Errorf("ошибка при запуске consumer'а order_notifications: %w", err)
	}

	err = c.rabbitMQ.ConsumeMessages("deposit_notifications", "notification_service_deposits", c.handleDepositNotification)
	if err != nil {
		return fmt.Errorf("ошибка при запуске consumer'а deposit_notifications: %w", err)
	}

	err = c.rabbitMQ.ConsumeMessages("insufficient_funds_notifications", "notification_service_insufficient_funds", c.handleInsufficientFundsNotification)
	if err != nil {
		return fmt.Errorf("ошибка при запуске consumer'а insufficient_funds_notifications: %w", err)
	}

	err = c.rabbitMQ.ConsumeMessages("order_cancellation_notifications", "notification_service_cancellations", c.handleOrderCancellation)
	if err != nil {
		return fmt.Errorf("ошибка при запуске consumer'а order_cancellation_notifications: %w", err)
	}

	err = c.rabbitMQ.ConsumeMessages("notification_saga_queue", "notification_service_saga_step", c.handleNotifyCustomer)
	if err != nil {
		return fmt.Errorf("ошибка при запуске consumer'а notification_saga_queue: %w", err)
	}

	c.logger.Println("Запущены все consumers для notification service")
	return nil
}

// handleOrderNotification обрабатывает уведомление о создании заказа
func (c *NotificationConsumer) handleOrderNotification(body []byte) error {
	var orderNotification entity.OrderNotification

	err := json.Unmarshal(body, &orderNotification)
	if err != nil {
		return fmt.Errorf("ошибка при десериализации сообщения о заказе: %w", err)
	}

	log.Printf("Получено уведомление о заказе: %+v", orderNotification)

	err = c.notificationUseCase.ProcessOrderNotification(context.Background(), orderNotification)
	if err != nil {
		return fmt.Errorf("ошибка при обработке уведомления о заказе: %w", err)
	}

	log.Printf("Уведомление о заказе успешно обработано")
	return nil
}

// handleDepositNotification обрабатывает уведомление о пополнении баланса
func (c *NotificationConsumer) handleDepositNotification(body []byte) error {
	var depositNotification entity.DepositNotification

	err := json.Unmarshal(body, &depositNotification)
	if err != nil {
		return fmt.Errorf("ошибка при десериализации сообщения о пополнении: %w", err)
	}

	log.Printf("Получено уведомление о пополнении баланса: %+v", depositNotification)

	err = c.notificationUseCase.ProcessDepositNotification(context.Background(), depositNotification)
	if err != nil {
		return fmt.Errorf("ошибка при обработке уведомления о пополнении: %w", err)
	}

	log.Printf("Уведомление о пополнении баланса успешно обработано")
	return nil
}

// handleInsufficientFundsNotification обрабатывает уведомление о недостатке средств
func (c *NotificationConsumer) handleInsufficientFundsNotification(body []byte) error {
	var insufficientFundsNotification entity.InsufficientFundsNotification

	err := json.Unmarshal(body, &insufficientFundsNotification)
	if err != nil {
		return fmt.Errorf("ошибка при десериализации сообщения о недостатке средств: %w", err)
	}

	log.Printf("Получено уведомление о недостатке средств: %+v", insufficientFundsNotification)

	err = c.notificationUseCase.ProcessInsufficientFundsNotification(context.Background(), insufficientFundsNotification)
	if err != nil {
		return fmt.Errorf("ошибка при обработке уведомления о недостатке средств: %w", err)
	}

	log.Printf("Уведомление о недостатке средств успешно обработано")
	return nil
}

// handleOrderCancellation обрабатывает уведомление об отмене/ошибке заказа
func (c *NotificationConsumer) handleOrderCancellation(body []byte) error {
	var cancellationEvent usecase.OrderCancellationPayload

	err := json.Unmarshal(body, &cancellationEvent)
	if err != nil {
		c.logger.Printf("[ERROR] Ошибка десериализации сообщения %s: %v", string(body), err)
		return fmt.Errorf("ошибка при десериализации сообщения order.cancelled/failed: %w", err)
	}

	c.logger.Printf("Получено уведомление об отмене/ошибке заказа: %+v", cancellationEvent)

	err = c.notificationUseCase.ProcessOrderCancellation(context.Background(), cancellationEvent)
	if err != nil {
		// Логируем ошибку, но не возвращаем ее, чтобы не блокировать очередь
		c.logger.Printf("[ERROR] Ошибка при обработке уведомления %s для OrderID=%d: %v", cancellationEvent.Type, cancellationEvent.OrderID, err)
		return nil // Возвращаем nil, чтобы сообщение было удалено из очереди
	}

	c.logger.Printf("Уведомление %s для OrderID=%d успешно обработано", cancellationEvent.Type, cancellationEvent.OrderID)
	return nil
}

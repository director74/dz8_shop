package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/director74/dz8_shop/delivery-service/internal/entity"
	"github.com/director74/dz8_shop/delivery-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
	"github.com/director74/dz8_shop/pkg/sagahandler"
)

// SagaConsumer обработчик сообщений саги для доставки
type SagaConsumer struct {
	sagahandler.BaseSagaConsumer
	deliveryUseCase *usecase.DeliveryUseCase
}

// NewSagaConsumer создает новый обработчик сообщений саги для доставки
func NewSagaConsumer(deliveryUseCase *usecase.DeliveryUseCase, rabbitMQ *rabbitmq.RabbitMQ) *SagaConsumer {
	return &SagaConsumer{
		BaseSagaConsumer: sagahandler.BaseSagaConsumer{
			RabbitMQ: rabbitMQ,
			Logger:   log.New(log.Writer(), "[DeliveryService] [Saga] ", log.LstdFlags),
			Step:     "reserve_delivery",
		},
		deliveryUseCase: deliveryUseCase,
	}
}

// Setup настраивает обработчик событий саги
func (c *SagaConsumer) Setup() error {
	return c.SetupQueues(
		"saga_exchange",
		"delivery_reserve_queue",
		"delivery_compensate_queue",
		c.handleReserveDelivery,
		c.handleCompensateDelivery,
	)
}

// handleReserveDelivery обрабатывает сообщение для резервирования курьера
func (c *SagaConsumer) handleReserveDelivery(data []byte) error {

	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		return err
	}

	c.Logger.Printf("SagaID=%s: Получено сообщение саги для резервирования курьера, StepName=%s",
		message.SagaID, message.StepName)

	if len(message.Data) == 0 {
		c.Logger.Printf("SagaID=%s: [WARN] Получены пустые данные в message.Data", message.SagaID)
		return c.PublishFailureResult(message.SagaID, "пустые данные в сообщении")
	}

	var sagaData sagahandler.SagaData
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка десериализации данных заказа: %v", message.SagaID, err)
		return c.PublishFailureResult(message.SagaID,
			fmt.Sprintf("ошибка десериализации данных заказа: %v", err))
	}

	if sagaData.CompensatedSteps == nil {
		sagaData.CompensatedSteps = make(map[string]bool)
	}

	c.Logger.Printf("SagaID=%s: Десериализован заказ: OrderID=%d, UserID=%d", message.SagaID, sagaData.OrderID, sagaData.UserID)

	if sagaData.DeliveryInfo == nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Отсутствует информация о доставке", message.SagaID)
		return c.PublishFailureResultWithData(message.SagaID,
			"отсутствует информация о доставке", message.Data)
	}

	if sagaData.DeliveryInfo.Address == "" {
		c.Logger.Printf("SagaID=%s: [ERROR] Отсутствует адрес доставки", message.SagaID)
		return c.PublishFailureResultWithData(message.SagaID,
			"отсутствует адрес доставки", message.Data)
	}

	if sagaData.DeliveryInfo.TimeSlotID == 0 || sagaData.DeliveryInfo.ZoneID == 0 {
		c.Logger.Printf("SagaID=%s: [ERROR] Отсутствует ID временного слота или зоны доставки", message.SagaID)
		return c.PublishFailureResultWithData(message.SagaID,
			"отсутствует ID временного слота или зоны доставки", message.Data)
	}

	orderID := sagaData.OrderID
	userID := sagaData.UserID

	requestData := map[string]interface{}{
		"order_id":          orderID,
		"user_id":           userID,
		"time_slot_id":      sagaData.DeliveryInfo.TimeSlotID,
		"address":           sagaData.DeliveryInfo.Address,
		"zone_id":           sagaData.DeliveryInfo.ZoneID,
		"compensated_steps": sagaData.CompensatedSteps,
	}

	c.Logger.Printf("SagaID=%s: Выполняем резервирование доставки для заказа ID=%d", message.SagaID, orderID)

	err = c.deliveryUseCase.ReserveForSaga(context.Background(), requestData)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка резервирования курьера: %v", message.SagaID, err)
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("ошибка резервирования курьера: %v", err), message.Data)
	}

	delivery, err := c.deliveryUseCase.GetDeliveryByOrderID(orderID)
	if err != nil || delivery == nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка получения информации о доставке после резервирования: %v", message.SagaID, err)
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("ошибка получения информации о доставке: %v", err), message.Data)
	}

	sagaData.DeliveryInfo.DeliveryID = fmt.Sprintf("%d", delivery.ID)
	sagaData.DeliveryInfo.Status = "reserved"
	sagaData.Status = "delivery_reserved"

	c.Logger.Printf("SagaID=%s: Доставка успешно зарезервирована для заказа ID=%d, DeliveryID=%s",
		message.SagaID, orderID, sagaData.DeliveryInfo.DeliveryID)

	updatedData, err := json.Marshal(sagaData)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка сериализации обновленных данных: %v", message.SagaID, err)
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("ошибка сериализации обновленных данных: %v", err), message.Data)
	}

	if sagaData.PaymentInfo != nil {
		c.Logger.Printf("SagaID=%s: PaymentInfo перед публикацией результата: PaymentID=%s, Status=%s", message.SagaID,
			sagaData.PaymentInfo.PaymentID, sagaData.PaymentInfo.Status)
	} else {
		c.Logger.Printf("SagaID=%s: [WARN] PaymentInfo отсутствует перед публикацией результата", message.SagaID)
	}

	return c.PublishSuccessResult(message.SagaID, updatedData)
}

// handleCompensateDelivery обрабатывает сообщения для компенсации резервирования доставки
func (c *SagaConsumer) handleCompensateDelivery(data []byte) error {

	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		return err
	}

	c.Logger.Printf("SagaID=%s: Получено сообщение саги для компенсации резервирования курьера, StepName=%s",
		message.SagaID, message.StepName)

	var sagaData sagahandler.SagaData
	err = json.Unmarshal(message.Data, &sagaData)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка десериализации данных саги при компенсации: %v", message.SagaID, err)
		// Если не можем распарсить данные, компенсация невозможна.
		_ = c.PublishCompensationResult(message.SagaID, message.Data)
		return fmt.Errorf("ошибка десериализации данных саги при компенсации: %w", err) // Возвращаем ошибку, чтобы сообщить о проблеме
	}

	orderID := sagaData.OrderID
	if orderID == 0 {
		c.Logger.Printf("SagaID=%s: [ERROR] Не удалось получить OrderID из данных саги для компенсации", message.SagaID)
		_ = c.PublishCompensationResult(message.SagaID, message.Data)                   // Попытаемся опубликовать результат с исходными данными
		return errors.New("не удалось получить OrderID из данных саги для компенсации") // Возвращаем ошибку, чтобы сообщить rabbitmq о проблеме
	}

	c.Logger.Printf("SagaID=%s: Получено сообщение на компенсацию доставки для OrderID: %d", message.SagaID, orderID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	releaseErr := c.deliveryUseCase.ReleaseCourier(ctx, &entity.ReleaseCourierRequest{OrderID: orderID})
	if releaseErr != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка компенсации доставки для заказа %d: %v", message.SagaID, orderID, releaseErr)
		_ = c.PublishCompensationResult(message.SagaID, message.Data) // Попытаемся опубликовать результат с исходными данными
		return fmt.Errorf("ошибка компенсации доставки для заказа %d: %w", orderID, releaseErr)
	}

	sagaData.CompensatedSteps["reserve_delivery"] = true
	sagaData.Status = "delivery_compensated"

	if sagaData.PaymentInfo != nil {
		c.Logger.Printf("SagaID=%s: PaymentInfo при компенсации: PaymentID=%s, Status=%s", message.SagaID,
			sagaData.PaymentInfo.PaymentID, sagaData.PaymentInfo.Status)
	} else {
		c.Logger.Printf("SagaID=%s: PaymentInfo отсутствует при компенсации", message.SagaID)
	}

	updatedData, err := json.Marshal(sagaData)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка сериализации обновленных данных после компенсации: %v", message.SagaID, err)
		_ = c.PublishCompensationResult(message.SagaID, message.Data)
		return fmt.Errorf("ошибка сериализации обновленных данных после компенсации для заказа %d: %w", orderID, err) // Возвращаем ошибку
	}

	c.Logger.Printf("SagaID=%s: Компенсация доставки успешно завершена для заказа ID=%d, статус: %s",
		message.SagaID, sagaData.OrderID, sagaData.Status)
	return c.PublishCompensationResult(message.SagaID, updatedData) // Возвращаем результат публикации (nil или error)
}

// SetupConfirmConsumer настраивает consumer для шага подтверждения заказа
func (c *SagaConsumer) SetupConfirmConsumer() error {
	queueName := "delivery_confirm_queue"
	exchangeName := "saga_exchange"
	routingKey := "saga.confirm_order.execute"

	err := c.RabbitMQ.DeclareQueue(queueName)
	if err != nil {
		return fmt.Errorf("ошибка при создании очереди %s: %w", queueName, err)
	}

	err = c.RabbitMQ.BindQueue(queueName, exchangeName, routingKey)
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди %s к обмену %s с ключом %s: %w", queueName, exchangeName, routingKey, err)
	}

	consumerTag := "delivery_confirm_consumer_" + queueName
	err = c.RabbitMQ.ConsumeMessages(queueName, consumerTag, c.handleConfirmDelivery)
	if err != nil {
		return fmt.Errorf("ошибка при запуске consumer'а для очереди %s: %w", queueName, err)
	}

	c.Logger.Printf("Настроен обработчик для шага confirm_order (очередь %s)", queueName)
	return nil
}

// handleConfirmDelivery обрабатывает сообщение для подтверждения (запуска) доставки
func (c *SagaConsumer) handleConfirmDelivery(data []byte) error {
	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		return err // Ошибка парсинга, сообщение будет переотправлено или уйдет в DLQ
	}

	c.Logger.Printf("SagaID=%s: Получено сообщение саги для подтверждения доставки, StepName=%s",
		message.SagaID, message.StepName)

	var sagaData sagahandler.SagaData
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка десериализации данных заказа при подтверждении: %v", message.SagaID, err)
		_ = c.PublishFailureResult(message.SagaID, fmt.Sprintf("ошибка десериализации данных заказа: %v", err))
		return fmt.Errorf("ошибка десериализации данных заказа: %w", err)
	}

	// Формируем данные для use case (по аналогии с ConfirmForSaga)
	// Добавляем saga_id и saga_data для передачи в use case
	reqData := map[string]interface{}{
		"order_id":  sagaData.OrderID,
		"saga_id":   message.SagaID, // Передаем SagaID
		"saga_data": sagaData,       // Передаем все данные саги
	}

	c.Logger.Printf("SagaID=%s: Вызываем ConfirmForSaga для OrderID=%d", message.SagaID, sagaData.OrderID)

	// Вызываем use case. ConfirmForSaga должен обработать подтверждение и запустить
	// асинхронную имитацию доставки. Результат обратно должен отправить simulateDeliveryCompletion.
	err = c.deliveryUseCase.ConfirmForSaga(context.Background(), reqData)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка при вызове ConfirmForSaga для OrderID=%d: %v", message.SagaID, sagaData.OrderID, err)
		// Публикуем неудачный результат обратно в order-service
		_ = c.PublishFailureResultWithData(message.SagaID, fmt.Sprintf("ошибка подтверждения доставки: %v", err), message.Data)
		return err // Возвращаем ошибку, чтобы RabbitMQ знал о проблеме
	}

	// Если ConfirmForSaga вернул nil, значит команда принята к исполнению.
	// Мы не отправляем SuccessResult здесь, так как фактический результат шага
	// (доставка завершена) придет позже от simulateDeliveryCompletion.
	c.Logger.Printf("SagaID=%s: Команда confirm_order для OrderID=%d принята к исполнению.", message.SagaID, sagaData.OrderID)
	return nil // Возвращаем nil, чтобы подтвердить получение сообщения RabbitMQ
}

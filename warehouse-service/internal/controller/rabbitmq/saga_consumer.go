package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/director74/dz8_shop/pkg/rabbitmq"
	"github.com/director74/dz8_shop/pkg/sagahandler"
	"github.com/director74/dz8_shop/warehouse-service/internal/entity"
	"github.com/director74/dz8_shop/warehouse-service/internal/usecase"
)

// SagaConsumer обработчик сообщений саги для склада
type SagaConsumer struct {
	sagahandler.BaseSagaConsumer
	warehouseUseCase *usecase.WarehouseUseCase
}

// NewSagaConsumer создает новый обработчик сообщений саги для склада
func NewSagaConsumer(warehouseUseCase *usecase.WarehouseUseCase, rabbitMQ *rabbitmq.RabbitMQ) *SagaConsumer {
	return &SagaConsumer{
		BaseSagaConsumer: sagahandler.BaseSagaConsumer{
			RabbitMQ: rabbitMQ,
			Logger:   log.New(log.Writer(), "[WarehouseService] [Saga] ", log.LstdFlags),
			Step:     "reserve_warehouse",
		},
		warehouseUseCase: warehouseUseCase,
	}
}

// Setup настраивает обработчик событий саги
func (c *SagaConsumer) Setup() error {
	return c.SetupQueues(
		"saga_exchange",              // exchangeName
		"warehouse_reserve_queue",    // executeQueueName
		"warehouse_compensate_queue", // compensateQueueName
		c.handleReserveWarehouse,     // handleExecute
		c.handleCompensateWarehouse,  // handleCompensate
	)
}

// handleReserveWarehouse обрабатывает сообщение для резервирования на складе
func (c *SagaConsumer) handleReserveWarehouse(data []byte) error {
	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		return err
	}

	// Используем новый формат лога
	c.Logger.Printf("SagaID=%s: Получено сообщение execute для шага %s", message.SagaID, message.StepName)

	// Логируем сырые данные перед десериализацией
	// c.Logger.Printf("Получены сырые данные для SagaData: %s", string(message.Data)) // Можно оставить для отладки при необходимости

	// Парсим данные заказа из сообщения
	var sagaData sagahandler.SagaData
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		// Логгируем ошибку десериализации
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка десериализации данных: %v", message.SagaID, err)
		// Отправляем результат с ошибкой (PublishFailureResult не логирует сам)
		return c.PublishFailureResult(message.SagaID,
			fmt.Sprintf("ошибка десериализации данных заказа: %v", err))
	}

	// Логируем десериализованные данные (можно убрать или сделать INFO уровнем)
	// c.Logger.Printf("SagaID=%s: Десериализованные SagaData: %+v", message.SagaID, sagaData)

	if len(sagaData.Items) == 0 {
		// Логируем отсутствие товаров
		c.Logger.Printf("[ERROR] SagaID=%s: Заказ OrderID=%d не содержит товаров для резервирования", message.SagaID, sagaData.OrderID)
		return c.PublishFailureResultWithData(message.SagaID,
			"заказ не содержит товаров для резервирования", message.Data)
	}

	if sagaData.CompensatedSteps == nil {
		sagaData.CompensatedSteps = make(map[string]bool)
	}

	reservationItems := make([]entity.WarehouseReservationItem, 0, len(sagaData.Items))
	for i := 0; i < len(sagaData.Items); i++ {
		reservationItems = append(reservationItems, entity.WarehouseReservationItem{
			ProductID: sagaData.Items[i].ProductID,
			Quantity:  uint(sagaData.Items[i].Quantity),
		})
	}

	reserveRequest := &entity.ReserveWarehouseRequest{
		OrderID: sagaData.OrderID,
		UserID:  sagaData.UserID,
		Items:   make([]entity.ReserveItem, 0, len(sagaData.Items)),
	}
	for i := 0; i < len(sagaData.Items); i++ {
		reserveRequest.Items = append(reserveRequest.Items, entity.ReserveItem{
			ProductID: sagaData.Items[i].ProductID,
			Quantity:  int(sagaData.Items[i].Quantity),
		})
	}

	result, err := c.warehouseUseCase.ReserveWarehouseItems(context.Background(), reserveRequest)
	if err != nil {
		// Логируем ошибку резервирования
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка резервирования для OrderID=%d: %v", message.SagaID, sagaData.OrderID, err)
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("ошибка резервирования на складе: %v", err), message.Data)
	}

	// Логируем успешное резервирование
	c.Logger.Printf("SagaID=%s: Резервирование для OrderID=%d выполнено успешно. ReservationID: %d", message.SagaID, sagaData.OrderID, result.OrderID)

	if len(result.ReservedItems) > 0 {
		if sagaData.WarehouseInfo == nil {
			sagaData.WarehouseInfo = &sagahandler.WarehouseInfo{}
		}
		// Используем OrderID из реквеста как ReservationID, так как в usecase он так и возвращается
		sagaData.WarehouseInfo.ReservationID = fmt.Sprintf("%d", sagaData.OrderID)
		sagaData.Status = "warehouse_reserved"
	} else {
		// Этот случай маловероятен, если ReserveWarehouseItems вернул nil error
		c.Logger.Printf("[ERROR] SagaID=%s: Резервирование для OrderID=%d не вернуло зарезервированных товаров, хотя ошибки не было", message.SagaID, sagaData.OrderID)
		return c.PublishFailureResultWithData(message.SagaID,
			"ошибка резервирования на складе: нет зарезервированных товаров", message.Data)
	}

	updatedData, err := json.Marshal(sagaData)
	if err != nil {
		// Логируем ошибку сериализации
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка сериализации обновленных данных для OrderID=%d: %v", message.SagaID, sagaData.OrderID, err)
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("ошибка сериализации обновленных данных: %v", err), message.Data)
	}

	// Логируем отправку успешного результата
	c.Logger.Printf("SagaID=%s: Отправка успешного результата шага %s", message.SagaID, c.Step)
	return c.PublishSuccessResult(message.SagaID, updatedData)
}

// handleCompensateWarehouse обрабатывает сообщение для компенсации резервирования на складе
func (c *SagaConsumer) handleCompensateWarehouse(data []byte) error {
	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		// Ошибка парсинга самого сообщения, SagaID может быть недоступен
		c.Logger.Printf("[ERROR] Ошибка парсинга сообщения компенсации: %v", err)
		return err // Просто возвращаем ошибку, не публикуя результат
	}

	// Используем новый формат лога
	c.Logger.Printf("SagaID=%s: Получено сообщение compensate для шага %s", message.SagaID, message.StepName)

	var sagaData sagahandler.SagaData
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		// Логируем ошибку десериализации
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка десериализации данных компенсации: %v", message.SagaID, err)
		// Не можем отправить Failed, так как это compensate. Оркестратор должен обработать таймаут.
		return err
	}

	if sagaData.WarehouseInfo != nil && sagaData.WarehouseInfo.ReservationID != "" {
		reservationIDstr := sagaData.WarehouseInfo.ReservationID
		reservationID := sagahandler.ParseUint(reservationIDstr)
		releaseRequest := &entity.ReleaseWarehouseRequest{
			// Используем ID из WarehouseInfo как OrderID для отмены
			OrderID: reservationID,
			UserID:  sagaData.UserID,
		}
		if err := c.warehouseUseCase.ReleaseWarehouseItems(context.Background(), releaseRequest); err != nil {
			// Логируем ошибку отмены
			c.Logger.Printf("[ERROR] SagaID=%s: Ошибка отмены резервирования %s (OrderID=%d): %v", message.SagaID, reservationIDstr, sagaData.OrderID, err)
			// TODO: Решить, нужно ли отправлять compensate/failed. Пока просто логируем.
		} else {
			// Логируем успешную отмену
			c.Logger.Printf("SagaID=%s: Резервирование %s (OrderID=%d) успешно отменено.", message.SagaID, reservationIDstr, sagaData.OrderID)
		}
	} else {
		// Логируем отсутствие ID
		c.Logger.Printf("SagaID=%s: Нет ID резервирования для компенсации. OrderID=%d", message.SagaID, sagaData.OrderID)
	}

	sagaData.Status = "warehouse_released"
	if sagaData.WarehouseInfo != nil {
		sagaData.WarehouseInfo.ReservationID = ""
	}

	if sagaData.CompensatedSteps == nil {
		sagaData.CompensatedSteps = make(map[string]bool)
	}

	updatedData, err := json.Marshal(sagaData)
	if err != nil {
		// Логируем ошибку сериализации
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка сериализации данных после компенсации для OrderID=%d: %v", message.SagaID, sagaData.OrderID, err)
		return err // Не можем отправить результат
	}

	// Логируем отправку результата компенсации
	c.Logger.Printf("SagaID=%s: Отправка результата compensate/compensated шага %s", message.SagaID, c.Step)
	return c.PublishCompensationResult(message.SagaID, updatedData)
}

package sagahandler

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/director74/dz8_shop/pkg/rabbitmq"
)

// SagaOperation --- Добавляем константы ---
type SagaOperation string
type SagaStatus string

const (
	OperationExecute    SagaOperation = "execute"
	OperationCompensate SagaOperation = "compensate"
)

const (
	StatusPending     SagaStatus = "pending"
	StatusCompleted   SagaStatus = "completed"
	StatusFailed      SagaStatus = "failed"
	StatusCompensated SagaStatus = "compensated" // Шаг был компенсирован
	// StatusRunning добавим еще статус Running, хоть он и не используется в SagaMessage напрямую, но может быть полезен для статуса SagaState в БД
	StatusRunning SagaStatus = "running"
)

// SagaMessage представляет сообщение для оркестрации саги
type SagaMessage struct {
	SagaID    string          `json:"saga_id"`
	StepName  string          `json:"step_name"`
	Operation SagaOperation   `json:"operation"` // Используем тип SagaOperation
	Status    SagaStatus      `json:"status"`    // Используем тип SagaStatus
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// OrderItem представляет элемент заказа в саге
type OrderItem struct {
	ID        uint      `json:"id,omitempty"`
	OrderID   uint      `json:"order_id,omitempty"`
	ProductID uint      `json:"product_id"`
	Name      string    `json:"name,omitempty"`
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// PaymentInfo информация о платеже
type PaymentInfo struct {
	PaymentID     string  `json:"payment_id,omitempty"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	TransactionID string  `json:"transaction_id,omitempty"`
}

// DeliveryInfo информация о доставке
type DeliveryInfo struct {
	DeliveryID   string  `json:"delivery_id,omitempty"`
	Address      string  `json:"address"`
	DeliveryDate string  `json:"delivery_date"`
	Cost         float64 `json:"cost"`
	Status       string  `json:"status"`
	TimeSlotID   uint    `json:"time_slot_id,omitempty"`
	ZoneID       uint    `json:"zone_id,omitempty"`
}

// WarehouseInfo информация о резервации товаров на складе
type WarehouseInfo struct {
	ReservationID string `json:"reservation_id,omitempty"`
	Status        string `json:"status"`
}

// BillingInfo информация о биллинге
type BillingInfo struct {
	TransactionID string  `json:"transaction_id,omitempty"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
}

// SagaData представляет данные для передачи между шагами саги
type SagaData struct {
	OrderID          uint            `json:"order_id"`
	UserID           uint            `json:"user_id"`
	Items            []OrderItem     `json:"items"`
	Amount           float64         `json:"amount"`
	Status           string          `json:"status"` // Оставляем string для совместимости с entity.OrderStatus? Или нужно привести к SagaStatus? Пока оставим string.
	PaymentInfo      *PaymentInfo    `json:"payment_info,omitempty"`
	DeliveryInfo     *DeliveryInfo   `json:"delivery_info,omitempty"`
	WarehouseInfo    *WarehouseInfo  `json:"warehouse_info,omitempty"`
	BillingInfo      *BillingInfo    `json:"billing_info,omitempty"`
	Error            string          `json:"error,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	CompensatedSteps map[string]bool `json:"compensated_steps,omitempty"`
}

// BaseSagaConsumer базовый обработчик сообщений саги
type BaseSagaConsumer struct {
	RabbitMQ *rabbitmq.RabbitMQ
	Logger   *log.Logger
	Step     string // шаг, за который отвечает этот обработчик
}

// SetupQueues настраивает очереди и обмены для обработки саги
func (b *BaseSagaConsumer) SetupQueues(
	exchangeName string,
	reserveQueueName string,
	compensateQueueName string,
	handleExecute func([]byte) error,
	handleCompensate func([]byte) error,
) error {
	// Объявляем exchange для саги
	err := b.RabbitMQ.DeclareExchange(exchangeName, "topic")
	if err != nil {
		return fmt.Errorf("ошибка при объявлении exchange для саги: %w", err)
	}

	// Объявляем очередь для обработки шага
	err = b.RabbitMQ.DeclareQueue(reserveQueueName)
	if err != nil {
		return fmt.Errorf("ошибка при объявлении очереди для выполнения шага: %w", err)
	}

	// Объявляем очередь для компенсации
	err = b.RabbitMQ.DeclareQueue(compensateQueueName)
	if err != nil {
		return fmt.Errorf("ошибка при объявлении очереди для компенсации: %w", err)
	}

	// Привязываем очереди к соответствующим ключам маршрутизации
	executeRoutingKey := fmt.Sprintf("saga.%s.execute", b.Step)
	err = b.RabbitMQ.BindQueue(reserveQueueName, exchangeName, executeRoutingKey)
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди к ключу %s: %w", executeRoutingKey, err)
	}

	compensateRoutingKey := fmt.Sprintf("saga.%s.compensate", b.Step)
	err = b.RabbitMQ.BindQueue(compensateQueueName, exchangeName, compensateRoutingKey)
	if err != nil {
		return fmt.Errorf("ошибка при привязке очереди к ключу %s: %w", compensateRoutingKey, err)
	}

	// Настраиваем обработчик сообщений для выполнения шага
	consumerExecuteName := fmt.Sprintf("%s-execute-%d", b.Step, time.Now().UnixNano())
	err = b.RabbitMQ.ConsumeMessages(reserveQueueName, consumerExecuteName, handleExecute)
	if err != nil {
		return fmt.Errorf("ошибка при настройке обработчика сообщений для выполнения: %w", err)
	}

	// Настраиваем обработчик сообщений для компенсации
	consumerCompensateName := fmt.Sprintf("%s-compensate-%d", b.Step, time.Now().UnixNano())
	err = b.RabbitMQ.ConsumeMessages(compensateQueueName, consumerCompensateName, handleCompensate)
	if err != nil {
		return fmt.Errorf("ошибка при настройке обработчика сообщений для компенсации: %w", err)
	}

	b.Logger.Printf("Настроена обработка сообщений саги для шага %s", b.Step)
	return nil
}

// PublishSuccessResult публикует сообщение об успешном выполнении шага
func (b *BaseSagaConsumer) PublishSuccessResult(sagaID string, data []byte) error {
	// Логируем содержимое данных для отладки
	var sagaData SagaData
	if err := json.Unmarshal(data, &sagaData); err == nil {
		// Проверяем наличие критических полей
		if sagaData.OrderID > 0 {
			if sagaData.PaymentInfo != nil {
				b.Logger.Printf("Публикация успешного результата для OrderID=%d с PaymentID=%s, CompensatedSteps=%v",
					sagaData.OrderID, sagaData.PaymentInfo.PaymentID, sagaData.CompensatedSteps)
			} else {
				b.Logger.Printf("Публикация успешного результата для OrderID=%d, PaymentInfo=nil, CompensatedSteps=%v",
					sagaData.OrderID, sagaData.CompensatedSteps)
			}
		} else {
			b.Logger.Printf("ВНИМАНИЕ: Публикация успешного результата с OrderID=0, CompensatedSteps=%v",
				sagaData.CompensatedSteps)
		}
	}

	resultMessage := SagaMessage{
		SagaID:    sagaID,
		StepName:  b.Step,
		Operation: OperationExecute, // Используем константу
		Status:    StatusCompleted,  // Используем константу
		Data:      data,
		Timestamp: GetTimestamp(),
	}

	resultRoutingKey := fmt.Sprintf("saga.%s.result", b.Step)
	if err := b.RabbitMQ.PublishMessage("saga_exchange", resultRoutingKey, resultMessage); err != nil {
		b.Logger.Printf("Ошибка при публикации результата выполнения шага %s: %v", b.Step, err)
		return err
	}

	b.Logger.Printf("Шаг %s саги %s успешно выполнен", b.Step, sagaID)
	return nil
}

// PublishFailureResult публикует сообщение о неудачном выполнении шага
// Отправляет команду на компенсацию этого же шага (OperationCompensate)
// со статусом Failed.
func (b *BaseSagaConsumer) PublishFailureResult(sagaID string, errorMsg string) error {
	failureMessage := SagaMessage{
		SagaID:    sagaID,
		StepName:  b.Step,
		Operation: OperationCompensate, // Используем константу
		Status:    StatusFailed,        // Используем константу
		Error:     errorMsg,
		Timestamp: GetTimestamp(),
	}

	// Отправляем результат на общий ключ *.result, чтобы оркестратор получил уведомление
	resultRoutingKey := fmt.Sprintf("saga.%s.result", b.Step)
	if err := b.RabbitMQ.PublishMessage("saga_exchange", resultRoutingKey, failureMessage); err != nil {
		b.Logger.Printf("Ошибка при публикации сообщения о неудаче шага %s: %v", b.Step, err)
		return err
	}

	b.Logger.Printf("Опубликовано сообщение о неудаче шага %s саги %s: %s", b.Step, sagaID, errorMsg)
	return nil
}

// PublishFailureResultWithData публикует сообщение о неудачном выполнении шага с сохранением данных отправляет команду на компенсацию этого же шага (OperationCompensate) со статусом Failed.
func (b *BaseSagaConsumer) PublishFailureResultWithData(sagaID string, errorMsg string, data []byte) error {
	// Логируем содержимое данных для отладки
	var sagaData SagaData
	if err := json.Unmarshal(data, &sagaData); err == nil {
		// Проверяем наличие критических полей
		if sagaData.OrderID > 0 {
			b.Logger.Printf("Публикация неудачного результата для OrderID=%d с ошибкой: %s",
				sagaData.OrderID, errorMsg)
			if sagaData.PaymentInfo != nil {
				b.Logger.Printf("PaymentInfo присутствует в данных: PaymentID=%s",
					sagaData.PaymentInfo.PaymentID)
			}
		} else {
			b.Logger.Printf("ВНИМАНИЕ: Публикация неудачного результата с OrderID=0, ошибка: %s",
				errorMsg)
		}
	} else {
		b.Logger.Printf("Ошибка при разборе данных: %v", err)
	}

	failureMessage := SagaMessage{
		SagaID:    sagaID,
		StepName:  b.Step,
		Operation: OperationCompensate,
		Status:    StatusFailed,
		Error:     errorMsg,
		Data:      data, // Сохраняем данные для шага компенсации
		Timestamp: GetTimestamp(),
	}

	// Отправляем результат на общий ключ *.result, чтобы оркестратор получил уведомление
	resultRoutingKey := fmt.Sprintf("saga.%s.result", b.Step)
	if err := b.RabbitMQ.PublishMessage("saga_exchange", resultRoutingKey, failureMessage); err != nil {
		b.Logger.Printf("Ошибка при публикации сообщения о неудаче с данными для шага %s: %v", b.Step, err)
		return err
	}

	b.Logger.Printf("Опубликовано сообщение о неудаче шага %s саги %s (с данными): %s", b.Step, sagaID, errorMsg)
	return nil
}

// PublishCompensationResult публикует сообщение о выполнении компенсации
func (b *BaseSagaConsumer) PublishCompensationResult(sagaID string, data []byte) error {
	compensationMessage := SagaMessage{
		SagaID:    sagaID,
		StepName:  b.Step,
		Operation: OperationCompensate, // Используем константу
		Status:    StatusCompensated,   // Используем константу
		Data:      data,
		Timestamp: GetTimestamp(),
	}

	// Отправляем результат на общий ключ *.result, чтобы оркестратор получил уведомление
	resultRoutingKey := fmt.Sprintf("saga.%s.result", b.Step)
	if err := b.RabbitMQ.PublishMessage("saga_exchange", resultRoutingKey, compensationMessage); err != nil {
		b.Logger.Printf("Ошибка при публикации результата компенсации шага %s: %v", b.Step, err)
		return err
	}

	b.Logger.Printf("Шаг %s саги %s успешно компенсирован", b.Step, sagaID)
	return nil
}

// ParseSagaMessage десериализует SagaMessage из байтов
func ParseSagaMessage(data []byte) (*SagaMessage, error) {
	var message SagaMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, fmt.Errorf("ошибка десериализации сообщения саги: %w", err)
	}
	return &message, nil
}

// ParseUint преобразует разные типы в uint
func ParseUint(value interface{}) uint {
	switch v := value.(type) {
	case uint:
		return v
	case int:
		return uint(v)
	case float64:
		return uint(v)
	case string:
		var result uint
		fmt.Sscanf(v, "%d", &result)
		return result
	default:
		return 0
	}
}

// GetTimestamp возвращает текущее время в формате Unix timestamp
func GetTimestamp() int64 {
	return time.Now().Unix()
}

// NewSagaMessage создает новое сообщение саги
func NewSagaMessage(sagaID, stepName string, operation SagaOperation, status SagaStatus, data interface{}) (SagaMessage, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return SagaMessage{}, fmt.Errorf("ошибка сериализации данных саги: %w", err)
	}
	return SagaMessage{
		SagaID:    sagaID,
		StepName:  stepName,
		Operation: operation,
		Status:    status,
		Data:      jsonData,
		Timestamp: GetTimestamp(),
	}, nil
}

// NewSagaErrorMessage создает сообщение саги с ошибкой
func NewSagaErrorMessage(sagaID, stepName string, operation SagaOperation, status SagaStatus, err error) SagaMessage {
	msg := SagaMessage{
		SagaID:    sagaID,
		StepName:  stepName,
		Operation: operation,
		Status:    status,
		Timestamp: GetTimestamp(),
	}
	if err != nil {
		msg.Error = err.Error()
	}
	return msg
}

// ParseSagaData извлекает данные из сообщения саги
func ParseSagaData(message SagaMessage) (SagaData, error) {
	var sagaData SagaData
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		return sagaData, fmt.Errorf("ошибка при десериализации данных саги: %w", err)
	}
	return sagaData, nil
}

// UpdateSagaData обновляет данные сообщения саги
func UpdateSagaData(message SagaMessage, sagaData SagaData) (SagaMessage, error) {
	dataBytes, err := json.Marshal(sagaData)
	if err != nil {
		return message, fmt.Errorf("ошибка маршалинга обновленных данных: %w", err)
	}

	message.Data = dataBytes
	return message, nil
}

// DebugSagaData печатает содержимое sagaData для отладки
func DebugSagaData(sagaData SagaData) string {
	// Формируем отладочную информацию
	debug := fmt.Sprintf("SagaData: OrderID=%d, UserID=%d, Amount=%.2f, Status=%s\n",
		sagaData.OrderID, sagaData.UserID, sagaData.Amount, sagaData.Status)

	debug += fmt.Sprintf("Items count: %d\n", len(sagaData.Items))
	for i, item := range sagaData.Items {
		debug += fmt.Sprintf("Item %d: ProductID=%d, Quantity=%d, Price=%.2f\n",
			i+1, item.ProductID, item.Quantity, item.Price)
	}

	// Добавляем информацию о доставке, если она есть
	if sagaData.DeliveryInfo != nil {
		debug += fmt.Sprintf("DeliveryInfo: Address=%s, Date=%s, Cost=%.2f, TimeSlotID=%d, ZoneID=%d\n",
			sagaData.DeliveryInfo.Address, sagaData.DeliveryInfo.DeliveryDate,
			sagaData.DeliveryInfo.Cost, sagaData.DeliveryInfo.TimeSlotID, sagaData.DeliveryInfo.ZoneID)
	} else {
		debug += "DeliveryInfo: <nil>\n"
	}

	return debug
}

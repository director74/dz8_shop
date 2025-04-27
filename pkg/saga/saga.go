package saga

import (
	"encoding/json"
	"fmt"
	"time"
)

// NewSagaMessage создает новое сообщение саги
func NewSagaMessage(sagaID, stepName, operation, status string, data interface{}) (SagaMessage, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return SagaMessage{}, fmt.Errorf("ошибка маршалинга данных: %w", err)
	}

	return SagaMessage{
		SagaID:    sagaID,
		StepName:  stepName,
		Operation: operation,
		Status:    status,
		Data:      dataBytes,
		Timestamp: time.Now().Unix(),
	}, nil
}

// NewSagaErrorMessage создает сообщение саги с ошибкой
func NewSagaErrorMessage(sagaID, stepName, operation string, err error) SagaMessage {
	return SagaMessage{
		SagaID:    sagaID,
		StepName:  stepName,
		Operation: operation,
		Status:    "failed",
		Error:     err.Error(),
		Timestamp: time.Now().Unix(),
	}
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

	return debug
}

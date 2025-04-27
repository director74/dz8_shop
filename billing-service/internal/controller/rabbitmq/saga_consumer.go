package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/director74/dz8_shop/billing-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
	"github.com/director74/dz8_shop/pkg/sagahandler"
)

// SagaConsumer обработчик сообщений саги для биллинга
type SagaConsumer struct {
	sagahandler.BaseSagaConsumer
	billingUseCase *usecase.BillingUseCase
}

// NewSagaConsumer создает новый обработчик сообщений саги для биллинга
func NewSagaConsumer(billingUseCase *usecase.BillingUseCase, rabbitMQ *rabbitmq.RabbitMQ) *SagaConsumer {
	return &SagaConsumer{
		BaseSagaConsumer: sagahandler.BaseSagaConsumer{
			RabbitMQ: rabbitMQ,
			Logger:   log.New(log.Writer(), "[BillingService] [Saga] ", log.LstdFlags),
			Step:     "process_billing",
		},
		billingUseCase: billingUseCase,
	}
}

// Setup настраивает обработчик событий саги
func (c *SagaConsumer) Setup() error {
	return c.SetupQueues(
		"saga_exchange",            // exchangeName
		"billing_process_queue",    // executeQueueName
		"billing_compensate_queue", // compensateQueueName
		c.handleProcessBilling,     // handleExecute
		c.handleCompensateBilling,  // handleCompensate
	)
}

// handleProcessBilling обрабатывает сообщение для проведения платежа
func (c *SagaConsumer) handleProcessBilling(data []byte) error {
	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		c.Logger.Printf("[ERROR] Ошибка парсинга сообщения execute: %v", err)
		return err
	}

	c.Logger.Printf("SagaID=%s: Получено сообщение execute для шага %s", message.SagaID, message.StepName)

	var sagaData sagahandler.SagaData
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка десериализации данных: %v", message.SagaID, err)
		return c.PublishFailureResult(message.SagaID,
			fmt.Sprintf("ошибка десериализации данных заказа: %v", err))
	}

	if sagaData.CompensatedSteps == nil {
		sagaData.CompensatedSteps = make(map[string]bool)
	}

	c.Logger.Printf("SagaID=%s: Обработка биллинга для OrderID=%d, UserID=%d, Amount=%.2f",
		message.SagaID, sagaData.OrderID, sagaData.UserID, sagaData.Amount)

	if sagaData.Amount <= 0 {
		c.Logger.Printf("[ERROR] SagaID=%s: Некорректная сумма заказа (<= 0): %.2f", message.SagaID, sagaData.Amount)
		return c.PublishFailureResultWithData(message.SagaID,
			"сумма заказа должна быть больше нуля", message.Data)
	}

	transaction, err := c.billingUseCase.Withdraw(context.Background(), sagaData.UserID, sagaData.Amount, "")
	if err != nil {
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка вызова Withdraw для UserID=%d: %v", message.SagaID, sagaData.UserID, err)
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("ошибка списания средств: %v", err), message.Data)
	}

	if !transaction.Success {
		c.Logger.Printf("[WARN] SagaID=%s: Списание средств не выполнено для UserID=%d (недостаточно средств?) TransactionID=%d",
			message.SagaID, sagaData.UserID, transaction.Transaction.ID)
		sagaData.Status = "billing_failed"
		if sagaData.BillingInfo == nil {
			sagaData.BillingInfo = &sagahandler.BillingInfo{}
		}
		sagaData.BillingInfo.TransactionID = fmt.Sprintf("%d", transaction.Transaction.ID)
		sagaData.BillingInfo.Amount = transaction.Transaction.Amount
		sagaData.BillingInfo.Status = transaction.Transaction.Status
		updatedData, err := json.Marshal(sagaData)
		if err != nil {
			c.Logger.Printf("[ERROR] SagaID=%s: Ошибка сериализации данных после неудачного списания: %v", message.SagaID, err)
			return c.PublishFailureResultWithData(message.SagaID,
				fmt.Sprintf("недостаточно средств на счете пользователя %d", sagaData.UserID), message.Data)
		}
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("недостаточно средств на счете пользователя %d", sagaData.UserID), updatedData)
	}

	c.Logger.Printf("SagaID=%s: Списание средств для UserID=%d выполнено успешно. TransactionID=%d",
		message.SagaID, sagaData.UserID, transaction.Transaction.ID)

	if sagaData.BillingInfo == nil {
		sagaData.BillingInfo = &sagahandler.BillingInfo{}
	}
	sagaData.Status = "billing_processed"
	sagaData.BillingInfo.TransactionID = fmt.Sprintf("%d", transaction.Transaction.ID)
	sagaData.BillingInfo.Amount = transaction.Transaction.Amount
	sagaData.BillingInfo.Status = transaction.Transaction.Status

	updatedData, err := json.Marshal(sagaData)
	if err != nil {
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка сериализации данных после успешного списания: %v", message.SagaID, err)
		return c.PublishFailureResult(message.SagaID,
			fmt.Sprintf("ошибка сериализации обновленных данных: %v", err))
	}

	c.Logger.Printf("SagaID=%s: Отправка успешного результата шага %s", message.SagaID, c.Step)
	return c.PublishSuccessResult(message.SagaID, updatedData)
}

// handleCompensateBilling обрабатывает сообщение для компенсации платежа
func (c *SagaConsumer) handleCompensateBilling(data []byte) error {
	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		c.Logger.Printf("[ERROR] Ошибка парсинга сообщения compensate: %v", err)
		return err
	}

	c.Logger.Printf("SagaID=%s: Получено сообщение compensate для шага %s",
		message.SagaID, message.StepName)

	var sagaData sagahandler.SagaData
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка десериализации данных компенсации: %v", message.SagaID, err)
		return err
	}

	if sagaData.CompensatedSteps == nil {
		sagaData.CompensatedSteps = make(map[string]bool)
	}

	c.Logger.Printf("SagaID=%s: Обработка компенсации биллинга для OrderID=%d, UserID=%d",
		message.SagaID, sagaData.OrderID, sagaData.UserID)

	if sagaData.BillingInfo == nil || sagaData.BillingInfo.TransactionID == "" {
		c.Logger.Printf("[WARN] SagaID=%s: Нет данных о транзакции для компенсации (OrderID=%d). Считаем компенсацию выполненной.",
			message.SagaID, sagaData.OrderID)
		sagaData.Status = "billing_compensation_completed"
		updatedData, err := json.Marshal(sagaData)
		if err != nil {
			c.Logger.Printf("[ERROR] SagaID=%s: Ошибка сериализации данных после компенсации (нет транзакции): %v", message.SagaID, err)
			return err
		}
		return c.PublishCompensationResult(message.SagaID, updatedData)
	}

	amount := sagaData.BillingInfo.Amount
	transactionID := sagaData.BillingInfo.TransactionID
	_, err = c.billingUseCase.Deposit(context.Background(), sagaData.UserID, amount, "")
	if err != nil {
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка возврата средств (Deposit) для UserID=%d, Amount=%.2f (исходная транзакция: %s): %v",
			message.SagaID, sagaData.UserID, amount, transactionID, err)
	} else {
		c.Logger.Printf("SagaID=%s: Возврат средств для UserID=%d выполнен успешно (компенсация транзакции %s)",
			message.SagaID, sagaData.UserID, transactionID)
	}

	sagaData.Status = "billing_compensation_completed"

	updatedData, err := json.Marshal(sagaData)
	if err != nil {
		c.Logger.Printf("[ERROR] SagaID=%s: Ошибка сериализации данных после компенсации: %v", message.SagaID, err)
		return err
	}

	c.Logger.Printf("SagaID=%s: Отправка результата compensate/compensated шага %s", message.SagaID, c.Step)
	return c.PublishCompensationResult(message.SagaID, updatedData)
}

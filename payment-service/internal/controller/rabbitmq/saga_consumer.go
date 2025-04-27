package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/director74/dz8_shop/payment-service/internal/entity"
	"github.com/director74/dz8_shop/payment-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
	"github.com/director74/dz8_shop/pkg/sagahandler"
)

// SagaConsumer обработчик сообщений саги для платежей
type SagaConsumer struct {
	sagahandler.BaseSagaConsumer
	paymentUseCase usecase.PaymentUseCaseInterface
}

// NewSagaConsumer создает новый обработчик сообщений саги для платежей
func NewSagaConsumer(paymentUseCase usecase.PaymentUseCaseInterface, rabbitMQ *rabbitmq.RabbitMQ) *SagaConsumer {
	return &SagaConsumer{
		BaseSagaConsumer: sagahandler.BaseSagaConsumer{
			RabbitMQ: rabbitMQ,
			Logger:   log.New(log.Writer(), "[PaymentService] [Saga] ", log.LstdFlags),
			Step:     "process_payment",
		},
		paymentUseCase: paymentUseCase,
	}
}

// Setup настраивает обработчик событий саги
func (c *SagaConsumer) Setup() error {
	return c.SetupQueues(
		"saga_exchange",
		"payment_process_queue",
		"payment_compensate_queue",
		c.handlePayment,           // handleExecute
		c.handleCompensatePayment, // handleCompensate
	)
}

// handlePayment обрабатывает сообщение для выполнения платежа
func (c *SagaConsumer) handlePayment(data []byte) error {
	c.Logger.Printf("Получено сага-сообщение для оплаты")

	var sagaData sagahandler.SagaData
	if err := json.Unmarshal(data, &sagaData); err != nil {
		return fmt.Errorf("ошибка десериализации данных саги: %w", err)
	}

	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		return err
	}

	c.Logger.Printf("SagaID=%s: Получено сообщение саги для платежа, StepName=%s",
		message.SagaID, message.StepName)

	var sagaDataRabbitmq sagahandler.SagaData
	if err := json.Unmarshal(message.Data, &sagaDataRabbitmq); err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка десериализации данных заказа: %v", message.SagaID, err)
		return c.PublishFailureResult(message.SagaID,
			fmt.Sprintf("ошибка десериализации данных заказа: %v", err))
	}

	if sagaDataRabbitmq.CompensatedSteps == nil {
		sagaDataRabbitmq.CompensatedSteps = make(map[string]bool)
	}

	if sagaDataRabbitmq.CompensatedSteps["process_payment"] {
		c.Logger.Printf("SagaID=%s: Шаг 'process_payment' уже компенсирован, пропускаем обработку", message.SagaID)
		return nil
	}

	if sagaDataRabbitmq.OrderID == 0 {
		c.Logger.Printf("SagaID=%s: [ERROR] Отсутствует OrderID при создании платежа", message.SagaID)
		return c.PublishFailureResult(message.SagaID, "отсутствует OrderID при создании платежа")
	}

	if sagaDataRabbitmq.UserID == 0 {
		c.Logger.Printf("SagaID=%s: [ERROR] Отсутствует UserID при создании платежа", message.SagaID)
		return c.PublishFailureResult(message.SagaID, "отсутствует UserID при создании платежа")
	}

	if sagaDataRabbitmq.Amount <= 0 {
		c.Logger.Printf("SagaID=%s: [ERROR] Некорректная сумма платежа: %.2f", message.SagaID, sagaDataRabbitmq.Amount)
		return c.PublishFailureResult(message.SagaID, fmt.Sprintf("некорректная сумма платежа: %.2f", sagaDataRabbitmq.Amount))
	}

	payment := &entity.CreatePaymentRequest{
		OrderID:     sagaDataRabbitmq.OrderID,
		UserID:      sagaDataRabbitmq.UserID,
		Amount:      sagaDataRabbitmq.Amount,
		PaymentType: "CREDIT_CARD",
	}

	paymentInfo, err := c.paymentUseCase.CreatePayment(context.Background(), payment)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка создания платежа: %v", message.SagaID, err)
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("ошибка обработки платежа: %v", err), message.Data)
	}
	c.Logger.Printf("SagaID=%s: Платеж создан успешно, PaymentID=%d", message.SagaID, paymentInfo.ID)

	if sagaDataRabbitmq.PaymentInfo == nil {
		sagaDataRabbitmq.PaymentInfo = &sagahandler.PaymentInfo{}
	}

	sagaDataRabbitmq.PaymentInfo.PaymentID = fmt.Sprintf("%d", paymentInfo.ID)
	sagaDataRabbitmq.PaymentInfo.Status = string(paymentInfo.Status)
	sagaDataRabbitmq.Status = "payment_processed"

	if sagaDataRabbitmq.PaymentInfo == nil || sagaDataRabbitmq.PaymentInfo.PaymentID == "" {
		c.Logger.Printf("SagaID=%s: [ERROR] КРИТИЧЕСКАЯ ОШИБКА: PaymentID не установлен перед публикацией результата", message.SagaID)
		return c.PublishFailureResult(message.SagaID, "внутренняя ошибка: PaymentID не установлен")
	}

	updatedData, err := json.Marshal(sagaDataRabbitmq)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка сериализации обновленных данных: %v", message.SagaID, err)
		return c.PublishFailureResultWithData(message.SagaID,
			fmt.Sprintf("ошибка сериализации обновленных данных: %v", err), message.Data)
	}
	c.Logger.Printf("SagaID=%s: Успешно обработан шаг платежа, публикуем результат", message.SagaID)
	return c.PublishSuccessResult(message.SagaID, updatedData)
}

// handleCompensatePayment обрабатывает сообщение для возврата платежа
func (c *SagaConsumer) handleCompensatePayment(data []byte) error {
	c.Logger.Printf("Получено сага-сообщение для компенсации оплаты")

	// Парсим основное сообщение
	message, err := sagahandler.ParseSagaMessage(data)
	if err != nil {
		return err
	}
	c.Logger.Printf("SagaID=%s: Получено сообщение саги для компенсации платежа, StepName=%s",
		message.SagaID, message.StepName)

	// Парсим данные из message.Data для получения информации о заказе
	var sagaData sagahandler.SagaData // Используем эту переменную для хранения данных из message.Data
	if err := json.Unmarshal(message.Data, &sagaData); err != nil {
		// Если не удалось распарсить Data, это проблема, но попробуем восстановить OrderID из SagaID
		c.Logger.Printf("SagaID=%s: [WARN] Ошибка десериализации данных (message.Data) при компенсации: %v. Пытаемся восстановить OrderID из SagaID.", message.SagaID, err)
		parts := strings.Split(message.SagaID, "-")
		if len(parts) >= 3 {
			orderID, parseErr := strconv.ParseUint(parts[2], 10, 64)
			if parseErr == nil && orderID > 0 {
				sagaData.OrderID = uint(orderID)
				c.Logger.Printf("SagaID=%s: Восстановлен OrderID=%d из SagaID", message.SagaID, sagaData.OrderID)
			}
		}
		// Если и из SagaID не вышло, sagaData.OrderID останется 0
	} else {
		c.Logger.Printf("SagaID=%s: Успешно извлечены данные из сообщения: OrderID=%d, UserID=%d", message.SagaID, sagaData.OrderID, sagaData.UserID)
	}

	// Проверка OrderID после всех попыток извлечения
	if sagaData.OrderID == 0 {
		c.Logger.Printf("SagaID=%s: [ERROR] Не удалось определить OrderID для компенсации ни из данных сообщения, ни из SagaID.", message.SagaID)
		// Публикуем результат ошибки, т.к. без OrderID мало что можно сделать
		sagaData.CompensatedSteps = map[string]bool{"process_payment": true} // Помечаем как компенсированное с ошибкой
		sagaData.Status = "payment_compensation_error"
		updatedData, _ := json.Marshal(sagaData) // Ошибку маршалинга игнорируем, т.к. уже в процессе обработки ошибки
		return c.PublishCompensationResult(message.SagaID, updatedData)
	}

	// Инициализируем CompensatedSteps, если они nil (может быть nil после парсинга)
	if sagaData.CompensatedSteps == nil {
		sagaData.CompensatedSteps = make(map[string]bool)
	}

	// Проверяем, не был ли шаг уже компенсирован
	if sagaData.CompensatedSteps["process_payment"] {
		c.Logger.Printf("SagaID=%s: Шаг 'process_payment' уже компенсирован, пропускаем компенсацию", message.SagaID)
		return nil // Возвращаем nil, чтобы сообщение подтвердилось и не обрабатывалось повторно
	}

	var paymentID uint
	var paymentFound bool

	if sagaData.PaymentInfo != nil && sagaData.PaymentInfo.PaymentID != "" {
		paymentID = sagahandler.ParseUint(sagaData.PaymentInfo.PaymentID)
		paymentFound = (paymentID > 0)
		if paymentFound {
			c.Logger.Printf("SagaID=%s: Найден PaymentID=%d в данных саги", message.SagaID, paymentID)
		} else {
			c.Logger.Printf("SagaID=%s: [WARN] Найден некорректный PaymentID='%s' в данных саги", message.SagaID, sagaData.PaymentInfo.PaymentID)
		}
	}

	if !paymentFound && sagaData.OrderID > 0 {
		c.Logger.Printf("SagaID=%s: PaymentID не найден/некорректен в данных саги, пытаемся получить по OrderID=%d", message.SagaID, sagaData.OrderID)
		payment, err := c.paymentUseCase.GetPaymentForOrder(sagaData.OrderID)
		if err == nil && payment != nil {
			paymentID = payment.ID
			paymentFound = true
			c.Logger.Printf("SagaID=%s: Найден PaymentID=%d по OrderID=%d", message.SagaID, paymentID, sagaData.OrderID)

			if sagaData.PaymentInfo == nil {
				sagaData.PaymentInfo = &sagahandler.PaymentInfo{}
			}
			sagaData.PaymentInfo.PaymentID = fmt.Sprintf("%d", paymentID)
			sagaData.PaymentInfo.Status = string(payment.Status)
		} else {
			c.Logger.Printf("SagaID=%s: [WARN] Не удалось найти платеж по OrderID=%d: %v", message.SagaID, sagaData.OrderID, err)
		}
	}

	if !paymentFound {
		c.Logger.Printf("SagaID=%s: [ERROR] КРИТИЧЕСКАЯ ОШИБКА: Не удалось найти PaymentID для компенсации платежа (OrderID=%d)", message.SagaID, sagaData.OrderID)

		sagaData.CompensatedSteps["process_payment"] = true

		if sagaData.PaymentInfo == nil {
			sagaData.PaymentInfo = &sagahandler.PaymentInfo{}
		}
		sagaData.PaymentInfo.Status = "COMPENSATION_ERROR"
		sagaData.Status = "payment_compensation_error"

		var messageSagaData sagahandler.SagaData
		if err := json.Unmarshal(message.Data, &messageSagaData); err == nil {
			if sagaData.OrderID == 0 && messageSagaData.OrderID > 0 {
				sagaData.OrderID = messageSagaData.OrderID
				c.Logger.Printf("SagaID=%s: Восстановлен OrderID=%d из исходного сообщения", message.SagaID, sagaData.OrderID)
			}
			if sagaData.UserID == 0 && messageSagaData.UserID > 0 {
				sagaData.UserID = messageSagaData.UserID
				c.Logger.Printf("SagaID=%s: Восстановлен UserID=%d из исходного сообщения", message.SagaID, sagaData.UserID)
			}
		} else {
			c.Logger.Printf("SagaID=%s: [WARN] Не удалось десериализовать исходные данные сообщения для восстановления ID при ошибке компенсации: %v", message.SagaID, err)
		}

		updatedData, err := json.Marshal(sagaData)
		if err != nil {
			c.Logger.Printf("SagaID=%s: [ERROR] Ошибка сериализации обновленных данных при ошибке компенсации: %v", message.SagaID, err)
			return c.PublishCompensationResult(message.SagaID, message.Data)
		}
		c.Logger.Printf("SagaID=%s: Публикуем результат компенсации с ошибкой (PaymentID не найден)", message.SagaID)
		return c.PublishCompensationResult(message.SagaID, updatedData)
	}

	c.Logger.Printf("SagaID=%s: Получено сообщение саги для возврата платежа: StepName=%s, PaymentID=%d",
		message.SagaID, message.StepName, paymentID)

	refundRequest := &entity.RefundPaymentRequest{
		PaymentID: paymentID,
		Amount:    sagaData.Amount,
	}

	err = c.paymentUseCase.RefundPayment(context.Background(), refundRequest)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка возврата платежа PaymentID=%d: %v", message.SagaID, paymentID, err)
		sagaData.CompensatedSteps["process_payment"] = true
		if sagaData.PaymentInfo == nil {
			sagaData.PaymentInfo = &sagahandler.PaymentInfo{}
		}
		sagaData.PaymentInfo.Status = "COMPENSATION_ERROR"
		sagaData.Status = "payment_compensation_error"

		if sagaData.OrderID == 0 || sagaData.UserID == 0 {
			var messageSagaData sagahandler.SagaData
			if err := json.Unmarshal(message.Data, &messageSagaData); err == nil {
				if sagaData.OrderID == 0 && messageSagaData.OrderID > 0 {
					sagaData.OrderID = messageSagaData.OrderID
				}
				if sagaData.UserID == 0 && messageSagaData.UserID > 0 {
					sagaData.UserID = messageSagaData.UserID
				}
			}
		}

		updatedData, marshalErr := json.Marshal(sagaData)
		if marshalErr != nil {
			c.Logger.Printf("SagaID=%s: [ERROR] Ошибка сериализации данных при ошибке возврата платежа: %v", message.SagaID, marshalErr)
			return c.PublishCompensationResult(message.SagaID, message.Data)
		}
		return c.PublishCompensationResult(message.SagaID, updatedData)
	}

	c.Logger.Printf("SagaID=%s: Успешно компенсирован платеж PaymentID=%d", message.SagaID, paymentID)

	sagaData.CompensatedSteps["process_payment"] = true
	if sagaData.PaymentInfo == nil {
		sagaData.PaymentInfo = &sagahandler.PaymentInfo{}
	}
	sagaData.PaymentInfo.Status = "REFUNDED"
	sagaData.Status = "payment_compensated"

	if sagaData.OrderID == 0 || sagaData.UserID == 0 {
		var messageSagaData sagahandler.SagaData
		if err := json.Unmarshal(message.Data, &messageSagaData); err == nil {
			if sagaData.OrderID == 0 && messageSagaData.OrderID > 0 {
				sagaData.OrderID = messageSagaData.OrderID
			}
			if sagaData.UserID == 0 && messageSagaData.UserID > 0 {
				sagaData.UserID = messageSagaData.UserID
			}
		}
	}

	updatedData, err := json.Marshal(sagaData)
	if err != nil {
		c.Logger.Printf("SagaID=%s: [ERROR] Ошибка сериализации данных после успешной компенсации: %v", message.SagaID, err)
		return c.PublishCompensationResult(message.SagaID, message.Data)
	}
	c.Logger.Printf("SagaID=%s: Шаг компенсации платежа завершен, публикуем результат (%s)", message.SagaID, sagaData.Status)
	return c.PublishCompensationResult(message.SagaID, updatedData)
}

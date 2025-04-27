package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/director74/dz8_shop/payment-service/internal/entity"
	"github.com/director74/dz8_shop/payment-service/internal/repo"
	"github.com/director74/dz8_shop/pkg/messaging"
)

// PaymentUseCaseInterface определяет интерфейс для работы с платежами в саге
type PaymentUseCaseInterface interface {
	CreatePayment(ctx context.Context, req *entity.CreatePaymentRequest) (*entity.Payment, error)
	RefundPayment(ctx context.Context, req *entity.RefundPaymentRequest) error
	GetPaymentForOrder(orderID uint) (*entity.Payment, error)
}

// PaymentUseCase реализует бизнес-логику для платежей
type PaymentUseCase struct {
	paymentRepo  repo.PaymentRepository
	publisher    messaging.MessagePublisher
	exchangeName string
}

// NewPaymentUseCase создает новый use case для платежей
func NewPaymentUseCase(paymentRepo repo.PaymentRepository, publisher messaging.MessagePublisher, exchangeName string) *PaymentUseCase {
	return &PaymentUseCase{
		paymentRepo:  paymentRepo,
		publisher:    publisher,
		exchangeName: exchangeName,
	}
}

// CreatePayment создает новый платеж в рамках саги
func (uc *PaymentUseCase) CreatePayment(ctx context.Context, req *entity.CreatePaymentRequest) (*entity.Payment, error) {
	// Создаем новый платеж
	payment := &entity.Payment{
		OrderID:       req.OrderID,
		UserID:        req.UserID,
		Amount:        req.Amount,
		PaymentMethod: req.PaymentType,
		Status:        entity.PaymentStatusPending,
	}

	// Сохраняем платеж
	if err := uc.paymentRepo.CreatePayment(payment); err != nil {
		return nil, fmt.Errorf("ошибка создания платежа: %w", err)
	}

	// Эмулируем процесс платежа (в реальной системе здесь был бы вызов внешнего платежного шлюза)
	success, transactionID := uc.simulatePaymentGateway(payment.Amount)

	var status entity.PaymentStatus
	var paymentErr error // Переменная для хранения ошибки

	if !success {
		status = entity.PaymentStatusFailed
		// Генерируем ошибку, если симуляция не удалась
		paymentErr = errors.New("сбой обработки платежа (симуляция)")
		log.Printf("Сработал неудачный исход при симуляции платежа для OrderID=%d, Amount=%.2f", payment.OrderID, payment.Amount)
	} else {
		status = entity.PaymentStatusCompleted
		paymentErr = nil // Нет ошибки, если успешно
	}

	// Обновляем статус платежа в любом случае (успех или неудача)
	if err := uc.paymentRepo.UpdatePaymentStatus(payment.ID, status, transactionID); err != nil {
		// Ошибка при обновлении статуса - это более серьезная проблема, возвращаем ее
		return nil, fmt.Errorf("ошибка обновления статуса платежа на %s: %w", status, err)
	}

	// Обновляем локальный объект payment с новыми значениями
	payment.Status = status
	payment.TransactionID = transactionID

	// Возвращаем объект платежа и ошибку, если она была
	return payment, paymentErr
}

// RefundPayment выполняет возврат платежа в рамках саги
func (uc *PaymentUseCase) RefundPayment(ctx context.Context, req *entity.RefundPaymentRequest) error {
	payment, err := uc.paymentRepo.GetPaymentByID(req.PaymentID)
	if err != nil {
		return fmt.Errorf("ошибка получения платежа: %w", err)
	}

	if payment == nil {
		return errors.New("платеж не найден")
	}

	// Проверяем, можно ли выполнить возврат
	if payment.Status != entity.PaymentStatusCompleted && payment.Status != entity.PaymentStatusPending {
		return fmt.Errorf("невозможно выполнить возврат для платежа в статусе %s", payment.Status)
	}

	// Обновляем статус платежа на возвращенный
	if err := uc.paymentRepo.UpdatePaymentStatus(payment.ID, entity.PaymentStatusRefunded, payment.TransactionID); err != nil {
		return fmt.Errorf("ошибка обновления статуса платежа: %w", err)
	}

	// Отправляем событие о возврате платежа (опционально)
	uc.publishPaymentRefund(payment)

	return nil
}

// ProcessPayment обрабатывает платеж
func (uc *PaymentUseCase) ProcessPayment(paymentReq *entity.PaymentRequest) (*entity.PaymentConfirmation, error) {
	// Создаем новый платеж
	payment := &entity.Payment{
		OrderID:       paymentReq.OrderID,
		UserID:        paymentReq.UserID,
		Amount:        paymentReq.Amount,
		PaymentMethod: paymentReq.PaymentMethod,
		Status:        entity.PaymentStatusPending,
	}

	// Сохраняем платеж
	if err := uc.paymentRepo.CreatePayment(payment); err != nil {
		return nil, fmt.Errorf("ошибка создания платежа: %w", err)
	}

	// Эмулируем процесс платежа (в реальной системе здесь был бы вызов внешнего платежного шлюза)
	success, transactionID := uc.simulatePaymentGateway(payment.Amount)

	status := entity.PaymentStatusCompleted
	message := "Платеж успешно обработан"

	if !success {
		status = entity.PaymentStatusFailed
		message = "Платеж не прошел"
	}

	// Обновляем статус платежа
	if err := uc.paymentRepo.UpdatePaymentStatus(payment.ID, status, transactionID); err != nil {
		return nil, fmt.Errorf("ошибка обновления статуса платежа: %w", err)
	}

	// Обновляем локальный объект payment с новыми значениями
	payment.Status = status
	payment.TransactionID = transactionID

	// Отправляем событие с результатом платежа
	uc.publishPaymentResult(payment)

	return &entity.PaymentConfirmation{
		PaymentID:     payment.ID,
		OrderID:       payment.OrderID,
		Amount:        payment.Amount,
		Status:        status,
		TransactionID: transactionID,
		Message:       message,
	}, nil
}

// CancelPayment отменяет платеж
func (uc *PaymentUseCase) CancelPayment(paymentID uint) error {
	payment, err := uc.paymentRepo.GetPaymentByID(paymentID)
	if err != nil {
		return fmt.Errorf("ошибка получения платежа при отмене: %w", err)
	}

	if payment == nil {
		return errors.New("платеж не найден")
	}

	// Проверяем, можно ли отменить платеж
	if payment.Status != entity.PaymentStatusPending && payment.Status != entity.PaymentStatusCompleted {
		return fmt.Errorf("невозможно отменить платеж в статусе %s", payment.Status)
	}

	// Обновляем статус платежа
	if err := uc.paymentRepo.UpdatePaymentStatus(paymentID, entity.PaymentStatusCancelled, payment.TransactionID); err != nil {
		// Даже если произошла ошибка обновления статуса, компенсацию нужно продолжить
		log.Printf("Ошибка при обновлении статуса платежа %d на cancelled: %v", paymentID, err)
	}
	payment.Status = entity.PaymentStatusCancelled

	// Отправляем событие об отмене платежа
	uc.publishPaymentCancellation(payment)

	return nil
}

// GetPaymentForOrder возвращает платеж для заказа
func (uc *PaymentUseCase) GetPaymentForOrder(orderID uint) (*entity.Payment, error) {
	payment, err := uc.paymentRepo.GetPaymentByOrderID(orderID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения платежа для заказа: %w", err)
	}
	return payment, nil
}

// GetPaymentByID возвращает платеж по ID
func (uc *PaymentUseCase) GetPaymentByID(paymentID uint) (*entity.Payment, error) {
	payment, err := uc.paymentRepo.GetPaymentByID(paymentID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения платежа: %w", err)
	}
	return payment, nil
}

// GetPaymentsByUserID возвращает все платежи пользователя
func (uc *PaymentUseCase) GetPaymentsByUserID(userID uint) ([]entity.Payment, error) {
	payments, err := uc.paymentRepo.GetPaymentsByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения платежей пользователя: %w", err)
	}
	return payments, nil
}

// simulatePaymentGateway имитирует обработку платежа через платежный шлюз
func (uc *PaymentUseCase) simulatePaymentGateway(amount float64) (bool, string) {
	// Имитация задержки обработки платежа
	time.Sleep(time.Millisecond * 200)

	// Генерация случайного ID транзакции
	transactionID := fmt.Sprintf("TRX-%d", rand.Intn(1000000))

	// 98% платежей успешны (для тестирования), 2% - неуспешны
	success := rand.Float64() < 0.98
	return success, transactionID
}

// PaymentResultMessage Структура сообщения о результате платежа
type PaymentResultMessage struct {
	PaymentID     uint                 `json:"payment_id"`
	OrderID       uint                 `json:"order_id"`
	UserID        uint                 `json:"user_id"`
	Amount        float64              `json:"amount"`
	Status        entity.PaymentStatus `json:"status"`
	TransactionID string               `json:"transaction_id"`
	Timestamp     int64                `json:"timestamp"`
}

// OrderEventMessage структура события от сервиса заказов
type OrderEventMessage struct {
	Type      string          `json:"type"`
	OrderID   uint            `json:"order_id"`
	UserID    uint            `json:"user_id"`
	Amount    float64         `json:"amount"`
	Status    string          `json:"status"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// publishPaymentResult публикует сообщение о результате платежа
func (uc *PaymentUseCase) publishPaymentResult(payment *entity.Payment) {
	message := PaymentResultMessage{
		PaymentID:     payment.ID,
		OrderID:       payment.OrderID,
		UserID:        payment.UserID,
		Amount:        payment.Amount,
		Status:        payment.Status,
		TransactionID: payment.TransactionID,
		Timestamp:     time.Now().Unix(),
	}

	// Определяем роутинг-ключ в зависимости от статуса
	routingKey := "payment.processed"
	if payment.Status == entity.PaymentStatusFailed {
		routingKey = "payment.failed"
	}

	// Публикуем сообщение
	err := messaging.PublishWithRetryAndLogging(uc.publisher, uc.exchangeName, routingKey, message, 3)
	if err != nil {
		log.Printf("Ошибка публикации сообщения о результате платежа: %v", err)
	}
}

// publishPaymentCancellation публикует сообщение об отмене платежа
func (uc *PaymentUseCase) publishPaymentCancellation(payment *entity.Payment) {
	message := PaymentResultMessage{
		PaymentID:     payment.ID,
		OrderID:       payment.OrderID,
		UserID:        payment.UserID,
		Amount:        payment.Amount,
		Status:        payment.Status,
		TransactionID: payment.TransactionID,
		Timestamp:     time.Now().Unix(),
	}

	err := messaging.PublishWithRetryAndLogging(uc.publisher, uc.exchangeName, "payment.cancelled", message, 3)
	if err != nil {
		log.Printf("Ошибка публикации сообщения об отмене платежа: %v", err)
	}
}

// publishPaymentRefund публикует сообщение о возврате платежа
func (uc *PaymentUseCase) publishPaymentRefund(payment *entity.Payment) {
	message := PaymentResultMessage{
		PaymentID:     payment.ID,
		OrderID:       payment.OrderID,
		UserID:        payment.UserID,
		Amount:        payment.Amount,
		Status:        entity.PaymentStatusRefunded,
		TransactionID: payment.TransactionID,
		Timestamp:     time.Now().Unix(),
	}

	err := messaging.PublishWithRetryAndLogging(uc.publisher, uc.exchangeName, "payment.refunded", message, 3)
	if err != nil {
		log.Printf("Ошибка публикации сообщения о возврате платежа: %v", err)
	}
}

// HandleOrderEvent обрабатывает события связанные с заказами
func (uc *PaymentUseCase) HandleOrderEvent(data []byte) error {
	var event OrderEventMessage

	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("ошибка десериализации события: %w", err)
	}

	// Обработка события создания заказа
	if event.Type == "order_created" {
		log.Printf("Получено событие создания заказа: OrderID=%d, UserID=%d, Amount=%.2f",
			event.OrderID, event.UserID, event.Amount)

		// Проверяем, существует ли уже платеж для этого заказа
		existingPayment, err := uc.paymentRepo.GetPaymentByOrderID(event.OrderID)
		if err != nil {
			return fmt.Errorf("ошибка проверки существующего платежа: %w", err)
		}

		if existingPayment != nil {
			log.Printf("Платеж для заказа %d уже существует, пропускаем", event.OrderID)
			return nil
		}

		// Автоматически создаем платеж для нового заказа
		// В реальной системе здесь мог бы быть дополнительный шаг подтверждения от пользователя
		paymentReq := &entity.PaymentRequest{
			OrderID:       event.OrderID,
			UserID:        event.UserID,
			Amount:        event.Amount,
			PaymentMethod: "credit_card", // Значение по умолчанию
		}

		if _, err := uc.ProcessPayment(paymentReq); err != nil {
			log.Printf("Ошибка обработки платежа для заказа %d: %v", event.OrderID, err)
			return err
		}

		log.Printf("Платеж для заказа %d успешно создан", event.OrderID)
		return nil
	}

	// Обработка события отмены заказа
	if event.Type == "order_cancelled" {
		log.Printf("Получено событие отмены заказа: OrderID=%d", event.OrderID)

		// Находим платеж для заказа
		payment, err := uc.paymentRepo.GetPaymentByOrderID(event.OrderID)
		if err != nil {
			return fmt.Errorf("ошибка получения платежа для отмены: %w", err)
		}

		if payment == nil {
			log.Printf("Платеж для заказа %d не найден, пропускаем", event.OrderID)
			return nil
		}

		if err := uc.CancelPayment(payment.ID); err != nil {
			log.Printf("Ошибка отмены платежа %d: %v", payment.ID, err)
			return err
		}

		log.Printf("Платеж %d для заказа %d успешно отменен", payment.ID, event.OrderID)
		return nil
	}

	log.Printf("Пропускаем необрабатываемое событие типа: %s", event.Type)
	return nil
}

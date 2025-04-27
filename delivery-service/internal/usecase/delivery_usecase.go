package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/director74/dz8_shop/delivery-service/internal/entity"
	"github.com/director74/dz8_shop/delivery-service/internal/repo"
	"github.com/director74/dz8_shop/pkg/messaging"
	"github.com/director74/dz8_shop/pkg/sagahandler"
)

// DeliveryUseCase бизнес-логика для работы с доставкой
type DeliveryUseCase struct {
	repo         *repo.DeliveryRepo
	publisher    messaging.MessagePublisher
	exchangeName string
}

// NewDeliveryUseCase создает новый use case для доставки
func NewDeliveryUseCase(repo *repo.DeliveryRepo, publisher messaging.MessagePublisher, exchangeName string) *DeliveryUseCase {
	return &DeliveryUseCase{
		repo:         repo,
		publisher:    publisher,
		exchangeName: exchangeName,
	}
}

// GetDeliveryByID получает информацию о доставке по ID
func (u *DeliveryUseCase) GetDeliveryByID(id uint) (*entity.GetDeliveryResponse, error) {
	delivery, err := u.repo.GetDeliveryByID(id)
	if err != nil {
		return nil, err
	}

	if delivery == nil {
		return nil, nil
	}

	return &entity.GetDeliveryResponse{
		ID:                 delivery.ID,
		OrderID:            delivery.OrderID,
		UserID:             delivery.UserID,
		CourierID:          delivery.CourierID,
		Status:             delivery.Status,
		ScheduledStartTime: delivery.ScheduledStartTime,
		ScheduledEndTime:   delivery.ScheduledEndTime,
		ActualStartTime:    delivery.ActualStartTime,
		ActualEndTime:      delivery.ActualEndTime,
		DeliveryAddress:    delivery.DeliveryAddress,
		RecipientName:      delivery.RecipientName,
		RecipientPhone:     delivery.RecipientPhone,
		TrackingCode:       delivery.TrackingCode,
		CreatedAt:          delivery.CreatedAt,
		UpdatedAt:          delivery.UpdatedAt,
	}, nil
}

// GetDeliveryByOrderID получает информацию о доставке по ID заказа
func (u *DeliveryUseCase) GetDeliveryByOrderID(orderID uint) (*entity.GetDeliveryResponse, error) {
	delivery, err := u.repo.GetDeliveryByOrderID(orderID)
	if err != nil {
		return nil, err
	}

	if delivery == nil {
		return nil, nil
	}

	return &entity.GetDeliveryResponse{
		ID:                 delivery.ID,
		OrderID:            delivery.OrderID,
		UserID:             delivery.UserID,
		CourierID:          delivery.CourierID,
		Status:             delivery.Status,
		ScheduledStartTime: delivery.ScheduledStartTime,
		ScheduledEndTime:   delivery.ScheduledEndTime,
		ActualStartTime:    delivery.ActualStartTime,
		ActualEndTime:      delivery.ActualEndTime,
		DeliveryAddress:    delivery.DeliveryAddress,
		RecipientName:      delivery.RecipientName,
		RecipientPhone:     delivery.RecipientPhone,
		TrackingCode:       delivery.TrackingCode,
		CreatedAt:          delivery.CreatedAt,
		UpdatedAt:          delivery.UpdatedAt,
	}, nil
}

// GetAllDeliveries получает список всех доставок с пагинацией
func (u *DeliveryUseCase) GetAllDeliveries(limit, offset int) (*entity.ListDeliveryResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	deliveries, total, err := u.repo.GetAllDeliveries(limit, offset)
	if err != nil {
		return nil, err
	}

	var response entity.ListDeliveryResponse
	response.Total = int64(total)

	for _, delivery := range deliveries {
		response.Deliveries = append(response.Deliveries, entity.GetDeliveryResponse{
			ID:                 delivery.ID,
			OrderID:            delivery.OrderID,
			UserID:             delivery.UserID,
			CourierID:          delivery.CourierID,
			Status:             delivery.Status,
			ScheduledStartTime: delivery.ScheduledStartTime,
			ScheduledEndTime:   delivery.ScheduledEndTime,
			ActualStartTime:    delivery.ActualStartTime,
			ActualEndTime:      delivery.ActualEndTime,
			DeliveryAddress:    delivery.DeliveryAddress,
			RecipientName:      delivery.RecipientName,
			RecipientPhone:     delivery.RecipientPhone,
			TrackingCode:       delivery.TrackingCode,
			CreatedAt:          delivery.CreatedAt,
			UpdatedAt:          delivery.UpdatedAt,
		})
	}

	return &response, nil
}

// CheckAvailability проверяет доступность временных слотов для доставки
func (u *DeliveryUseCase) CheckAvailability(req *entity.CheckAvailabilityRequest) (*entity.CheckAvailabilityResponse, error) {
	return u.repo.CheckAvailability(req.DeliveryDate, req.ZoneID)
}

// ReserveCourier резервирует курьера для доставки
func (u *DeliveryUseCase) ReserveCourier(ctx context.Context, req *entity.ReserveCourierRequest) (*entity.DeliveryResponse, error) {
	return u.repo.ReserveCourier(ctx, req.OrderID, req.UserID, req.TimeSlotID, req.Address, req.ZoneID)
}

// ReleaseCourier освобождает резервацию курьера
func (u *DeliveryUseCase) ReleaseCourier(ctx context.Context, req *entity.ReleaseCourierRequest) error {
	return u.repo.ReleaseCourier(ctx, req.OrderID)
}

// ConfirmDelivery подтверждает доставку
func (u *DeliveryUseCase) ConfirmDelivery(ctx context.Context, req *entity.ConfirmCourierRequest) error {
	return u.repo.ConfirmDelivery(ctx, req.OrderID)
}

// Методы для интеграции с системой саг

// ReserveForSaga резервирует курьера для заказа в контексте саги
func (u *DeliveryUseCase) ReserveForSaga(ctx context.Context, data interface{}) error {
	reqData, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("неверный формат данных для резервации")
	}

	// Извлекаем данные из контекста саги
	orderIDRaw, ok := reqData["order_id"]
	if !ok {
		return fmt.Errorf("отсутствует ID заказа")
	}
	orderID, ok := parseUint(orderIDRaw)
	if !ok {
		return fmt.Errorf("неверный формат ID заказа")
	}

	userIDRaw, ok := reqData["user_id"]
	if !ok {
		return fmt.Errorf("отсутствует ID пользователя")
	}
	userID, ok := parseUint(userIDRaw)
	if !ok {
		return fmt.Errorf("неверный формат ID пользователя")
	}

	timeSlotIDRaw, ok := reqData["time_slot_id"]
	if !ok {
		return fmt.Errorf("отсутствует ID временного слота")
	}
	timeSlotID, ok := parseUint(timeSlotIDRaw)
	if !ok {
		return fmt.Errorf("неверный формат ID временного слота")
	}

	address, ok := reqData["address"].(string)
	if !ok {
		return fmt.Errorf("отсутствует адрес доставки")
	}

	zoneIDRaw, ok := reqData["zone_id"]
	if !ok {
		return fmt.Errorf("отсутствует ID зоны")
	}
	zoneID, ok := parseUint(zoneIDRaw)
	if !ok {
		return fmt.Errorf("неверный формат ID зоны")
	}

	// Резервируем курьера
	req := &entity.ReserveCourierRequest{
		OrderID:      orderID,
		UserID:       userID,
		TimeSlotID:   timeSlotID,
		Address:      address,
		ZoneID:       zoneID,
		DeliveryDate: time.Now(), // Временно используем текущее время, в реальном приложении должно быть из запроса
	}

	_, err := u.ReserveCourier(ctx, req)
	return err
}

// DeliveryCompletedMessage структура сообщения об успешной доставке
type DeliveryCompletedMessage struct {
	OrderID     uint      `json:"order_id"`
	DeliveryID  uint      `json:"delivery_id"`
	Status      string    `json:"status"`
	CompletedAt time.Time `json:"completed_at"`
}

// ConfirmForSaga подтверждает доставку в контексте саги
func (u *DeliveryUseCase) ConfirmForSaga(ctx context.Context, data interface{}) error {
	reqData, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("неверный формат данных для подтверждения доставки")
	}

	// Извлекаем данные из контекста саги
	orderIDRaw, ok := reqData["order_id"]
	if !ok {
		return fmt.Errorf("отсутствует ID заказа")
	}
	orderID, ok := parseUint(orderIDRaw)
	if !ok {
		return fmt.Errorf("неверный формат ID заказа")
	}

	// Извлекаем sagaID и sagaData
	sagaID, ok := reqData["saga_id"].(string)
	if !ok {
		return fmt.Errorf("отсутствует saga_id")
	}
	sagaData, ok := reqData["saga_data"].(sagahandler.SagaData)
	if !ok {
		// Попытка десериализации, если передали неструктурированные данные
		sagaDataMap, okMap := reqData["saga_data"].(map[string]interface{})
		if okMap {
			sagaDataBytes, _ := json.Marshal(sagaDataMap)
			if err := json.Unmarshal(sagaDataBytes, &sagaData); err != nil {
				return fmt.Errorf("не удалось извлечь или десериализовать saga_data: %v", err)
			}
		} else {
			return fmt.Errorf("неверный формат saga_data")
		}
	}

	// Получаем доставку
	delivery, err := u.repo.GetDeliveryByOrderID(orderID)
	if err != nil {
		return fmt.Errorf("ошибка получения доставки для подтверждения: %w", err)
	}
	if delivery == nil {
		return fmt.Errorf("доставка для заказа %d не найдена", orderID)
	}

	// Проверяем статус, можно ли начать доставку
	if delivery.Status != entity.DeliveryStatusScheduled && delivery.Status != entity.DeliveryStatusConfirmed {
		return fmt.Errorf("доставка для заказа %d не находится в статусе scheduled или confirmed (текущий статус: %s)", orderID, delivery.Status)
	}

	// Обновляем статус на delivering
	delivery.Status = entity.DeliveryStatusDelivering
	now := time.Now()
	delivery.ActualStartTime = &now
	if err := u.repo.UpdateDelivery(delivery); err != nil {
		return fmt.Errorf("ошибка обновления статуса доставки на delivering: %w", err)
	}

	// Запускаем goroutine для имитации завершения доставки, передаем sagaID и sagaData
	go u.simulateDeliveryCompletion(delivery.ID, delivery.OrderID, sagaID, sagaData)

	return nil
}

// simulateDeliveryCompletion имитирует завершение доставки и отправляет сообщение саги
func (u *DeliveryUseCase) simulateDeliveryCompletion(deliveryID uint, orderID uint, sagaID string, sagaData sagahandler.SagaData) {
	deliveryDuration := 10 * time.Second
	fmt.Printf("Имитация доставки для заказа %d (SagaID: %s, DeliveryID: %d) на %s...\n", orderID, sagaID, deliveryID, deliveryDuration)
	time.Sleep(deliveryDuration)

	fmt.Printf("Завершение доставки для заказа %d (SagaID: %s, DeliveryID: %d)...\n", orderID, sagaID, deliveryID)
	// Получаем актуальную информацию о доставке
	delivery, err := u.repo.GetDeliveryByID(deliveryID)
	if err != nil {
		fmt.Printf("[Ошибка] Имитация доставки: не удалось получить доставку %d: %v\n", deliveryID, err)
		return
	}
	if delivery == nil {
		fmt.Printf("[Ошибка] Имитация доставки: доставка %d не найдена после ожидания.\n", deliveryID)
		return
	}

	// Обновляем статус на completed
	delivery.Status = entity.DeliveryStatusCompleted
	now := time.Now()
	delivery.ActualEndTime = &now
	if err := u.repo.UpdateDelivery(delivery); err != nil {
		fmt.Printf("[Ошибка] Имитация доставки (SagaID: %s): не удалось обновить статус доставки %d на completed: %v\\n", sagaID, deliveryID, err)
		// Отправляем сообщение об ошибке в сагу
		u.publishSagaResult(sagaID, "confirm_order", string(sagahandler.StatusFailed), sagaData, fmt.Sprintf("ошибка обновления статуса доставки на completed: %v", err))
		return
	}

	// Обновляем данные саги
	if sagaData.DeliveryInfo == nil {
		sagaData.DeliveryInfo = &sagahandler.DeliveryInfo{}
	}
	sagaData.DeliveryInfo.Status = string(entity.DeliveryStatusCompleted)
	sagaData.DeliveryInfo.DeliveryID = fmt.Sprintf("%d", deliveryID)
	// Можно добавить другие обновленные поля в sagaData, если нужно

	// Публикуем сообщение об успешном завершении шага саги
	u.publishSagaResult(sagaID, "confirm_order", string(sagahandler.StatusCompleted), sagaData, "")

	fmt.Printf("Доставка для заказа %d (SagaID: %s, DeliveryID: %d) успешно завершена и событие саги отправлено.\\n", orderID, sagaID, deliveryID)
}

// publishSagaResult отправляет результат шага саги
func (u *DeliveryUseCase) publishSagaResult(sagaID, stepName, status string, sagaData sagahandler.SagaData, errorMsg string) {
	routingKey := fmt.Sprintf("saga.%s.result", stepName)

	dataBytes, err := json.Marshal(sagaData)
	if err != nil {
		fmt.Printf("[Критическая Ошибка] (SagaID: %s) Ошибка сериализации sagaData для отправки результата шага %s: %v\\n", sagaID, stepName, err)
		// Что делать в этом случае? Паниковать? Логировать?
		// Пока просто логируем
		return
	}

	message := sagahandler.SagaMessage{
		SagaID:    sagaID,
		StepName:  stepName,
		Operation: sagahandler.OperationExecute,   // Всегда execute для результата? Или зависит от статуса?
		Status:    sagahandler.SagaStatus(status), // Преобразуем string к SagaStatus
		Data:      dataBytes,
		Error:     errorMsg,
		Timestamp: time.Now().Unix(),
	}

	// Используем тот же publisher, что и для других сообщений
	err = messaging.PublishWithRetryAndLogging(u.publisher, u.exchangeName, routingKey, message, 3)
	if err != nil {
		fmt.Printf("[Ошибка] (SagaID: %s) Не удалось опубликовать результат (%s) шага %s: %v\\n", sagaID, status, stepName, err)
	}
}

// Вспомогательные функции

// parseUint преобразует интерфейс в uint
func parseUint(value interface{}) (uint, bool) {
	switch v := value.(type) {
	case float64:
		return uint(v), true
	case int:
		return uint(v), true
	case int64:
		return uint(v), true
	case uint:
		return v, true
	case uint64:
		return uint(v), true
	case string:
		var result int
		_, err := fmt.Sscanf(v, "%d", &result)
		if err != nil {
			return 0, false
		}
		return uint(result), true
	default:
		return 0, false
	}
}

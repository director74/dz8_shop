package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/director74/dz8_shop/order-service/internal/entity"
	"github.com/director74/dz8_shop/order-service/internal/repo"
	"github.com/director74/dz8_shop/pkg/sagahandler"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Step описывает шаг саги
type Step struct {
	Name              string
	CompensateOnError bool
}

// SagaData представляет данные для передачи между шагами саги
type SagaData struct {
	OrderID          uint               `json:"order_id"`
	UserID           uint               `json:"user_id"`
	Items            []entity.OrderItem `json:"items"`
	Amount           float64            `json:"amount"`
	Status           entity.OrderStatus `json:"status"`
	DeliveryInfo     *DeliveryInfo      `json:"delivery_info,omitempty"`
	PaymentInfo      *PaymentInfo       `json:"payment_info,omitempty"`
	WarehouseInfo    *WarehouseInfo     `json:"warehouse_info,omitempty"`
	BillingInfo      *BillingInfo       `json:"billing_info,omitempty"`
	Error            string             `json:"error,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	CompensatedSteps map[string]bool    `json:"compensated_steps,omitempty"`
}

// OrderCancellationPayload структура для события отмены/ошибки заказа
// (локальная копия)
type OrderCancellationPayload struct {
	Type    string `json:"type"` // "order.cancelled" или "order.failed"
	OrderID uint   `json:"order_id"`
	UserID  uint   `json:"user_id"`
	Email   string `json:"email"`
	Reason  string `json:"reason"`
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

// PaymentInfo информация о платеже
type PaymentInfo struct {
	PaymentID     string  `json:"payment_id,omitempty"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	TransactionID string  `json:"transaction_id,omitempty"`
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

// ServiceType тип сервиса для шагов саги
type ServiceType string

const (
	ServiceOrder     ServiceType = "order"
	ServicePayment   ServiceType = "payment"
	ServiceBilling   ServiceType = "billing"
	ServiceDelivery  ServiceType = "delivery"
	ServiceWarehouse ServiceType = "warehouse"
)

// SagaMessage представляет сообщение для оркестрации саги
type SagaMessage struct {
	SagaID    string          `json:"saga_id"`
	StepName  string          `json:"step_name"`
	Operation string          `json:"operation"`
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// SagaRabbitMQClient интерфейс для работы с RabbitMQ в контексте саги
type SagaRabbitMQClient interface {
	PublishMessage(exchange, routingKey string, message interface{}) error
}

// SagaStateRepository интерфейс для работы с репозиторием состояний саг
type SagaStateRepository interface {
	Create(ctx context.Context, state *entity.SagaState) error
	GetByID(ctx context.Context, sagaID string) (*entity.SagaState, error)
	Update(ctx context.Context, state *entity.SagaState) error
	Delete(ctx context.Context, sagaID string) error
}

// SagaOrchestrator оркестратор саги для обработки заказа
type SagaOrchestrator struct {
	orderRepo     OrderRepository
	sagaStateRepo SagaStateRepository
	rabbitMQ      SagaRabbitMQClient
	userRepo      repo.UserRepository
	sagaExchange  string
	orderExchange string
	logger        *log.Logger
	sagaSteps     []Step
}

// OrderRepository интерфейс для работы с репозиторием заказов
type OrderRepository interface {
	Create(ctx context.Context, order *entity.Order) error
	GetByID(ctx context.Context, id uint) (*entity.Order, error)
	Update(ctx context.Context, order *entity.Order) error
	UpdateOrderStatus(ctx context.Context, orderID uint, status entity.OrderStatus) error
}

type SagaStep struct {
	Name         string
	Dependencies []string
}

// NewSagaOrchestrator создает новый оркестратор саги
func NewSagaOrchestrator(
	orderRepo OrderRepository,
	sagaStateRepo SagaStateRepository,
	rabbitMQ SagaRabbitMQClient,
	userRepo repo.UserRepository,
	sagaExchange string,
	orderExchange string,
	logger *log.Logger,
) *SagaOrchestrator {
	if logger == nil {
		logger = log.New(log.Writer(), "[SagaOrchestrator] [Saga] ", log.LstdFlags)
	}

	steps := []Step{
		{Name: "create_order", CompensateOnError: false},
		{Name: "process_billing", CompensateOnError: true},
		{Name: "process_payment", CompensateOnError: true},
		{Name: "reserve_warehouse", CompensateOnError: true},
		{Name: "reserve_delivery", CompensateOnError: true},
		{Name: "confirm_order", CompensateOnError: false},
		{Name: "notify_customer", CompensateOnError: false},
	}

	return &SagaOrchestrator{
		orderRepo:     orderRepo,
		sagaStateRepo: sagaStateRepo,
		rabbitMQ:      rabbitMQ,
		userRepo:      userRepo,
		sagaExchange:  sagaExchange,
		orderExchange: orderExchange,
		logger:        logger,
		sagaSteps:     steps,
	}
}

// convertOrderItems преобразует entity.OrderItem в sagahandler.OrderItem
func convertOrderItems(items []entity.OrderItem) []sagahandler.OrderItem {
	result := make([]sagahandler.OrderItem, len(items))
	for i, item := range items {
		result[i] = sagahandler.OrderItem{
			ID:        item.ID,
			OrderID:   item.OrderID,
			ProductID: item.ProductID,
			Name:      item.Name,
			Price:     item.Price,
			Quantity:  item.Quantity,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
	}
	return result
}

// convertToEntityItems преобразует sagahandler.OrderItem в entity.OrderItem
func convertToEntityItems(items []sagahandler.OrderItem) []entity.OrderItem {
	result := make([]entity.OrderItem, len(items))
	for i, item := range items {
		result[i] = entity.OrderItem{
			ID:        item.ID,
			OrderID:   item.OrderID,
			ProductID: item.ProductID,
			Name:      item.Name,
			Price:     item.Price,
			Quantity:  item.Quantity,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		}
	}
	return result
}

// StartOrderSaga начинает сагу для обработки заказа
func (s *SagaOrchestrator) StartOrderSaga(ctx context.Context, orderData *sagahandler.SagaData) error {
	s.logger.Printf("Начата обработка заказа: UserID=%d, Amount=%.2f, Items=%d", orderData.UserID, orderData.Amount, len(orderData.Items))

	order := &entity.Order{
		UserID:    orderData.UserID,
		Amount:    orderData.Amount,
		Items:     convertToEntityItems(orderData.Items),
		Status:    entity.OrderStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return fmt.Errorf("ошибка при создании заказа: %w", err)
	}

	orderData.OrderID = order.ID
	orderData.CreatedAt = order.CreatedAt
	s.logger.Printf("Заказ создан: ID=%d", order.ID)

	for i := range order.Items {
		order.Items[i].OrderID = order.ID
	}
	orderData.Items = convertOrderItems(order.Items)

	sagaID := fmt.Sprintf("saga-order-%d-%d", order.ID, time.Now().UnixNano())

	initialSagaState := &entity.SagaState{
		SagaID:            sagaID,
		OrderID:           order.ID,
		Status:            entity.SagaStatusRunning,
		CompensatedSteps:  make(datatypes.JSONMap),
		TotalToCompensate: 0,
		LastStep:          "",
	}
	if err := s.sagaStateRepo.Create(ctx, initialSagaState); err != nil {
		s.logger.Printf("[ERROR] SagaID=%s: Не удалось создать состояние саги: %v", sagaID, err)
		return fmt.Errorf("ошибка создания состояния саги: %w", err)
	}
	s.logger.Printf("SagaID=%s: Сага запущена для заказа %d, состояние сохранено в БД", sagaID, orderData.OrderID)

	var actualFirstStep *Step
	if len(s.sagaSteps) > 1 {
		actualFirstStep = &s.sagaSteps[1]
	}

	if actualFirstStep != nil {
		initialSagaState.LastStep = actualFirstStep.Name
		if err := s.sagaStateRepo.Update(ctx, initialSagaState); err != nil {
			s.logger.Printf("[WARN] SagaID=%s: Не удалось обновить LastStep при старте: %v", sagaID, err)
		}

		message, err := sagahandler.NewSagaMessage(sagaID, actualFirstStep.Name, sagahandler.OperationExecute, sagahandler.StatusPending, orderData)
		if err != nil {
			return fmt.Errorf("ошибка при создании сообщения саги для шага %s: %w", actualFirstStep.Name, err)
		}
		routingKey := "saga." + actualFirstStep.Name + ".execute"
		err = s.rabbitMQ.PublishMessage(s.sagaExchange, routingKey, message)
		if err != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Ошибка публикации для первого шага %s: %v", sagaID, actualFirstStep.Name, err)
			initialSagaState.Status = entity.SagaStatusFailed
			initialSagaState.ErrorMessage = fmt.Sprintf("Ошибка публикации первого шага %s: %v", actualFirstStep.Name, err)
			if uErr := s.sagaStateRepo.Update(ctx, initialSagaState); uErr != nil {
				s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус саги на Failed после ошибки публикации: %v", sagaID, uErr)
			}
			return err
		}
		s.logger.Printf("SagaID=%s: Стартует первый реальный шаг: %s", sagaID, actualFirstStep.Name)
	} else {
		s.logger.Printf("[WARN] SagaID=%s: Не найден первый реальный шаг для запуска саги", sagaID)
		initialSagaState.Status = entity.SagaStatusFailed
		initialSagaState.ErrorMessage = "Не найден первый реальный шаг для запуска саги"
		if uErr := s.sagaStateRepo.Update(ctx, initialSagaState); uErr != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус саги на Failed (нет шагов): %v", sagaID, uErr)
		}
	}

	s.logger.Printf("SagaID=%s: Сага для заказа %d начата.", sagaID, order.ID)

	return nil
}

// getNextStep возвращает следующий шаг после указанного
func (s *SagaOrchestrator) getNextStep(currentStep string) *Step {
	currentIdx := -1

	for i, step := range s.sagaSteps {
		if step.Name == currentStep {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 || currentIdx >= len(s.sagaSteps)-1 {
		return nil
	}

	return &s.sagaSteps[currentIdx+1]
}

// publishNextStep публикует сообщение для следующего шага саги
func (s *SagaOrchestrator) publishNextStep(sagaID string, currentStep string, sagaData sagahandler.SagaData) error {
	nextStep := s.getNextStep(currentStep)
	if nextStep == nil {
		return nil
	}
	if sagaData.CompensatedSteps == nil {
		sagaData.CompensatedSteps = make(map[string]bool)
	}
	message, err := sagahandler.NewSagaMessage(sagaID, nextStep.Name, sagahandler.OperationExecute, sagahandler.StatusPending, sagaData)
	if err != nil {
		return fmt.Errorf("ошибка сериализации сообщения для шага %s: %w", nextStep.Name, err)
	}
	routingKey := "saga." + nextStep.Name + ".execute"
	if err := s.rabbitMQ.PublishMessage(s.sagaExchange, routingKey, message); err != nil {
		return fmt.Errorf("ошибка публикации сообщения для шага %s: %w", nextStep.Name, err)
	}
	s.logger.Printf("SagaID=%s: Сообщение для следующего шага %s отправлено.", sagaID, nextStep.Name)
	return nil
}

// removeUnusedSagaStates удаляет состояния саг, которые завершились (успешно или с компенсацией)
func (s *SagaOrchestrator) cleanupSagaState(ctx context.Context, sagaID string) {
	err := s.sagaStateRepo.Delete(ctx, sagaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Printf("[WARN] SagaID=%s: Попытка очистить состояние для уже несуществующей саги.", sagaID)
		} else {
			s.logger.Printf("[ERROR] SagaID=%s: Ошибка удаления состояния саги из БД: %v", sagaID, err)
		}
	} else {
		s.logger.Printf("SagaID=%s: Состояние саги успешно удалено из БД.", sagaID)
	}
}

// startCompensationProcess запускает процесс компенсации для шагов, предшествующих failedStep
func (s *SagaOrchestrator) startCompensationProcess(ctx context.Context, sagaID string, failedStep string, sagaData sagahandler.SagaData, compensatedStepsFromCaller map[string]bool) error {
	s.logger.Printf("SagaID=%s: Запуск компенсации для шагов перед %s.", sagaID, failedStep)

	// Находим индекс шага, вызвавшего сбой
	failedStepIndex := -1
	for i, step := range s.sagaSteps {
		if step.Name == failedStep {
			failedStepIndex = i
			break
		}
	}

	if failedStepIndex == -1 {
		s.logger.Printf("[ERROR] SagaID=%s: Шаг %s не найден в конфигурации саги.", sagaID, failedStep)
		return fmt.Errorf("шаг %s не найден в конфигурации саги", failedStep)
	}

	// Определяем шаги для компенсации (только предыдущие и компенсируемые)
	stepsToCompensate := make([]Step, 0)
	for i := failedStepIndex - 1; i >= 0; i-- {
		step := s.sagaSteps[i]
		// Шаг нужно компенсировать, только если он имеет флаг CompensateOnError
		// и он еще не был компенсирован (согласно compensatedStepsFromCaller)
		if step.CompensateOnError {
			stepsToCompensate = append(stepsToCompensate, step)
		}
	}

	// Рассчитываем общее количество шагов, которые *теоретически* требуют компенсации
	totalPotentialCompensatable := len(stepsToCompensate)
	s.logger.Printf("SagaID=%s: Найдено %d предыдущих шагов с флагом CompensateOnError перед %s.", sagaID, totalPotentialCompensatable, failedStep)

	// Получаем текущее состояние саги из репозитория
	state, err := s.sagaStateRepo.GetByID(ctx, sagaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Если сага уже удалена (возможно, завершена и очищена другим процессом), то делать нечего
			s.logger.Printf("[WARN] SagaID=%s: Состояние саги не найдено при запуске компенсации. Возможно, уже очищена.", sagaID)
			return nil
		}
		// Другая ошибка при получении состояния
		s.logger.Printf("[ERROR] SagaID=%s: Ошибка получения состояния саги при запуске компенсации: %v", sagaID, err)
		return fmt.Errorf("ошибка получения состояния саги %s: %w", sagaID, err)
	}

	// Если сага уже в конечном статусе (Compensated или Failed), компенсацию запускать не нужно
	if state.Status == entity.SagaStatusCompensated || state.Status == entity.SagaStatusFailed {
		s.logger.Printf("SagaID=%s: Сага уже в конечном статусе (%s), запуск компенсации не требуется.", sagaID, state.Status)
		return nil
	}

	// Если нет шагов, которые *теоретически* требуют компенсации (totalPotentialCompensatable == 0),
	// то сагу можно считать компенсированной (так как нечего компенсировать).
	if totalPotentialCompensatable == 0 {
		s.logger.Printf("SagaID=%s: Нет предыдущих шагов, требующих компенсации перед %s. Завершаем сагу как Compensated.", sagaID, failedStep)
		state.Status = entity.SagaStatusCompensated
		if uErr := s.sagaStateRepo.Update(ctx, state); uErr != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус саги на Compensated (нет шагов для компенсации): %v", sagaID, uErr)
			// Логируем, но не возвращаем ошибку, чтобы попытаться очистить
		}
		s.cleanupSagaState(ctx, sagaID)
		return nil
	}

	needsStatusUpdate := false
	// Переводим статус в Compensating, если он еще не такой (может быть Running)
	if state.Status != entity.SagaStatusCompensating {
		state.Status = entity.SagaStatusCompensating
		needsStatusUpdate = true
		s.logger.Printf("SagaID=%s: Статус изменен на %s.", sagaID, state.Status)
	}

	// Устанавливаем TotalToCompensate, если он еще не установлен (равен 0).
	// Это значение фиксируется при первом запуске компенсации и не должно меняться при последующих вызовах
	// startCompensationProcess для той же саги (например, при ошибке компенсирующего шага).
	if state.TotalToCompensate == 0 {
		if totalPotentialCompensatable > 0 {
			state.TotalToCompensate = totalPotentialCompensatable
			needsStatusUpdate = true
			s.logger.Printf("SagaID=%s: Установлено TotalToCompensate = %d (инициировано сбоем/компенсацией шага %s).", sagaID, state.TotalToCompensate, failedStep)
		} else {
			// Этот случай уже обработан выше, но для полноты картины
			s.logger.Printf("SagaID=%s: Нет шагов для компенсации, TotalToCompensate остается 0.", sagaID)
		}
	} else {
		// Если TotalToCompensate уже установлен, логируем это. Сравнение с totalPotentialCompensatable может быть полезно для отладки.
		if state.TotalToCompensate != totalPotentialCompensatable {
			s.logger.Printf("SagaID=%s: Установленный TotalToCompensate (%d) отличается от рассчитанного сейчас (%d). Используется установленное значение.", sagaID, state.TotalToCompensate, totalPotentialCompensatable)
		} else {
			s.logger.Printf("SagaID=%s: TotalToCompensate уже установлен: %d.", sagaID, state.TotalToCompensate)
		}
	}

	// Если были изменения в статусе или TotalToCompensate, обновляем запись в БД
	if needsStatusUpdate {
		if uErr := s.sagaStateRepo.Update(ctx, state); uErr != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус/totalToCompensate: %v", sagaID, uErr)
			// Это критическая ошибка, так как состояние саги не актуально
			return fmt.Errorf("не удалось обновить состояние саги %s: %w", sagaID, uErr)
		}
		s.logger.Printf("SagaID=%s: Состояние саги обновлено в БД (Status: %s, TotalToCompensate: %d).", sagaID, state.Status, state.TotalToCompensate)
	}

	// Отправляем сообщения компенсации только для тех шагов из stepsToCompensate,
	// которые еще не были компенсированы (т.е. отсутствуют в compensatedStepsFromCaller)
	stepsForWhichCompensationSent := 0
	for _, step := range stepsToCompensate {
		if _, alreadyCompensated := compensatedStepsFromCaller[step.Name]; !alreadyCompensated {
			// Готовим и отправляем сообщение компенсации для этого шага
			dataCopy := sagaData
			jsonData, err := json.Marshal(dataCopy)
			if err != nil {
				s.logger.Printf("[ERROR] SagaID=%s: Ошибка маршалинга данных для компенсации шага %s: %v", sagaID, step.Name, err)
				continue // Пропускаем этот шаг, но пытаемся компенсировать остальные
			}

			message := sagahandler.SagaMessage{
				SagaID:    sagaID,
				StepName:  step.Name,
				Operation: sagahandler.OperationCompensate,
				Status:    sagahandler.StatusPending,
				Data:      jsonData,
				Timestamp: sagahandler.GetTimestamp(),
			}
			routingKey := fmt.Sprintf("saga.%s.compensate", step.Name)

			if err := s.rabbitMQ.PublishMessage(s.sagaExchange, routingKey, message); err != nil {
				s.logger.Printf("[ERROR] SagaID=%s: Ошибка публикации сообщения компенсации для шага %s (key: %s): %v", sagaID, step.Name, routingKey, err)
				// TODO: Рассмотреть механизм повторных попыток или DLQ. Пока пропускаем.
				continue
			}
			s.logger.Printf("SagaID=%s: Запрос на компенсацию шага %s отправлен (key: %s).", sagaID, step.Name, routingKey)
			stepsForWhichCompensationSent++
		} else {
			s.logger.Printf("SagaID=%s: Шаг %s уже помечен как компенсированный (в данных от вызывающего), пропускаем отправку сообщения компенсации.", sagaID, step.Name)
		}
	}

	currentCompensatedCount := len(compensatedStepsFromCaller)
	// Логируем информацию о проделанной работе
	if stepsForWhichCompensationSent > 0 {
		s.logger.Printf("SagaID=%s: Отправлено %d новых сообщений компенсации. Всего компенсировано на данный момент: %d из %d.", sagaID, stepsForWhichCompensationSent, currentCompensatedCount, state.TotalToCompensate)
	} else {
		// Если новых сообщений не отправлено, проверяем, не завершена ли уже компенсация
		if currentCompensatedCount >= state.TotalToCompensate && state.TotalToCompensate > 0 {
			// Этот блок дублирует проверку ниже, но может быть полезен для логирования
			s.logger.Printf("SagaID=%s: Новых сообщений компенсации не отправлено. Компенсация завершена (скомпенсировано %d из %d).", sagaID, currentCompensatedCount, state.TotalToCompensate)
		} else {
			s.logger.Printf("SagaID=%s: Новых сообщений компенсации не отправлено (уже отправлены или шаги уже скомпенсированы). Ожидаем результаты. Компенсировано: %d из %d.", sagaID, currentCompensatedCount, state.TotalToCompensate)
		}
	}

	// Проверка на завершение всего процесса компенсации
	// Компенсация завершена, если количество фактически компенсированных шагов (currentCompensatedCount)
	// достигло общего числа шагов, требующих компенсации (state.TotalToCompensate),
	// и при этом есть хотя бы один шаг для компенсации (state.TotalToCompensate > 0).
	if currentCompensatedCount >= state.TotalToCompensate && state.TotalToCompensate > 0 {
		s.logger.Printf("SagaID=%s: Все %d необходимых шагов компенсированы (последний инициирующий шаг: %s). Завершение саги как Compensated.", sagaID, state.TotalToCompensate, failedStep)
		state.Status = entity.SagaStatusCompensated
		if uErr := s.sagaStateRepo.Update(ctx, state); uErr != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус саги на Compensated после завершения всех компенсаций: %v", sagaID, uErr)
			// Логируем, но не возвращаем ошибку, чтобы попытаться очистить
		}
		// Очищаем состояние саги после успешной компенсации
		s.cleanupSagaState(ctx, sagaID)
	}

	s.logger.Printf("SagaID=%s: Функция startCompensationProcess завершена.", sagaID)
	return nil
}

// HandleSagaResult обрабатывает результат выполнения шага саги
func (s *SagaOrchestrator) HandleSagaResult(result []byte) error {
	ctx := context.Background()

	var message sagahandler.SagaMessage
	if err := json.Unmarshal(result, &message); err != nil {
		s.logger.Printf("[ERROR] Не удалось десериализовать сообщение саги: %v", err)
		return fmt.Errorf("ошибка при десериализации сообщения саги: %w", err)
	}
	s.logger.Printf("SagaID=%s: Получен результат: Step=%s, Op=%s, Status=%s", message.SagaID, message.StepName, message.Operation, message.Status)

	sagaData, err := sagahandler.ParseSagaData(message)
	if err != nil {
		s.logger.Printf("[WARN] SagaID=%s: Не удалось десериализовать данные (Data) из сообщения: %v. Обработка продолжится без них.", message.SagaID, err)
		sagaData = sagahandler.SagaData{}
	}

	state, err := s.sagaStateRepo.GetByID(ctx, message.SagaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Printf("[WARN] SagaID=%s: Получено сообщение для неизвестной или уже очищенной саги [%s/%s/%s]. Игнорируется.",
				message.SagaID, message.StepName, message.Operation, message.Status)
			return nil
		}
		s.logger.Printf("[ERROR] SagaID=%s: Ошибка получения состояния саги из БД: %v", message.SagaID, err)
		return err
	}

	if state.CompensatedSteps == nil {
		state.CompensatedSteps = make(datatypes.JSONMap)
	}
	deliveryInfoBackup := sagaData.DeliveryInfo

	stateUpdated := false
	compensationCompleted := false

	if message.Operation == sagahandler.OperationCompensate && message.Status == sagahandler.StatusCompensated {
		_, alreadyCompensated := state.CompensatedSteps[message.StepName]

		if alreadyCompensated {
			s.logger.Printf("SagaID=%s: Шаг %s уже был компенсирован, игнорируем повторное сообщение.", message.SagaID, message.StepName)
			return nil
		}

		state.CompensatedSteps[message.StepName] = true
		state.LastStep = message.StepName
		stateUpdated = true
		s.logger.Printf("SagaID=%s: Шаг %s помечен как компенсированный.", message.SagaID, message.StepName)

		if state.TotalToCompensate > 0 && len(state.CompensatedSteps) >= state.TotalToCompensate {
			s.logger.Printf("SagaID=%s: Все %d ожидаемых шагов компенсированы. Завершаем компенсацию саги. Компенсированные шаги: %v", message.SagaID, state.TotalToCompensate, state.CompensatedSteps)
			state.Status = entity.SagaStatusCompensated
			compensationCompleted = true
		} else {
			s.logger.Printf("SagaID=%s: Компенсация продолжается. Шагов компенсировано: %d из %d. Компенсированные шаги: %v", message.SagaID, len(state.CompensatedSteps), state.TotalToCompensate, state.CompensatedSteps)
			state.Status = entity.SagaStatusCompensating
		}

		if uErr := s.sagaStateRepo.Update(ctx, state); uErr != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить состояние саги после компенсации шага %s: %v", message.SagaID, message.StepName, uErr)
			return uErr
		}
		stateUpdated = false

		order, oErr := s.orderRepo.GetByID(ctx, state.OrderID)
		if oErr != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Ошибка получения заказа %d для обновления статуса на Canceled: %v", message.SagaID, state.OrderID, oErr)
		} else if order.Status != entity.OrderStatusCancelled {
			if uoErr := s.orderRepo.UpdateOrderStatus(ctx, order.ID, entity.OrderStatusCancelled); uoErr != nil {
				s.logger.Printf("[ERROR] SagaID=%s: Ошибка обновления статуса заказа %d на Canceled: %v", message.SagaID, state.OrderID, uoErr)
				return uoErr
			}
		}

		if compensationCompleted {
			s.logger.Printf("SagaID=%s: Компенсация завершена. Запуск очистки состояния.", message.SagaID)
			// Отправляем уведомление об отмене перед очисткой
			if order != nil {
				s.publishCancellationEvent(ctx, state.OrderID, order.UserID, "order.cancelled", "Компенсация саги успешно завершена")
			} else {
				// Крайне маловероятно, что order будет nil здесь, но на всякий случай
				s.logger.Printf("[WARN] SagaID=%s: order is nil при отправке уведомления order.cancelled. Используем UserID=0.", message.SagaID)
				s.publishCancellationEvent(ctx, state.OrderID, 0, "order.cancelled", "Компенсация саги успешно завершена")
			}
			s.cleanupSagaState(ctx, message.SagaID)
		}
		return nil
	}

	order, err := s.orderRepo.GetByID(ctx, state.OrderID)
	if err != nil {
		if !(message.Operation == sagahandler.OperationExecute && (message.Status == sagahandler.StatusFailed || message.Status == sagahandler.StatusCompensated)) {
			return fmt.Errorf("ошибка при получении заказа %d для обработки шага %s саги %s: %w", state.OrderID, message.StepName, message.SagaID, err)
		} else {
			s.logger.Printf("[ERROR] SagaID=%s: Ошибка при получении заказа %d перед запуском компенсации (шаг %s саги %s): %v. Компенсация будет запущена.", message.SagaID, state.OrderID, message.StepName, message.SagaID, err)
			order = nil
		}
	}
	if order == nil && message.Operation == sagahandler.OperationExecute && !(message.Status == sagahandler.StatusFailed || message.Status == sagahandler.StatusCompensated) {
		s.logger.Printf("[ERROR] SagaID=%s: Не удалось получить информацию о заказе %d при операции execute.", message.SagaID, state.OrderID)
		return fmt.Errorf("не удалось получить информацию о заказе %d для саги %s при операции execute", state.OrderID, message.SagaID)
	}

	if state.LastStep != message.StepName {
		state.LastStep = message.StepName
		stateUpdated = true
	}

	switch {
	case message.Operation == sagahandler.OperationExecute && message.Status == sagahandler.StatusCompleted:
		// Обработка успешного завершения шага

		// Получаем заказ (нужен в любом случае, кроме ошибки)
		order, err := s.orderRepo.GetByID(ctx, state.OrderID)
		if err != nil {
			// Критическая ошибка, если заказ не найден на этом этапе
			return fmt.Errorf("критическая ошибка: не удалось получить заказ %d при обработке шага %s саги %s: %w", state.OrderID, message.StepName, message.SagaID, err)
		}

		if message.StepName == "notify_customer" {
			// Это был предпоследний шаг, теперь завершаем заказ
			s.logger.Printf("SagaID=%s: Получен успешный результат от notify_customer. Завершение заказа ID=%d.", message.SagaID, order.ID)

			// Обновляем статус заказа на Completed
			if order.Status != entity.OrderStatusCompleted { // Проверяем, чтобы не обновлять повторно
				if err := s.orderRepo.UpdateOrderStatus(ctx, order.ID, entity.OrderStatusCompleted); err != nil {
					s.logger.Printf("[ERROR] SagaID=%s: Ошибка при обновлении статуса заказа %d на Completed: %v", message.SagaID, order.ID, err)
					// Пытаемся обновить статус саги, но возвращаем ошибку обновления заказа
					state.Status = entity.SagaStatusFailed // Ставим Failed, т.к. не смогли обновить заказ
					state.ErrorMessage = fmt.Sprintf("Ошибка обновления статуса заказа на Completed: %v", err)
					if uErr := s.sagaStateRepo.Update(ctx, state); uErr != nil {
						s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус саги на Failed после ошибки обновления заказа: %v", message.SagaID, uErr)
					}
					return err // Возвращаем исходную ошибку
				}
				s.logger.Printf("SagaID=%s: Статус заказа ID=%d успешно обновлен на Completed в БД.", message.SagaID, order.ID)
			} else {
				s.logger.Printf("SagaID=%s: Статус заказа ID=%d уже был Completed.", message.SagaID, order.ID)
			}

			s.logger.Printf("SagaID=%s: Заказ %d успешно завершен.", message.SagaID, order.ID)
			state.Status = entity.SagaStatusCompleted
			state.LastStep = message.StepName // Обновляем LastStep на имя завершенного шага
			if err := s.sagaStateRepo.Update(ctx, state); err != nil {
				s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус саги на Completed: %v", message.SagaID, err)
				// Логируем, но не возвращаем ошибку, т.к. заказ уже обновлен. Пытаемся очистить.
			}
			s.cleanupSagaState(ctx, message.SagaID)
			return nil // Завершаем обработку успешно

		} else if message.StepName == "complete_order" {
			// Этот шаг больше не должен вызываться через сообщение, но оставим лог на всякий случай
			s.logger.Printf("[WARN] SagaID=%s: Получено сообщение для устаревшего шага 'complete_order'. Игнорируется.", message.SagaID)
			// Можно просто проигнорировать или проверить статус заказа/саги и очистить если нужно
			return nil

		} else {
			// Обработка успешного завершения промежуточного шага (не notify_customer)
			s.logger.Printf("SagaID=%s: Успешно завершен промежуточный шаг: %s. Запуск следующего.", message.SagaID, message.StepName)

			// Восстановление DeliveryInfo, если оно пропало (может быть актуально)
			if sagaData.DeliveryInfo == nil && deliveryInfoBackup != nil {
				sagaData.DeliveryInfo = deliveryInfoBackup
			}

			// Публикация сообщения для следующего шага
			if err := s.publishNextStep(message.SagaID, message.StepName, sagaData); err != nil {
				// Ошибка публикации -> Переводим заказ и сагу в Failed
				order.Status = entity.OrderStatusFailed
				if uErr := s.orderRepo.UpdateOrderStatus(ctx, order.ID, entity.OrderStatusFailed); uErr != nil {
					s.logger.Printf("[ERROR] SagaID=%s: Ошибка обновления заказа %d на Failed после ошибки публикации: %v", message.SagaID, order.ID, uErr)
				}
				state.Status = entity.SagaStatusFailed
				state.ErrorMessage = fmt.Sprintf("Ошибка публикации следующего шага после %s: %v", message.StepName, err)
				if uErr := s.sagaStateRepo.Update(ctx, state); uErr != nil {
					s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус саги на Failed после ошибки публикации: %v", message.SagaID, uErr)
				}
				return err // Возвращаем ошибку публикации
			}

			// Если публикация успешна:
			// Статус заказа остается Pending
			if order.Status != entity.OrderStatusPending {
				if err := s.orderRepo.UpdateOrderStatus(ctx, order.ID, entity.OrderStatusPending); err != nil {
					// Ошибка обновления статуса заказа на Pending -> возвращаем ошибку
					return fmt.Errorf("ошибка при обновлении заказа %d на Pending: %w", order.ID, err)
				}
			}
			// Статус саги остается Running
			state.Status = entity.SagaStatusRunning
			state.LastStep = message.StepName // Обновляем LastStep на имя завершенного шага
			stateUpdated = true               // Помечаем, что нужно обновить состояние саги в БД (в конце функции)
		}

	case (message.Operation == sagahandler.OperationExecute && (message.Status == sagahandler.StatusFailed || message.Status == sagahandler.StatusCompensated)) ||
		(message.Operation == sagahandler.OperationCompensate && message.Status == sagahandler.StatusFailed):

		logPrefix := fmt.Sprintf("[%s/%s]", message.Operation, message.Status)
		s.logger.Printf("%s SagaID=%s: Получен статус, требующий компенсации для шага %s. Запуск компенсации. Ошибка: %s", logPrefix, message.SagaID, message.StepName, message.Error)

		if order != nil {
			order.Status = entity.OrderStatusFailed
			if err := s.orderRepo.UpdateOrderStatus(ctx, order.ID, entity.OrderStatusFailed); err != nil {
				s.logger.Printf("[ERROR] SagaID=%s: Ошибка при обновлении статуса заказа %d на Failed: %v", message.SagaID, order.ID, err)
			}
		}
		state.Status = entity.SagaStatusCompensating
		if message.Error != "" {
			state.ErrorMessage = message.Error
		} else {
			state.ErrorMessage = fmt.Sprintf("Компенсация инициирована из-за статуса %s/%s шага %s", message.Operation, message.Status, message.StepName)
		}
		stateUpdated = true

		// Update the state *before* starting compensation to persist the error message and Compensating status.
		if uErr := s.sagaStateRepo.Update(ctx, state); uErr != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Не удалось обновить статус саги на Compensating перед запуском компенсации: %v", message.SagaID, uErr)
			return uErr // Return early, as state is inconsistent
		}
		stateUpdated = false // Reset flag as state is now persisted

		// Отправляем уведомление об ошибке при инициации компенсации
		userID := sagaData.UserID // Используем UserID из sagaData, т.к. order может быть nil
		if order != nil {
			userID = order.UserID
		}
		s.publishCancellationEvent(ctx, state.OrderID, userID, "order.failed", state.ErrorMessage)

		// Запускаем процесс компенсации (если нужно)
		if state.Status == entity.SagaStatusCompensating {
			stepsToPass := convertJSONMapToBoolMap(state.CompensatedSteps)
			if err := s.startCompensationProcess(ctx, message.SagaID, message.StepName, sagaData, stepsToPass); err != nil {
				s.logger.Printf("[ERROR] SagaID=%s: Ошибка запуска компенсации после сбоя шага %s: %v", message.SagaID, message.StepName, err)
				// Не возвращаем ошибку, компенсация будет продолжена или зависнет
			}
		}
		return nil // Return nil because the error/compensation is being handled asynchronously

	default:
		s.logger.Printf("[WARN] SagaID=%s: Неизвестная или необработанная комбинация операции/статуса: %s/%s для шага %s",
			message.SagaID, message.Operation, message.Status, message.StepName)
		state.Status = entity.SagaStatusFailed
		state.ErrorMessage = fmt.Sprintf("Необработанная комбинация: %s/%s", message.Operation, message.Status)
		stateUpdated = true

		// Отправляем уведомление об ошибке
		userID := sagaData.UserID // Используем UserID из sagaData, т.к. order может быть nil
		if order != nil {
			userID = order.UserID
		}
		s.publishCancellationEvent(ctx, state.OrderID, userID, "order.failed", state.ErrorMessage)
	}

	if stateUpdated {
		if err := s.sagaStateRepo.Update(ctx, state); err != nil {
			s.logger.Printf("[ERROR] SagaID=%s: Не удалось сохранить финальное обновление состояния: %v", message.SagaID, err)
			return err
		}
	}

	return nil
}

// copyMap создает неглубокую копию map[string]bool (достаточно для этого случая)
func convertJSONMapToBoolMap(original datatypes.JSONMap) map[string]bool {
	if original == nil {
		return make(map[string]bool)
	}
	copied := make(map[string]bool, len(original))
	for key, value := range original {
		if boolVal, ok := value.(bool); ok {
			copied[key] = boolVal
		}
	}
	return copied
}

// SetupOrderSagaConsumer создает обработчик сообщений саги для обновления заказа
func (s *SagaOrchestrator) SetupOrderSagaConsumer() error {
	rmq, ok := s.rabbitMQ.(interface {
		DeclareExchange(name string, kind string) error
		DeclareQueue(name string) error
		BindQueue(queueName, exchangeName, routingKey string) error
		ConsumeMessages(queueName, consumerName string, handler func([]byte) error) error
	})

	if !ok {
		return fmt.Errorf("предоставленный SagaRabbitMQClient не поддерживает необходимые методы")
	}

	if err := rmq.DeclareExchange(s.sagaExchange, "topic"); err != nil {
		return fmt.Errorf("ошибка при создании обмена '%s': %w", s.sagaExchange, err)
	}

	queueName := "order_service.saga_results"
	if err := rmq.DeclareQueue(queueName); err != nil {
		return fmt.Errorf("ошибка при создании очереди '%s': %w", queueName, err)
	}

	routingKey := "saga.*.result"
	if err := rmq.BindQueue(queueName, s.sagaExchange, routingKey); err != nil {
		return fmt.Errorf("ошибка при привязке очереди '%s' к обмену '%s' с ключом '%s': %w", queueName, s.sagaExchange, routingKey, err)
	}

	consumerTag := "order_saga_result_consumer"
	if err := rmq.ConsumeMessages(queueName, consumerTag, s.HandleSagaResult); err != nil {
		return fmt.Errorf("ошибка при настройке получения сообщений из очереди '%s': %w", queueName, err)
	}

	s.logger.Printf("[INFO] Обработчик результатов саги ('%s') успешно настроен.", queueName)
	return nil
}

// sagaLogger адаптер для логгера саги
type sagaLogger struct {
	logger *log.Logger
}

func (l *sagaLogger) Info(msg string, args ...interface{}) {
	l.logger.Printf("[INFO] "+msg, args...)
}

func (l *sagaLogger) Error(msg string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+msg, args...)
}

// isFirstCompensatableStep проверяет, является ли данный шаг первым компенсируемым шагом в саге
func (s *SagaOrchestrator) isFirstCompensatableStep(stepName string) bool {
	for _, step := range s.sagaSteps {
		if step.CompensateOnError {
			return step.Name == stepName
		}
	}
	return false
}

// getFirstCompensatableStepName возвращает имя первого компенсируемого шага (для логов)
func (s *SagaOrchestrator) getFirstCompensatableStepName() string {
	for _, step := range s.sagaSteps {
		if step.CompensateOnError {
			return step.Name
		}
	}
	return "<не найдено>"
}

// publishCancellationEvent отправляет событие отмены/ошибки заказа
func (s *SagaOrchestrator) publishCancellationEvent(ctx context.Context, orderID uint, userID uint, eventType string, reason string) {
	// Получаем email пользователя
	user, err := s.userRepo.GetByID(ctx, userID)
	userEmail := ""
	if err != nil {
		s.logger.Printf("[WARN] SagaID=saga-order-%d: Не удалось получить пользователя %d для отправки уведомления об отмене/ошибке: %v", orderID, userID, err)
		// Продолжаем без email, notification-service использует заглушку
	} else {
		userEmail = user.Email
	}

	// Создаем payload
	payload := OrderCancellationPayload{
		Type:    eventType, // "order.cancelled" или "order.failed"
		OrderID: orderID,
		UserID:  userID,
		Email:   userEmail,
		Reason:  reason,
	}

	// Публикуем в exchange заказов (например, order_events), а не в saga_events
	if err := s.rabbitMQ.PublishMessage(s.orderExchange, eventType, payload); err != nil {
		s.logger.Printf("[ERROR] SagaID=saga-order-%d: Ошибка отправки уведомления %s: %v", orderID, eventType, err)
	}
}

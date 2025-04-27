package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/director74/dz8_shop/order-service/internal/entity"
	"github.com/director74/dz8_shop/order-service/internal/repo"
	"github.com/director74/dz8_shop/pkg/sagahandler"
)

// OrderUseCase представляет usecase для работы с заказами
type OrderUseCase struct {
	repo      repo.OrderRepository
	userRepo  repo.UserRepository
	billing   BillingService
	rabbitMQ  RabbitMQClient
	orderExch string
	sagaExch  string
	sagaOrch  *SagaOrchestrator
	logger    *log.Logger
}

// OrderNotificationPayload структура для отправки уведомления о заказе
// (локальная копия, чтобы избежать прямой зависимости от notification-service)
type OrderNotificationPayload struct {
	UserID  uint    `json:"user_id"`
	Email   string  `json:"email"`
	OrderID uint    `json:"order_id"`
	Amount  float64 `json:"amount"`
	Success bool    `json:"success"` // Добавляем поле Success
}

func NewOrderUseCase(
	orderRepo repo.OrderRepository,
	userRepo repo.UserRepository,
	sagaStateRepo SagaStateRepository,
	billing BillingService,
	rabbitMQ RabbitMQClient,
	orderExch string,
	sagaExch string,
) *OrderUseCase {
	logger := log.New(log.Writer(), "[OrderUseCase] ", log.LstdFlags)

	uc := &OrderUseCase{
		repo:      orderRepo,
		userRepo:  userRepo,
		billing:   billing,
		rabbitMQ:  rabbitMQ,
		orderExch: orderExch,
		sagaExch:  sagaExch,
		logger:    logger,
	}

	// Создаем оркестратор саги, передавая sagaStateRepo и userRepo
	uc.sagaOrch = NewSagaOrchestrator(orderRepo, sagaStateRepo, rabbitMQ, userRepo, sagaExch, uc.orderExch, logger)

	// Настраиваем обработчик событий саги
	go func() {
		if err := uc.sagaOrch.SetupOrderSagaConsumer(); err != nil {
			logger.Printf("Ошибка при настройке обработчика саги: %v", err)
		}
	}()

	return uc
}

func (uc *OrderUseCase) CreateUser(ctx context.Context, req entity.CreateUserRequest) (entity.CreateUserResponse, error) {
	_, err := uc.userRepo.GetByEmail(ctx, req.Email)
	if err == nil {
		return entity.CreateUserResponse{}, errors.New("пользователь с таким email уже существует")
	}

	user := &entity.User{
		Username:  req.Username,
		Email:     req.Email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = uc.userRepo.Create(ctx, user)
	if err != nil {
		return entity.CreateUserResponse{}, fmt.Errorf("ошибка при создании пользователя: %w", err)
	}

	err = uc.billing.CreateAccount(ctx, user.ID)
	if err != nil {
		// При ошибке создания аккаунта в биллинге удаляем пользователя
		deleteErr := uc.userRepo.Delete(ctx, user.ID)
		if deleteErr != nil {
			// Логируем ошибку удаления, но возвращаем основную ошибку
			fmt.Printf("Ошибка при удалении пользователя после неудачного создания аккаунта в биллинге: %v\n", deleteErr)
		}
		return entity.CreateUserResponse{}, fmt.Errorf("ошибка при создании аккаунта в биллинге: %w", err)
	}

	return entity.CreateUserResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}, nil
}

func (uc *OrderUseCase) CreateOrder(ctx context.Context, req entity.CreateOrderRequest) (entity.CreateOrderResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	user, err := uc.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return entity.CreateOrderResponse{}, fmt.Errorf("пользователь не найден: %w", err)
	}

	// Если сумма заказа не указана, вычисляем её автоматически
	if req.Amount == 0 {
		var totalAmount float64
		for _, item := range req.Items {
			totalAmount += item.Price * float64(item.Quantity)
		}
		req.Amount = totalAmount
	}

	// Подготавливаем данные для саги
	sagaData := SagaData{
		UserID:    req.UserID,
		Items:     req.Items,
		Amount:    req.Amount,
		Status:    entity.OrderStatusPending,
		CreatedAt: time.Now(),
	}

	// Кратко логируем создание заказа
	uc.logger.Printf("[Order] Создание заказа: UserID=%d, Amount=%.2f, Items=%d", req.UserID, req.Amount, len(req.Items))

	// Если в запросе есть информация о доставке, добавляем ее
	if req.Delivery != nil {
		sagaData.DeliveryInfo = &DeliveryInfo{
			Address:      req.Delivery.Address,
			TimeSlotID:   parseUintOrZero(req.Delivery.TimeSlotID),
			ZoneID:       parseUintOrZero(req.Delivery.ZoneID),
			Cost:         0, // дефолт, если нужно — можно вычислять
			Status:       "pending",
			DeliveryDate: time.Now().Format("2006-01-02"), // дефолт — сегодня
		}
	}

	// Конвертируем в формат для SagaOrchestrator
	sagaPkgData := sagahandler.SagaData{
		UserID:    sagaData.UserID,
		Items:     make([]sagahandler.OrderItem, len(sagaData.Items)),
		Amount:    sagaData.Amount,
		Status:    string(sagaData.Status),
		CreatedAt: sagaData.CreatedAt,
	}
	for i, item := range sagaData.Items {
		sagaPkgData.Items[i] = sagahandler.OrderItem{
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
	if sagaData.DeliveryInfo != nil {
		sagaPkgData.DeliveryInfo = &sagahandler.DeliveryInfo{
			Address:      sagaData.DeliveryInfo.Address,
			TimeSlotID:   sagaData.DeliveryInfo.TimeSlotID,
			ZoneID:       sagaData.DeliveryInfo.ZoneID,
			Cost:         sagaData.DeliveryInfo.Cost,
			Status:       sagaData.DeliveryInfo.Status,
			DeliveryDate: sagaData.DeliveryInfo.DeliveryDate,
		}
	}

	// Запускаем сагу
	err = uc.sagaOrch.StartOrderSaga(ctx, &sagaPkgData)
	if err != nil {
		uc.logger.Printf("[Order][ERROR] Ошибка запуска саги: %v", err)
		return entity.CreateOrderResponse{}, fmt.Errorf("ошибка при запуске процесса обработки заказа: %w", err)
	}

	// Получаем ID заказа из саги после создания
	orderID := sagaPkgData.OrderID
	uc.logger.Printf("[Order] Создан заказ ID=%d", orderID)

	// Обновляем ID для всех позиций заказа
	for i := range req.Items {
		req.Items[i].OrderID = orderID
	}

	// Отправляем нотификацию о начале обработки заказа, используем локальную структуру OrderNotificationPayload
	notification := OrderNotificationPayload{ // Используем локальный тип
		UserID:  user.ID,
		Email:   user.Email, // Убедиться, что user.Email действительно содержит email
		OrderID: orderID,
		Amount:  req.Amount,
		Success: true, // Явно указываем успех
	}

	if err = uc.rabbitMQ.PublishMessageWithRetry(uc.orderExch, "order.notification", notification, 3); err != nil {
		uc.logger.Printf("[Order][ERROR] Ошибка отправки нотификации о новом заказе: %v", err)
	}

	return entity.CreateOrderResponse{
		ID:        orderID,
		UserID:    req.UserID,
		Items:     req.Items,
		Amount:    req.Amount,
		Status:    entity.OrderStatusPending,
		CreatedAt: sagaData.CreatedAt,
	}, nil
}

func (uc *OrderUseCase) GetOrder(ctx context.Context, id uint) (entity.GetOrderResponse, error) {
	order, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return entity.GetOrderResponse{}, fmt.Errorf("заказ не найден: %w", err)
	}

	return entity.GetOrderResponse{
		ID:        order.ID,
		UserID:    order.UserID,
		Amount:    order.Amount,
		Status:    order.Status,
		CreatedAt: order.CreatedAt,
		UpdatedAt: order.UpdatedAt,
	}, nil
}

func (uc *OrderUseCase) ListUserOrders(ctx context.Context, userID uint, limit, offset int) (entity.ListOrdersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	orders, err := uc.repo.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		return entity.ListOrdersResponse{}, fmt.Errorf("ошибка при получении списка заказов: %w", err)
	}

	total, err := uc.repo.CountByUserID(ctx, userID)
	if err != nil {
		return entity.ListOrdersResponse{}, fmt.Errorf("ошибка при получении общего количества заказов: %w", err)
	}

	var response entity.ListOrdersResponse
	response.Total = total
	response.Orders = make([]entity.GetOrderResponse, len(orders))

	for i, order := range orders {
		response.Orders[i] = entity.GetOrderResponse{
			ID:        order.ID,
			UserID:    order.UserID,
			Amount:    order.Amount,
			Status:    order.Status,
			CreatedAt: order.CreatedAt,
			UpdatedAt: order.UpdatedAt,
		}
	}

	return response, nil
}

// parseUintOrZero — утилита для преобразования string в uint
func parseUintOrZero(s string) uint {
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return uint(u)
}

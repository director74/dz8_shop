package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/director74/dz8_shop/notification-service/internal/entity"
	"github.com/director74/dz8_shop/pkg/sagahandler"
)

// NotificationRepository интерфейс для работы с хранилищем нотификаций
type NotificationRepository interface {
	CreateNotification(ctx context.Context, notification entity.Notification) (entity.Notification, error)
	GetNotificationByID(ctx context.Context, id uint) (entity.Notification, error)
	UpdateNotificationStatus(ctx context.Context, id uint, status string) error
	ListNotificationsByUserID(ctx context.Context, userID uint, limit, offset int) ([]entity.Notification, int64, error)
	ListAllNotifications(ctx context.Context, limit, offset int) ([]entity.Notification, int64, error)
}

// OrderCancellationPayload структура для события отмены/ошибки заказа
// (локальная копия из order-service для избежания прямой зависимости)
type OrderCancellationPayload struct {
	Type    string `json:"type"`
	OrderID uint   `json:"order_id"`
	UserID  uint   `json:"user_id"`
	Email   string `json:"email"`
	Reason  string `json:"reason"`
}

// EmailSender интерфейс для отправки электронной почты
type EmailSender interface {
	SendEmail(to, subject, message string) error
}

// NotificationUseCase представляет usecase для работы с нотификациями
type NotificationUseCase struct {
	repo        NotificationRepository
	emailSender EmailSender
}

func NewNotificationUseCase(repo NotificationRepository, emailSender EmailSender) *NotificationUseCase {
	return &NotificationUseCase{
		repo:        repo,
		emailSender: emailSender,
	}
}

// SendNotification создает запись об уведомлении в БД и "отправляет" его (меняет статус)
func (uc *NotificationUseCase) SendNotification(ctx context.Context, req entity.SendNotificationRequest) (entity.SendNotificationResponse, error) {
	notification := entity.Notification{
		UserID:    req.UserID,
		Email:     req.Email,
		Subject:   req.Subject,
		Message:   req.Message,
		Status:    entity.NotificationStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	newNotification, err := uc.repo.CreateNotification(ctx, notification)
	if err != nil {
		return entity.SendNotificationResponse{}, fmt.Errorf("ошибка при создании уведомления: %w", err)
	}

	// TODO: Реализовать реальную отправку почты через EmailSender
	// err = uc.emailSender.SendEmail(req.Email, req.Subject, req.Message)
	// if err != nil {
	//     _ = uc.repo.UpdateNotificationStatus(ctx, newNotification.ID, entity.NotificationStatusFailed)
	//     return entity.SendNotificationResponse{}, fmt.Errorf("ошибка при отправке уведомления: %w", err)
	// }

	err = uc.repo.UpdateNotificationStatus(ctx, newNotification.ID, entity.NotificationStatusSent)
	if err != nil {
		// Если не удалось обновить статус, все равно возвращаем успех создания,
		// но логируем ошибку обновления статуса.
		log.Printf("[ERROR] Не удалось обновить статус уведомления ID %d на Sent: %v", newNotification.ID, err)
		// Не возвращаем ошибку здесь, чтобы не повлиять на вызывающий код, который может ожидать ID
	}

	return entity.SendNotificationResponse{
		ID:      newNotification.ID,
		UserID:  newNotification.UserID,
		Email:   newNotification.Email,
		Subject: newNotification.Subject,
		Status:  entity.NotificationStatusSent, // Возвращаем Sent, даже если обновление не удалось
	}, nil
}

// ProcessOrderNotification обрабатывает событие создания/ошибки заказа
func (uc *NotificationUseCase) ProcessOrderNotification(ctx context.Context, orderNotification entity.OrderNotification) error {
	var subject, message string

	if orderNotification.Success {
		subject = fmt.Sprintf("Заказ #%d успешно оформлен", orderNotification.OrderID)
		message = fmt.Sprintf("Уважаемый клиент, ваш заказ #%d на сумму %.2f успешно оформлен. Спасибо за покупку!",
			orderNotification.OrderID, orderNotification.Amount)
	} else {
		// Этот блок кода может быть неактуален, т.к. ошибки обрабатываются через ProcessOrderCancellation
		subject = fmt.Sprintf("Проблема с заказом #%d", orderNotification.OrderID)
		message = fmt.Sprintf("Уважаемый клиент, при оформлении заказа #%d на сумму %.2f возникла проблема.",
			orderNotification.OrderID, orderNotification.Amount)
	}

	req := entity.SendNotificationRequest{
		UserID:  orderNotification.UserID,
		Email:   orderNotification.Email,
		Subject: subject,
		Message: message,
	}

	_, err := uc.SendNotification(ctx, req)
	return err
}

// ProcessDepositNotification обрабатывает событие пополнения баланса
func (uc *NotificationUseCase) ProcessDepositNotification(ctx context.Context, depositNotification entity.DepositNotification) error {
	email := depositNotification.Email
	if email == "" {
		email = fmt.Sprintf("user%d@example.com", depositNotification.UserID)
	}

	subject := "Пополнение баланса"
	message := fmt.Sprintf("Уважаемый клиент, ваш счет был пополнен на сумму %.2f. Текущая операция: %s.",
		depositNotification.Amount, depositNotification.OperationType)

	req := entity.SendNotificationRequest{
		UserID:  depositNotification.UserID,
		Email:   email,
		Subject: subject,
		Message: message,
	}

	_, err := uc.SendNotification(ctx, req)
	return err
}

// ProcessInsufficientFundsNotification обрабатывает событие недостатка средств
func (uc *NotificationUseCase) ProcessInsufficientFundsNotification(ctx context.Context, notification entity.InsufficientFundsNotification) error {
	email := notification.Email
	if email == "" {
		email = fmt.Sprintf("user%d@example.com", notification.UserID)
	}

	subject := "Недостаточно средств на вашем счете"
	message := fmt.Sprintf("Уважаемый клиент, на вашем счете недостаточно средств для совершения операции на сумму %.2f. "+
		"Текущий баланс: %.2f. Пожалуйста, пополните баланс для совершения покупок.",
		notification.Amount, notification.Balance)

	req := entity.SendNotificationRequest{
		UserID:  notification.UserID,
		Email:   email,
		Subject: subject,
		Message: message,
	}

	_, err := uc.SendNotification(ctx, req)
	return err
}

func (uc *NotificationUseCase) GetNotification(ctx context.Context, id uint) (entity.GetNotificationResponse, error) {
	notification, err := uc.repo.GetNotificationByID(ctx, id)
	if err != nil {
		return entity.GetNotificationResponse{}, fmt.Errorf("уведомление не найдено: %w", err)
	}

	return entity.GetNotificationResponse{
		ID:        notification.ID,
		UserID:    notification.UserID,
		Email:     notification.Email,
		Subject:   notification.Subject,
		Message:   notification.Message,
		Status:    notification.Status,
		CreatedAt: notification.CreatedAt,
	}, nil
}

func (uc *NotificationUseCase) ListUserNotifications(ctx context.Context, userID uint, limit, offset int) (entity.ListNotificationsResponse, error) {
	notifications, total, err := uc.repo.ListNotificationsByUserID(ctx, userID, limit, offset)
	if err != nil {
		return entity.ListNotificationsResponse{}, fmt.Errorf("ошибка при получении списка уведомлений: %w", err)
	}

	var response entity.ListNotificationsResponse
	response.Total = total
	response.Notifications = make([]entity.GetNotificationResponse, len(notifications))

	for i, notification := range notifications {
		response.Notifications[i] = entity.GetNotificationResponse{
			ID:        notification.ID,
			UserID:    notification.UserID,
			Email:     notification.Email,
			Subject:   notification.Subject,
			Message:   notification.Message,
			Status:    notification.Status,
			CreatedAt: notification.CreatedAt,
		}
	}

	return response, nil
}

func (uc *NotificationUseCase) ListAllNotifications(ctx context.Context, limit, offset int) (entity.ListNotificationsResponse, error) {
	notifications, total, err := uc.repo.ListAllNotifications(ctx, limit, offset)
	if err != nil {
		return entity.ListNotificationsResponse{}, fmt.Errorf("ошибка при получении списка уведомлений: %w", err)
	}

	var response entity.ListNotificationsResponse
	response.Total = total
	response.Notifications = make([]entity.GetNotificationResponse, len(notifications))

	for i, notification := range notifications {
		response.Notifications[i] = entity.GetNotificationResponse{
			ID:        notification.ID,
			UserID:    notification.UserID,
			Email:     notification.Email,
			Subject:   notification.Subject,
			Message:   notification.Message,
			Status:    notification.Status,
			CreatedAt: notification.CreatedAt,
		}
	}

	return response, nil
}

// ProcessOrderCancellation обрабатывает событие отмены/ошибки заказа (order.cancelled/order.failed)
func (uc *NotificationUseCase) ProcessOrderCancellation(ctx context.Context, event OrderCancellationPayload) error {
	log.Printf("Обработка события %s для заказа %d", event.Type, event.OrderID)

	var subject, message string
	email := event.Email
	if email == "" {
		email = fmt.Sprintf("user%d@example.com", event.UserID)
		log.Printf("[WARN] Email для UserID %d не найден в событии %s, используется заглушка %s", event.UserID, event.Type, email)
	}

	if event.Type == "order.cancelled" {
		subject = fmt.Sprintf("Заказ #%d отменен", event.OrderID)
		message = fmt.Sprintf("Уважаемый клиент, ваш заказ #%d был отменен. Причина: %s.", event.OrderID, event.Reason)
	} else if event.Type == "order.failed" {
		subject = fmt.Sprintf("Проблема с заказом #%d", event.OrderID)
		message = fmt.Sprintf("Уважаемый клиент, при обработке вашего заказа #%d возникла ошибка. Причина: %s.", event.OrderID, event.Reason)
	} else {
		// Обработка неизвестного типа (маловероятно, но для полноты)
		log.Printf("[WARN] Получен неизвестный тип события в ProcessOrderCancellation: %s", event.Type)
		subject = fmt.Sprintf("Проблема с заказом #%d", event.OrderID)
		message = fmt.Sprintf("Уважаемый клиент, при обработке вашего заказа #%d возникла проблема. Причина: %s.", event.OrderID, event.Reason)
	}

	req := entity.SendNotificationRequest{
		UserID:  event.UserID,
		Email:   email,
		Subject: subject,
		Message: message,
	}

	_, err := uc.SendNotification(ctx, req)
	if err != nil {
		return fmt.Errorf("ошибка отправки уведомления для события %s заказа %d: %w", event.Type, event.OrderID, err)
	}

	log.Printf("Уведомление для события %s заказа %d успешно создано/отправлено.", event.Type, event.OrderID)
	return nil
}

// SendSagaNotification обрабатывает уведомление в рамках шага саги (notify_customer)
func (uc *NotificationUseCase) SendSagaNotification(ctx context.Context, sagaData sagahandler.SagaData) error {
	var subject, message string
	var email string

	// TODO: Убедиться, что order-service добавляет email пользователя в sagaData при запуске шага notify_customer
	if email == "" {
		// Пока используем заглушку
		email = fmt.Sprintf("user%d@example.com", sagaData.UserID)
		log.Printf("[WARN] Email для UserID %d не найден в sagaData, используется заглушка %s", sagaData.UserID, email)
	}

	// Простая версия уведомления: просто об успешном прохождении этапа
	subject = fmt.Sprintf("Обновление по заказу #%d", sagaData.OrderID)
	message = fmt.Sprintf("Заказ #%d успешно прошел этап обработки.", sagaData.OrderID)

	// Можно добавить логику для разных статусов, если они будут передаваться в sagaData
	// if sagaData.Status == "completed" { ... } else if sagaData.Error != "" { ... }

	req := entity.SendNotificationRequest{
		UserID:  sagaData.UserID,
		Email:   email,
		Subject: subject,
		Message: message,
	}

	_, err := uc.SendNotification(ctx, req)
	return err
}

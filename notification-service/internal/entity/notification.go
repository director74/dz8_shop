package entity

import (
	"time"
)

// Notification содержит данные об отправленных пользователю уведомлениях
type Notification struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	Email     string    `json:"email"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Возможные статусы уведомлений
const (
	NotificationStatusSent    = "sent"
	NotificationStatusPending = "pending"
	NotificationStatusFailed  = "failed"
)

type SendNotificationRequest struct {
	UserID  uint   `json:"user_id" binding:"required"`
	Email   string `json:"email" binding:"required,email"`
	Subject string `json:"subject" binding:"required"`
	Message string `json:"message" binding:"required"`
}

type SendNotificationResponse struct {
	ID      uint   `json:"id"`
	UserID  uint   `json:"user_id"`
	Email   string `json:"email"`
	Subject string `json:"subject"`
	Status  string `json:"status"`
}

type GetNotificationResponse struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	Email     string    `json:"email"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ListNotificationsResponse struct {
	Notifications []GetNotificationResponse `json:"notifications"`
	Total         int64                     `json:"total"`
}

// OrderNotification событие для уведомления о заказе (транспортная модель)
type OrderNotification struct {
	UserID  uint    `json:"user_id"`
	Email   string  `json:"email"`
	OrderID uint    `json:"order_id"`
	Amount  float64 `json:"amount"`
	Success bool    `json:"success"`
}

// DepositNotification событие для уведомления о пополнении баланса (транспортная модель)
type DepositNotification struct {
	Type          string  `json:"type"`
	UserID        uint    `json:"user_id"`
	TransactionID uint    `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	OperationType string  `json:"operation_type"`
	Status        string  `json:"status"`
	Email         string  `json:"email"`
}

// InsufficientFundsNotification событие для уведомления о недостатке средств (транспортная модель)
type InsufficientFundsNotification struct {
	UserID        uint    `json:"user_id"`
	TransactionID uint    `json:"transaction_id"`
	Amount        float64 `json:"amount"`
	Type          string  `json:"type"`
	Status        string  `json:"status"`
	Balance       float64 `json:"balance"`
	Reason        string  `json:"reason"`
	Email         string  `json:"email"`
}

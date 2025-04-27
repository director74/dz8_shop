package entity

import (
	"time"
)

// PaymentStatus статус платежа
type PaymentStatus string

// Константы для статусов платежа
const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
	PaymentStatusCancelled PaymentStatus = "cancelled"
)

// PaymentMethodType тип метода платежа
type PaymentMethodType string

// Константы для типов платежных методов
const (
	PaymentMethodCreditCard   PaymentMethodType = "credit_card"
	PaymentMethodDebitCard    PaymentMethodType = "debit_card"
	PaymentMethodBankTransfer PaymentMethodType = "bank_transfer"
	PaymentMethodWallet       PaymentMethodType = "wallet"
)

// Payment представляет платеж
type Payment struct {
	ID            uint          `json:"id" gorm:"primaryKey"`
	OrderID       uint          `json:"order_id" gorm:"not null"`
	UserID        uint          `json:"user_id" gorm:"not null;index"`
	Amount        float64       `json:"amount" gorm:"type:decimal(12,2);not null"`
	PaymentMethod string        `json:"payment_method" gorm:"not null"`
	Status        PaymentStatus `json:"status" gorm:"not null;default:pending"`
	TransactionID string        `json:"transaction_id"`
	CreatedAt     time.Time     `json:"created_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time     `json:"updated_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt     *time.Time    `json:"deleted_at" gorm:"index"`
}

// PaymentMethod представляет метод платежа пользователя
type PaymentMethod struct {
	ID        uint              `json:"id" gorm:"primaryKey"`
	UserID    uint              `json:"user_id" gorm:"not null;index"`
	Type      PaymentMethodType `json:"type" gorm:"not null"`
	Details   string            `json:"details" gorm:"type:jsonb;not null"`
	IsDefault bool              `json:"is_default" gorm:"not null;default:false"`
	CreatedAt time.Time         `json:"created_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time         `json:"updated_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time        `json:"deleted_at" gorm:"index"`
}

// PaymentRequest модель запроса для создания платежа
type PaymentRequest struct {
	OrderID       uint    `json:"order_id" binding:"required"`
	UserID        uint    `json:"user_id" binding:"required"`
	Amount        float64 `json:"amount" binding:"required,gt=0"`
	PaymentMethod string  `json:"payment_method" binding:"required"`
}

// CreatePaymentRequest модель запроса для создания платежа через сагу
type CreatePaymentRequest struct {
	OrderID     uint    `json:"order_id"`
	UserID      uint    `json:"user_id"`
	Amount      float64 `json:"amount"`
	PaymentType string  `json:"payment_type"`
}

// RefundPaymentRequest модель запроса для возврата платежа
type RefundPaymentRequest struct {
	PaymentID uint    `json:"payment_id"`
	Amount    float64 `json:"amount"`
}

// PaymentConfirmation модель ответа при подтверждении платежа
type PaymentConfirmation struct {
	PaymentID     uint          `json:"payment_id"`
	OrderID       uint          `json:"order_id"`
	Amount        float64       `json:"amount"`
	Status        PaymentStatus `json:"status"`
	TransactionID string        `json:"transaction_id,omitempty"`
	Message       string        `json:"message,omitempty"`
}

// GetPaymentResponse модель ответа при запросе платежа
type GetPaymentResponse struct {
	ID            uint          `json:"id"`
	OrderID       uint          `json:"order_id"`
	UserID        uint          `json:"user_id"`
	Amount        float64       `json:"amount"`
	PaymentMethod string        `json:"payment_method"`
	Status        PaymentStatus `json:"status"`
	TransactionID string        `json:"transaction_id,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// ListPaymentsResponse модель ответа при запросе списка платежей
type ListPaymentsResponse struct {
	Payments []GetPaymentResponse `json:"payments"`
	Total    int64                `json:"total"`
}

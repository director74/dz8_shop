package entity

import (
	"time"
)

// Account хранит информацию о финансовом аккаунте пользователя и его балансе
type Account struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	UserID    uint       `json:"user_id" gorm:"column:user_id;type:integer;not null"`
	Balance   float64    `json:"balance" gorm:"type:decimal(12,2);not null;default:0"`
	CreatedAt time.Time  `json:"created_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `json:"deleted_at" gorm:"index"`
}

// Transaction содержит запись о движении средств с типами deposit или withdrawal
type Transaction struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	AccountID uint       `json:"account_id" gorm:"index:idx_transactions_account_id"`
	Amount    float64    `json:"amount" gorm:"type:decimal(12,2);not null"`
	Type      string     `json:"type" gorm:"index:idx_transactions_type;type:varchar(20);not null"`     // deposit, withdrawal
	Status    string     `json:"status" gorm:"index:idx_transactions_status;type:varchar(20);not null"` // success, failed
	CreatedAt time.Time  `json:"created_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"not null;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `json:"deleted_at" gorm:"index"`
}

// Типы транзакций
const (
	TransactionTypeDeposit    = "deposit"
	TransactionTypeWithdrawal = "withdrawal"
)

// Статусы транзакций
const (
	TransactionStatusSuccess = "success"
	TransactionStatusFailed  = "failed"
)

type CreateAccountRequest struct {
	UserID uint `json:"user_id" binding:"required"`
}

type CreateAccountResponse struct {
	ID      uint    `json:"id"`
	UserID  uint    `json:"user_id"`
	Balance float64 `json:"balance"`
}

type GetAccountResponse struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	Balance   float64   `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
}

type DepositRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
	Email  string  `json:"email" binding:"omitempty,email"`
}

type WithdrawRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
	Email  string  `json:"email" binding:"omitempty,email"`
}

type TransactionResponse struct {
	ID        uint      `json:"id"`
	AccountID uint      `json:"account_id"`
	Amount    float64   `json:"amount"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type WithdrawResponse struct {
	Transaction TransactionResponse `json:"transaction"`
	Success     bool                `json:"success"`
}

type DepositResponse struct {
	Transaction TransactionResponse `json:"transaction"`
	Success     bool                `json:"success"`
}

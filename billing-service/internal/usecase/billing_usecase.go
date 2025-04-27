package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/director74/dz8_shop/billing-service/internal/entity"
)

// BillingRepository интерфейс для работы с хранилищем биллинга
type BillingRepository interface {
	CreateAccount(ctx context.Context, account entity.Account) (entity.Account, error)
	GetAccountByUserID(ctx context.Context, userID uint) (entity.Account, error)
	UpdateBalance(ctx context.Context, accountID uint, amount float64) error
	CreateTransaction(ctx context.Context, transaction entity.Transaction) (entity.Transaction, error)
	GetTransactionByID(ctx context.Context, id uint) (entity.Transaction, error)
	ListTransactionsByAccountID(ctx context.Context, accountID uint, limit, offset int) ([]entity.Transaction, int64, error)
	WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error
}

// RabbitMQClient интерфейс для работы с RabbitMQ
type RabbitMQClient interface {
	PublishMessage(exchange, routingKey string, message interface{}) error
	PublishMessageWithRetry(exchange, routingKey string, message interface{}, retries int) error
}

// BillingUseCase представляет usecase для работы с биллингом
type BillingUseCase struct {
	repo        BillingRepository
	rabbitMQ    RabbitMQClient
	billingExch string
}

// NewBillingUseCase создает новый usecase для работы с биллингом
func NewBillingUseCase(repo BillingRepository, rabbitMQ RabbitMQClient, billingExch string) *BillingUseCase {
	return &BillingUseCase{
		repo:        repo,
		rabbitMQ:    rabbitMQ,
		billingExch: billingExch,
	}
}

func (uc *BillingUseCase) CreateAccount(ctx context.Context, req entity.CreateAccountRequest) (entity.CreateAccountResponse, error) {
	_, err := uc.repo.GetAccountByUserID(ctx, req.UserID)
	if err == nil {
		return entity.CreateAccountResponse{}, errors.New("аккаунт для данного пользователя уже существует")
	}

	account := entity.Account{
		UserID:    req.UserID,
		Balance:   0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	newAccount, err := uc.repo.CreateAccount(ctx, account)
	if err != nil {
		return entity.CreateAccountResponse{}, fmt.Errorf("ошибка при создании аккаунта: %w", err)
	}

	return entity.CreateAccountResponse{
		ID:      newAccount.ID,
		UserID:  newAccount.UserID,
		Balance: newAccount.Balance,
	}, nil
}

func (uc *BillingUseCase) GetAccount(ctx context.Context, userID uint) (entity.GetAccountResponse, error) {
	account, err := uc.repo.GetAccountByUserID(ctx, userID)
	if err != nil {
		return entity.GetAccountResponse{}, fmt.Errorf("аккаунт не найден: %w", err)
	}

	return entity.GetAccountResponse{
		ID:        account.ID,
		UserID:    account.UserID,
		Balance:   account.Balance,
		CreatedAt: account.CreatedAt,
	}, nil
}

// Deposit пополняет баланс аккаунта
func (uc *BillingUseCase) Deposit(ctx context.Context, userID uint, amount float64, email string) (entity.DepositResponse, error) {
	account, err := uc.repo.GetAccountByUserID(ctx, userID)
	if err != nil {
		return entity.DepositResponse{}, fmt.Errorf("аккаунт не найден: %w", err)
	}

	transaction := entity.Transaction{
		AccountID: account.ID,
		Amount:    amount,
		Type:      entity.TransactionTypeDeposit,
		Status:    entity.TransactionStatusSuccess,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	var newTransaction entity.Transaction

	err = uc.repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		// Обновляем баланс
		if err := uc.repo.UpdateBalance(ctx, account.ID, amount); err != nil {
			return fmt.Errorf("ошибка при обновлении баланса: %w", err)
		}

		var txErr error
		newTransaction, txErr = uc.repo.CreateTransaction(ctx, transaction)
		if txErr != nil {
			return fmt.Errorf("ошибка при создании транзакции: %w", txErr)
		}

		return nil
	})

	if err != nil {
		return entity.DepositResponse{}, err
	}

	// Отправляем событие в RabbitMQ для нотификации с повторными попытками, если RabbitMQ инициализирован
	if uc.rabbitMQ != nil {
		// Определяем email для уведомления
		if email == "" {
			email = "user" + fmt.Sprintf("%d", account.UserID) + "@example.com"
		}

		messageWithType := struct {
			Type          string  `json:"type"`
			UserID        uint    `json:"user_id"`
			TransactionID uint    `json:"transaction_id"`
			Amount        float64 `json:"amount"`
			OperationType string  `json:"operation_type"`
			Status        string  `json:"status"`
			Email         string  `json:"email"`
		}{
			Type:          "billing.deposit",
			UserID:        account.UserID,
			TransactionID: newTransaction.ID,
			Amount:        amount,
			OperationType: entity.TransactionTypeDeposit,
			Status:        entity.TransactionStatusSuccess,
			Email:         email,
		}

		// Используем метод с повторными попытками для надежной публикации
		err = uc.rabbitMQ.PublishMessageWithRetry(uc.billingExch, "billing.deposit", messageWithType, 3)
		if err != nil {
			// Логируем ошибку, но не прерываем выполнение
			log.Printf("Ошибка при отправке нотификации о пополнении баланса после %d попыток: %v\n", 3, err)
		} else {
			// Логируем успешную отправку
			log.Printf("Успешно отправлено уведомление о пополнении баланса для пользователя %d на email %s\n",
				account.UserID, email)
		}
	}

	return entity.DepositResponse{
		Transaction: entity.TransactionResponse{
			ID:        newTransaction.ID,
			AccountID: newTransaction.AccountID,
			Amount:    newTransaction.Amount,
			Type:      newTransaction.Type,
			Status:    newTransaction.Status,
			CreatedAt: newTransaction.CreatedAt,
		},
		Success: true,
	}, nil
}

// Withdraw снимает деньги с аккаунта
func (uc *BillingUseCase) Withdraw(ctx context.Context, userID uint, amount float64, email string) (entity.WithdrawResponse, error) {
	account, err := uc.repo.GetAccountByUserID(ctx, userID)
	if err != nil {
		return entity.WithdrawResponse{}, fmt.Errorf("аккаунт не найден: %w", err)
	}

	if account.Balance < amount {
		transaction := entity.Transaction{
			AccountID: account.ID,
			Amount:    amount,
			Type:      entity.TransactionTypeWithdrawal,
			Status:    entity.TransactionStatusFailed,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		newTransaction, err := uc.repo.CreateTransaction(ctx, transaction)
		if err != nil {
			return entity.WithdrawResponse{}, fmt.Errorf("ошибка при создании транзакции: %w", err)
		}

		// Отправляем событие в RabbitMQ при недостатке средств
		if uc.rabbitMQ != nil {
			notification := struct {
				Type          string  `json:"type"`
				UserID        uint    `json:"user_id"`
				TransactionID uint    `json:"transaction_id"`
				Amount        float64 `json:"amount"`
				OperationType string  `json:"operation_type"`
				Status        string  `json:"status"`
				Balance       float64 `json:"balance"`
				Reason        string  `json:"reason"`
				Email         string  `json:"email"`
			}{
				Type:          "billing.insufficient_funds",
				UserID:        account.UserID,
				TransactionID: newTransaction.ID,
				Amount:        amount,
				OperationType: entity.TransactionTypeWithdrawal,
				Status:        entity.TransactionStatusFailed,
				Balance:       account.Balance,
				Reason:        "insufficient_funds",
				Email:         email,
			}

			// Используем метод с повторными попытками для надежной публикации
			err = uc.rabbitMQ.PublishMessageWithRetry(uc.billingExch, "billing.insufficient_funds", notification, 3)
			if err != nil {
				// Логируем ошибку, но не прерываем выполнение
				log.Printf("Ошибка при отправке нотификации о недостатке средств после %d попыток: %v\n", 3, err)
			}
		}

		return entity.WithdrawResponse{
			Transaction: entity.TransactionResponse{
				ID:        newTransaction.ID,
				AccountID: newTransaction.AccountID,
				Amount:    newTransaction.Amount,
				Type:      newTransaction.Type,
				Status:    newTransaction.Status,
				CreatedAt: newTransaction.CreatedAt,
			},
			Success: false,
		}, nil
	}

	transaction := entity.Transaction{
		AccountID: account.ID,
		Amount:    -amount, // Отрицательная сумма для снятия
		Type:      entity.TransactionTypeWithdrawal,
		Status:    entity.TransactionStatusSuccess,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	var newTransaction entity.Transaction

	err = uc.repo.WithTransaction(ctx, func(tx *gorm.DB) error {
		// Обновляем баланс
		if err := uc.repo.UpdateBalance(ctx, account.ID, -amount); err != nil {
			return fmt.Errorf("ошибка при обновлении баланса: %w", err)
		}

		var txErr error
		newTransaction, txErr = uc.repo.CreateTransaction(ctx, transaction)
		if txErr != nil {
			return fmt.Errorf("ошибка при создании транзакции: %w", txErr)
		}

		return nil
	})

	if err != nil {
		return entity.WithdrawResponse{}, err
	}

	return entity.WithdrawResponse{
		Transaction: entity.TransactionResponse{
			ID:        newTransaction.ID,
			AccountID: newTransaction.AccountID,
			Amount:    amount, // Возвращаем положительную сумму для ясности
			Type:      newTransaction.Type,
			Status:    newTransaction.Status,
			CreatedAt: newTransaction.CreatedAt,
		},
		Success: true,
	}, nil
}

// HandleOrderCreatedEvent обрабатывает событие создания заказа
func (uc *BillingUseCase) HandleOrderCreatedEvent(data []byte) error {
	// Структура для десериализации сообщения
	var message struct {
		OrderID   uint    `json:"order_id"`
		UserID    uint    `json:"user_id"`
		TotalCost float64 `json:"total_cost"`
		Email     string  `json:"email"`
	}

	// Десериализуем сообщение
	err := json.Unmarshal(data, &message)
	if err != nil {
		return fmt.Errorf("ошибка при разборе сообщения о создании заказа: %w", err)
	}

	log.Printf("Получено событие создания заказа: OrderID=%d, UserID=%d, TotalCost=%.2f",
		message.OrderID, message.UserID, message.TotalCost)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Выполняем списание средств
	resp, err := uc.Withdraw(ctx, message.UserID, message.TotalCost, message.Email)
	if err != nil {
		log.Printf("Ошибка при списании средств для заказа %d: %v", message.OrderID, err)
		return err
	}

	// Результат операции (статус транзакции)
	transactionSuccess := resp.Success

	// Отправляем событие о результате обработки платежа
	paymentEvent := struct {
		OrderID       uint    `json:"order_id"`
		UserID        uint    `json:"user_id"`
		TransactionID uint    `json:"transaction_id"`
		Amount        float64 `json:"amount"`
		Status        string  `json:"status"`
		Success       bool    `json:"success"`
	}{
		OrderID:       message.OrderID,
		UserID:        message.UserID,
		TransactionID: resp.Transaction.ID,
		Amount:        message.TotalCost,
		Status:        resp.Transaction.Status,
		Success:       transactionSuccess,
	}

	// Публикуем событие результата обработки
	err = uc.rabbitMQ.PublishMessageWithRetry(uc.billingExch, "billing.payment_processed", paymentEvent, 3)
	if err != nil {
		log.Printf("Ошибка при отправке события обработки платежа: %v", err)
		return err
	}

	log.Printf("Платеж для заказа %d обработан, результат: %v", message.OrderID, transactionSuccess)
	return nil
}

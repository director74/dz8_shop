package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/director74/dz8_shop/billing-service/internal/entity"
)

// BillingRepository представляет репозиторий для работы с биллингом
type BillingRepository struct {
	db *gorm.DB
}

func NewBillingRepository(db *gorm.DB) *BillingRepository {
	return &BillingRepository{
		db: db,
	}
}

func (r *BillingRepository) CreateAccount(ctx context.Context, account entity.Account) (entity.Account, error) {
	err := r.db.WithContext(ctx).Create(&account).Error
	return account, err
}

func (r *BillingRepository) GetAccountByUserID(ctx context.Context, userID uint) (entity.Account, error) {
	var account entity.Account
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&account).Error
	return account, err
}

// UpdateBalance обновляет баланс аккаунта
func (r *BillingRepository) UpdateBalance(ctx context.Context, accountID uint, amount float64) error {
	return r.db.WithContext(ctx).Model(&entity.Account{}).Where("id = ?", accountID).
		Update("balance", gorm.Expr("balance + ?", amount)).Error
}

func (r *BillingRepository) CreateTransaction(ctx context.Context, transaction entity.Transaction) (entity.Transaction, error) {
	err := r.db.WithContext(ctx).Create(&transaction).Error
	return transaction, err
}

func (r *BillingRepository) GetTransactionByID(ctx context.Context, id uint) (entity.Transaction, error) {
	var transaction entity.Transaction
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&transaction).Error
	return transaction, err
}

func (r *BillingRepository) ListTransactionsByAccountID(ctx context.Context, accountID uint, limit, offset int) ([]entity.Transaction, int64, error) {
	var transactions []entity.Transaction
	var total int64

	r.db.WithContext(ctx).Model(&entity.Transaction{}).Where("account_id = ?", accountID).Count(&total)
	err := r.db.WithContext(ctx).Where("account_id = ?", accountID).Limit(limit).Offset(offset).Order("created_at DESC").Find(&transactions).Error

	return transactions, total, err
}

// WithTransaction выполняет функцию в транзакции базы данных
func (r *BillingRepository) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(tx)
	})
}

package repo

import (
	"errors"

	"github.com/director74/dz8_shop/payment-service/internal/entity"
	"gorm.io/gorm"
)

// PaymentRepository интерфейс для работы с платежами
type PaymentRepository interface {
	CreatePayment(payment *entity.Payment) error
	GetPaymentByID(id uint) (*entity.Payment, error)
	GetPaymentByOrderID(orderID uint) (*entity.Payment, error)
	UpdatePaymentStatus(id uint, status entity.PaymentStatus, transactionID string) error
	GetPaymentsByUserID(userID uint) ([]entity.Payment, error)

	CreatePaymentMethod(method *entity.PaymentMethod) error
	GetPaymentMethodsByUserID(userID uint) ([]entity.PaymentMethod, error)
	GetDefaultPaymentMethod(userID uint) (*entity.PaymentMethod, error)
	SetDefaultPaymentMethod(id uint, userID uint) error
}

// PaymentRepo реализация репозитория платежей
type PaymentRepo struct {
	db *gorm.DB
}

// NewPaymentRepository создает новый репозиторий платежей
func NewPaymentRepository(db *gorm.DB) *PaymentRepo {
	return &PaymentRepo{db: db}
}

// CreatePayment создает новый платеж
func (r *PaymentRepo) CreatePayment(payment *entity.Payment) error {
	return r.db.Create(payment).Error
}

// GetPaymentByID возвращает платеж по ID
func (r *PaymentRepo) GetPaymentByID(id uint) (*entity.Payment, error) {
	var payment entity.Payment
	err := r.db.First(&payment, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &payment, nil
}

// GetPaymentByOrderID возвращает платеж по ID заказа
func (r *PaymentRepo) GetPaymentByOrderID(orderID uint) (*entity.Payment, error) {
	var payment entity.Payment
	err := r.db.Where("order_id = ?", orderID).First(&payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &payment, nil
}

// UpdatePaymentStatus обновляет статус платежа
func (r *PaymentRepo) UpdatePaymentStatus(id uint, status entity.PaymentStatus, transactionID string) error {
	result := r.db.Model(&entity.Payment{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         status,
			"transaction_id": transactionID,
		})
	return result.Error
}

// GetPaymentsByUserID возвращает все платежи пользователя
func (r *PaymentRepo) GetPaymentsByUserID(userID uint) ([]entity.Payment, error) {
	var payments []entity.Payment
	err := r.db.Where("user_id = ?", userID).Find(&payments).Error
	return payments, err
}

// CreatePaymentMethod создает новый метод платежа
func (r *PaymentRepo) CreatePaymentMethod(method *entity.PaymentMethod) error {
	return r.db.Create(method).Error
}

// GetPaymentMethodsByUserID возвращает все методы платежа пользователя
func (r *PaymentRepo) GetPaymentMethodsByUserID(userID uint) ([]entity.PaymentMethod, error) {
	var methods []entity.PaymentMethod
	err := r.db.Where("user_id = ?", userID).Find(&methods).Error
	return methods, err
}

// GetDefaultPaymentMethod возвращает метод платежа по умолчанию
func (r *PaymentRepo) GetDefaultPaymentMethod(userID uint) (*entity.PaymentMethod, error) {
	var method entity.PaymentMethod
	err := r.db.Where("user_id = ? AND is_default = ?", userID, true).First(&method).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &method, nil
}

// SetDefaultPaymentMethod устанавливает метод платежа по умолчанию
func (r *PaymentRepo) SetDefaultPaymentMethod(id uint, userID uint) error {
	// Сначала снимаем флаг is_default у всех методов пользователя
	err := r.db.Model(&entity.PaymentMethod{}).
		Where("user_id = ?", userID).
		Update("is_default", false).Error
	if err != nil {
		return err
	}

	// Затем устанавливаем флаг is_default для указанного метода
	return r.db.Model(&entity.PaymentMethod{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_default", true).Error
}

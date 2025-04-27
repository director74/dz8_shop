package repo

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/director74/dz8_shop/order-service/internal/entity"
)

// OrderRepository интерфейс репозитория для работы с заказами
type OrderRepository interface {
	Create(ctx context.Context, order *entity.Order) error
	GetByID(ctx context.Context, id uint) (*entity.Order, error)
	GetByUserID(ctx context.Context, userID uint, limit, offset int) ([]entity.Order, error)
	CountByUserID(ctx context.Context, userID uint) (int64, error)
	Update(ctx context.Context, order *entity.Order) error
	Delete(ctx context.Context, id uint) error
	ListOrdersByUserID(ctx context.Context, userID uint, limit, offset int) ([]entity.Order, int64, error)
	UpdateOrderStatus(ctx context.Context, orderID uint, status entity.OrderStatus) error
}

// ErrOrderNotFound ошибка, когда заказ не найден
var ErrOrderNotFound = errors.New("заказ не найден")

// OrderRepositoryImpl реализация репозитория заказов на GORM
type OrderRepositoryImpl struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &OrderRepositoryImpl{
		db: db,
	}
}

func (r *OrderRepositoryImpl) Create(ctx context.Context, order *entity.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *OrderRepositoryImpl) GetByID(ctx context.Context, id uint) (*entity.Order, error) {
	var order entity.Order
	result := r.db.WithContext(ctx).First(&order, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, result.Error
	}
	return &order, nil
}

func (r *OrderRepositoryImpl) GetByUserID(ctx context.Context, userID uint, limit, offset int) ([]entity.Order, error) {
	var orders []entity.Order
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&orders)

	if result.Error != nil {
		return nil, result.Error
	}
	return orders, nil
}

// CountByUserID подсчитывает количество заказов пользователя
func (r *OrderRepositoryImpl) CountByUserID(ctx context.Context, userID uint) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Model(&entity.Order{}).
		Where("user_id = ?", userID).
		Count(&count)

	if result.Error != nil {
		return 0, result.Error
	}
	return count, nil
}

// Update обновляет заказ
func (r *OrderRepositoryImpl) Update(ctx context.Context, order *entity.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

// Delete удаляет заказ
func (r *OrderRepositoryImpl) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&entity.Order{}, id).Error
}

func (r *OrderRepositoryImpl) ListOrdersByUserID(ctx context.Context, userID uint, limit, offset int) ([]entity.Order, int64, error) {
	orders, err := r.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	total, err := r.CountByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	return orders, total, nil
}

// UpdateOrderStatus обновляет только статус заказа
func (r *OrderRepositoryImpl) UpdateOrderStatus(ctx context.Context, orderID uint, status entity.OrderStatus) error {
	result := r.db.WithContext(ctx).Model(&entity.Order{}).Where("id = ?", orderID).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOrderNotFound // Или другая подходящая ошибка, если 0 строк обновлено
	}
	return nil
}

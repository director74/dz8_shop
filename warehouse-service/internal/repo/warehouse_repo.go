package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/director74/dz8_shop/warehouse-service/internal/entity"
	"gorm.io/gorm"
)

// WarehouseRepo репозиторий для работы со складом
type WarehouseRepo struct {
	db *gorm.DB
}

// NewWarehouseRepo создает новый репозиторий склада
func NewWarehouseRepo(db *gorm.DB) *WarehouseRepo {
	return &WarehouseRepo{
		db: db,
	}
}

// GetWarehouseItemByID получает товар по ID
func (r *WarehouseRepo) GetWarehouseItemByID(id uint) (*entity.WarehouseItem, error) {
	var item entity.WarehouseItem
	result := r.db.First(&item, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &item, nil
}

// GetWarehouseItemByProductID получает товар по ID продукта
func (r *WarehouseRepo) GetWarehouseItemByProductID(productID uint) (*entity.WarehouseItem, error) {
	var item entity.WarehouseItem
	result := r.db.Where("product_id = ?", productID).First(&item)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &item, nil
}

// GetAllWarehouseItems получает список всех товаров
func (r *WarehouseRepo) GetAllWarehouseItems(limit, offset int) ([]entity.WarehouseItem, int64, error) {
	var items []entity.WarehouseItem
	var total int64

	r.db.Model(&entity.WarehouseItem{}).Count(&total)
	result := r.db.Limit(limit).Offset(offset).Find(&items)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	return items, total, nil
}

// CreateWarehouseItem создает новый товар
func (r *WarehouseRepo) CreateWarehouseItem(item *entity.WarehouseItem) error {
	return r.db.Create(item).Error
}

// UpdateWarehouseItem обновляет товар
func (r *WarehouseRepo) UpdateWarehouseItem(item *entity.WarehouseItem) error {
	return r.db.Model(item).Omit("available").Save(item).Error
}

// DeleteWarehouseItem удаляет товар
func (r *WarehouseRepo) DeleteWarehouseItem(id uint) error {
	return r.db.Delete(&entity.WarehouseItem{}, id).Error
}

// ReserveWarehouseItem резервирует товар для заказа
func (r *WarehouseRepo) ReserveWarehouseItem(ctx context.Context, orderID, productID uint, quantity int, expiresIn *time.Duration) (*entity.WarehouseReservation, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем товар для обновления с блокировкой строки
	var item entity.WarehouseItem
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("product_id = ?", productID).First(&item).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("товар с ID продукта %d не найден", productID)
		}
		return nil, err
	}

	// Проверяем, достаточно ли товара
	if item.Available < int64(quantity) {
		tx.Rollback()
		return nil, fmt.Errorf("недостаточно товара для резервации: запрошено %d, доступно %d", quantity, item.Available)
	}

	// Обновляем количество зарезервированного товара
	item.ReservedQuantity += int64(quantity)
	item.UpdatedAt = time.Now()

	// Обновляем товар, исключая поле available
	if err := tx.Model(&item).Omit("available").Updates(item).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Создаем запись о резервации
	reservation := &entity.WarehouseReservation{
		OrderID:         orderID,
		WarehouseItemID: item.ID,
		ProductID:       productID,
		Quantity:        quantity,
		ReservedAt:      time.Now(),
		Status:          "active",
	}

	// Устанавливаем время истечения резервации, если оно указано
	if expiresIn != nil {
		reservation.ReservationExpiry = time.Now().Add(*expiresIn)
	}

	if err := tx.Create(reservation).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	return reservation, tx.Commit().Error
}

// ReleaseWarehouseItems освобождает резервацию товара
func (r *WarehouseRepo) ReleaseWarehouseItems(ctx context.Context, orderID uint) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем все резервации для заказа
	var reservations []entity.WarehouseReservation
	if err := tx.Where("order_id = ? AND status = ?", orderID, "active").Find(&reservations).Error; err != nil {
		tx.Rollback()
		return err
	}

	if len(reservations) == 0 {
		tx.Rollback()
		return fmt.Errorf("активных резерваций для заказа %d не найдено", orderID)
	}

	// Обрабатываем каждую резервацию
	for _, reservation := range reservations {
		// Получаем товар для обновления с блокировкой строки
		var item entity.WarehouseItem
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&item, reservation.WarehouseItemID).Error; err != nil {
			tx.Rollback()
			return err
		}

		// Обновляем количество зарезервированного товара
		item.ReservedQuantity -= int64(reservation.Quantity)
		item.UpdatedAt = time.Now()

		// Обновляем товар, исключая поле available
		if err := tx.Model(&item).Omit("available").Updates(item).Error; err != nil {
			tx.Rollback()
			return err
		}

		// Обновляем статус резервации
		reservation.Status = "cancelled"
		if err := tx.Save(&reservation).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// ConfirmWarehouseItems подтверждает резервацию товара (продажа)
func (r *WarehouseRepo) ConfirmWarehouseItems(ctx context.Context, orderID uint) error {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем все резервации для заказа
	var reservations []entity.WarehouseReservation
	if err := tx.Where("order_id = ? AND status = ?", orderID, "active").Find(&reservations).Error; err != nil {
		tx.Rollback()
		return err
	}

	if len(reservations) == 0 {
		tx.Rollback()
		return fmt.Errorf("активных резерваций для заказа %d не найдено", orderID)
	}

	// Обрабатываем каждую резервацию
	for _, reservation := range reservations {
		// Получаем товар для обновления с блокировкой строки
		var item entity.WarehouseItem
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&item, reservation.WarehouseItemID).Error; err != nil {
			tx.Rollback()
			return err
		}

		// Обновляем количество зарезервированного товара и общего количества
		item.ReservedQuantity -= int64(reservation.Quantity)
		item.Quantity -= int64(reservation.Quantity)
		item.LastOrderID = &orderID
		item.UpdatedAt = time.Now()

		// Обновляем товар, исключая поле available
		if err := tx.Model(&item).Omit("available").Updates(item).Error; err != nil {
			tx.Rollback()
			return err
		}

		// Обновляем статус резервации
		reservation.Status = "completed"
		if err := tx.Save(&reservation).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit().Error
}

// GetReservationsByOrderID получает все резервации для заказа
func (r *WarehouseRepo) GetReservationsByOrderID(orderID uint) ([]entity.WarehouseReservation, error) {
	var reservations []entity.WarehouseReservation
	result := r.db.Where("order_id = ?", orderID).Find(&reservations)
	if result.Error != nil {
		return nil, result.Error
	}
	return reservations, nil
}

// CheckWarehouseAvailability проверяет наличие товара
func (r *WarehouseRepo) CheckWarehouseAvailability(items []entity.ReserveItem) (bool, []entity.UnavailableItem, error) {
	var unavailableItems []entity.UnavailableItem

	// Проверяем каждый товар по отдельности
	for _, item := range items {
		var warehouseItem entity.WarehouseItem
		result := r.db.Where("product_id = ?", item.ProductID).First(&warehouseItem)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				unavailableItems = append(unavailableItems, entity.UnavailableItem{
					ProductID:         item.ProductID,
					RequestedQuantity: int64(item.Quantity),
					AvailableQuantity: 0,
				})
				continue
			}
			return false, nil, result.Error
		}

		// Проверка наличия
		if warehouseItem.Available < int64(item.Quantity) {
			unavailableItems = append(unavailableItems, entity.UnavailableItem{
				ProductID:         item.ProductID,
				RequestedQuantity: int64(item.Quantity),
				AvailableQuantity: warehouseItem.Available,
			})
		}
	}

	// Если есть недоступные товары, возвращаем false
	return len(unavailableItems) == 0, unavailableItems, nil
}

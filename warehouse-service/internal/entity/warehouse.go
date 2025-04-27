package entity

import (
	"time"
)

// WarehouseStatus статус товара на складе
type WarehouseStatus string

// Константы для статусов товара на складе
const (
	WarehouseStatusAvailable   WarehouseStatus = "available"   // Доступен
	WarehouseStatusReserved    WarehouseStatus = "reserved"    // Зарезервирован
	WarehouseStatusSold        WarehouseStatus = "sold"        // Продан
	WarehouseStatusUnavailable WarehouseStatus = "unavailable" // Недоступен
)

// WarehouseItem представляет товар на складе
type WarehouseItem struct {
	ID               uint            `json:"id" gorm:"primaryKey"`
	ProductID        uint            `json:"product_id" gorm:"not null;index"`
	SKU              string          `json:"sku" gorm:"not null;uniqueIndex"`
	Name             string          `json:"name" gorm:"not null"`
	Description      string          `json:"description"`
	Quantity         int64           `json:"quantity" gorm:"type:bigint;not null;default:0"`
	Available        int64           `json:"available" gorm:"->;-:migration;-:update;column:available"`
	ReservedQuantity int64           `json:"reserved_quantity" gorm:"column:reserved_quantity;type:bigint;not null;default:0"`
	Price            float64         `json:"price" gorm:"not null"`
	Status           WarehouseStatus `json:"status" gorm:"not null;default:'available'"`
	Location         string          `json:"location" gorm:"not null;default:''"`
	LastOrderID      *uint           `json:"last_order_id" gorm:"index"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// ReservationStatus статус резервирования
type ReservationStatus string

// Константы для статусов резервирования
const (
	ReservationStatusPending   ReservationStatus = "pending"   // Ожидает подтверждения
	ReservationStatusConfirmed ReservationStatus = "confirmed" // Подтверждена
	ReservationStatusCompleted ReservationStatus = "completed" // Завершена (товары выданы)
	ReservationStatusFailed    ReservationStatus = "failed"    // Неудачна (ошибка склада)
	ReservationStatusCancelled ReservationStatus = "cancelled" // Отменена
	ReservationStatusExpired   ReservationStatus = "expired"   // Срок истек
	ReservationStatusActive    ReservationStatus = "active"    // Активна
)

// WarehouseReservation представляет резервирование товаров
type WarehouseReservation struct {
	ID                uint              `json:"id" gorm:"primaryKey"`
	OrderID           uint              `json:"order_id" gorm:"not null;index"`
	WarehouseItemID   uint              `json:"warehouse_item_id" gorm:"not null"`
	ProductID         uint              `json:"product_id" gorm:"not null"`
	Quantity          int               `json:"quantity" gorm:"not null"`
	Status            ReservationStatus `json:"status" gorm:"not null;default:'pending'"`
	ReservedAt        time.Time         `json:"reserved_at"`
	ReservationExpiry time.Time         `json:"reservation_expiry"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// WarehouseReservationItem представляет товар в резервировании
type WarehouseReservationItem struct {
	ID            uint    `json:"id" db:"id"`
	ReservationID uint    `json:"reservation_id" db:"reservation_id"`
	ProductID     uint    `json:"product_id" db:"product_id"`
	Quantity      uint    `json:"quantity" db:"quantity"`
	Price         float64 `json:"price" db:"price"`
}

// AvailabilityCheck представляет запрос на проверку наличия товаров
type AvailabilityCheck struct {
	Items []struct {
		ProductID uint `json:"product_id"`
		Quantity  uint `json:"quantity"`
	} `json:"items"`
}

// AvailabilityResult представляет результат проверки наличия товаров
type AvailabilityResult struct {
	Available bool   `json:"available"`
	Message   string `json:"message,omitempty"`
}

// PaginationParams параметры пагинации
type PaginationParams struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// ListResult результат запроса списка с пагинацией
type ListResult struct {
	Items      []WarehouseItem `json:"items"`
	TotalItems int             `json:"total_items"`
	TotalPages int             `json:"total_pages"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
}

// ReserveWarehouseRequest запрос на резервацию товара
type ReserveWarehouseRequest struct {
	OrderID   uint           `json:"order_id" binding:"required"`
	UserID    uint           `json:"user_id" binding:"required"`
	Items     []ReserveItem  `json:"items" binding:"required,dive"`
	ExpiresIn *time.Duration `json:"expires_in,omitempty"`
}

// ReserveItem элемент для резервации
type ReserveItem struct {
	ProductID uint `json:"product_id" binding:"required"`
	Quantity  int  `json:"quantity" binding:"required,gt=0"`
}

// ReleaseWarehouseRequest запрос на освобождение резервации
type ReleaseWarehouseRequest struct {
	OrderID uint `json:"order_id" binding:"required"`
	UserID  uint `json:"user_id" binding:"required"`
}

// ConfirmWarehouseRequest запрос на подтверждение резервации (продажа)
type ConfirmWarehouseRequest struct {
	OrderID uint `json:"order_id" binding:"required"`
	UserID  uint `json:"user_id" binding:"required"`
}

// WarehouseResponse ответ на операции с товарами на складе
type WarehouseResponse struct {
	Success       bool               `json:"success"`
	Message       string             `json:"message,omitempty"`
	OrderID       uint               `json:"order_id,omitempty"`
	ReservedItems []ReservedItemInfo `json:"reserved_items,omitempty"`
}

// ReservedItemInfo информация о зарезервированном товаре
type ReservedItemInfo struct {
	ProductID  uint `json:"product_id"`
	Quantity   int  `json:"quantity"`
	ReservedID uint `json:"reserved_id"`
}

// GetWarehouseResponse ответ на запрос информации о товаре
type GetWarehouseResponse struct {
	ID          uint            `json:"id"`
	ProductID   uint            `json:"product_id"`
	SKU         string          `json:"sku"`
	Quantity    int64           `json:"quantity"`
	Available   int64           `json:"available"`
	Status      WarehouseStatus `json:"status"`
	Location    string          `json:"location"`
	LastOrderID *uint           `json:"last_order_id,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ListWarehouseResponse ответ на запрос списка товаров
type ListWarehouseResponse struct {
	Items []GetWarehouseResponse `json:"items"`
	Total int64                  `json:"total"`
}

// CheckWarehouseRequest запрос на проверку наличия товара
type CheckWarehouseRequest struct {
	Items []ReserveItem `json:"items" binding:"required,dive"`
}

// CheckWarehouseResponse ответ на проверку наличия товара
type CheckWarehouseResponse struct {
	Available        bool              `json:"available"`
	UnavailableItems []UnavailableItem `json:"unavailable_items,omitempty"`
}

// UnavailableItem информация о недоступном товаре
type UnavailableItem struct {
	ProductID         uint  `json:"product_id"`
	RequestedQuantity int64 `json:"requested_quantity"`
	AvailableQuantity int64 `json:"available_quantity"`
}

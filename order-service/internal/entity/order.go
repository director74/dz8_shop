package entity

import (
	"time"
)

// OrderStatus статус заказа
type OrderStatus string

const (
	OrderStatusCreated   OrderStatus = "created"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusFailed    OrderStatus = "failed"
	OrderStatusCompleted OrderStatus = "completed"
)

// OrderItem элемент заказа
type OrderItem struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	OrderID   uint      `json:"order_id" gorm:"index"`
	ProductID uint      `json:"product_id"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Order хранит информацию о заказе клиента, его статусе и связанных товарах
type Order struct {
	ID               uint            `json:"id" gorm:"primaryKey"`
	UserID           uint            `json:"user_id" gorm:"index"`
	Items            []OrderItem     `json:"items" gorm:"foreignKey:OrderID"`
	Amount           float64         `json:"amount"`
	Status           OrderStatus     `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DeletedAt        *time.Time      `json:"-" gorm:"index"`
	User             User            `json:"-" gorm:"foreignKey:UserID"`
	CompensatedSteps map[string]bool `json:"-" gorm:"-"`
}

// CreateOrderRequest запрос на создание заказа
type CreateOrderRequest struct {
	UserID   uint             `json:"user_id"`
	Items    []OrderItem      `json:"items" binding:"required,min=1"`
	Amount   float64          `json:"amount" binding:"omitempty,min=0"`
	Delivery *DeliveryRequest `json:"delivery,omitempty"`
}

// DeliveryRequest информация о доставке в запросе
type DeliveryRequest struct {
	Address    string `json:"address"`
	TimeSlotID string `json:"time_slot_id"`
	ZoneID     string `json:"zone_id"`
}

// CreateOrderResponse ответ на запрос создания заказа
type CreateOrderResponse struct {
	ID        uint        `json:"id"`
	UserID    uint        `json:"user_id"`
	Items     []OrderItem `json:"items"`
	Amount    float64     `json:"amount"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}

type GetOrderResponse struct {
	ID        uint        `json:"id"`
	UserID    uint        `json:"user_id"`
	Amount    float64     `json:"amount"`
	Status    OrderStatus `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type ListOrdersResponse struct {
	Orders []GetOrderResponse `json:"orders"`
	Total  int64              `json:"total"`
}

type BillingRequest struct {
	UserID uint    `json:"user_id"`
	Amount float64 `json:"amount"`
}

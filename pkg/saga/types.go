package saga

import (
	"encoding/json"
	"time"
)

// SagaMessage представляет сообщение для оркестрации саги
type SagaMessage struct {
	SagaID    string          `json:"saga_id"`
	StepName  string          `json:"step_name"`
	Operation string          `json:"operation"` // "execute", "compensate", "result"
	Status    string          `json:"status"`    // "pending", "completed", "failed"
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// OrderItem представляет элемент заказа в саге
type OrderItem struct {
	ID        uint      `json:"id,omitempty"`
	OrderID   uint      `json:"order_id,omitempty"`
	ProductID uint      `json:"product_id"`
	Name      string    `json:"name,omitempty"`
	Price     float64   `json:"price"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// PaymentInfo информация о платеже
type PaymentInfo struct {
	PaymentID     string  `json:"payment_id,omitempty"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	TransactionID string  `json:"transaction_id,omitempty"`
}

// DeliveryInfo информация о доставке
type DeliveryInfo struct {
	DeliveryID   string  `json:"delivery_id,omitempty"`
	Address      string  `json:"address"`
	DeliveryDate string  `json:"delivery_date"`
	Cost         float64 `json:"cost"`
	Status       string  `json:"status"`
}

// WarehouseInfo информация о резервации товаров на складе
type WarehouseInfo struct {
	ReservationID string `json:"reservation_id,omitempty"`
	Status        string `json:"status"`
}

// BillingInfo информация о биллинге
type BillingInfo struct {
	TransactionID uint    `json:"transaction_id,omitempty"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
}

// SagaData представляет данные для передачи между шагами саги
type SagaData struct {
	OrderID       uint           `json:"order_id"`
	UserID        uint           `json:"user_id"`
	Items         []OrderItem    `json:"items"`
	Amount        float64        `json:"amount"`
	Status        string         `json:"status"`
	PaymentInfo   *PaymentInfo   `json:"payment_info,omitempty"`
	DeliveryInfo  *DeliveryInfo  `json:"delivery_info,omitempty"`
	WarehouseInfo *WarehouseInfo `json:"warehouse_info,omitempty"`
	BillingInfo   *BillingInfo   `json:"billing_info,omitempty"`
	Error         string         `json:"error,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

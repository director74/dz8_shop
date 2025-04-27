package entity

import (
	"time"
)

// DeliveryStatus статус доставки
type DeliveryStatus string

// Константы для статусов доставки
const (
	DeliveryStatusPending    DeliveryStatus = "pending"    // Ожидает подтверждения
	DeliveryStatusScheduled  DeliveryStatus = "scheduled"  // Запланирована
	DeliveryStatusConfirmed  DeliveryStatus = "confirmed"  // Подтверждено
	DeliveryStatusDelivering DeliveryStatus = "delivering" // В процессе доставки
	DeliveryStatusCompleted  DeliveryStatus = "completed"  // Доставлено
	DeliveryStatusCancelled  DeliveryStatus = "cancelled"  // Отменено
	DeliveryStatusFailed     DeliveryStatus = "failed"     // Не удалось доставить
	DeliveryStatusReturned   DeliveryStatus = "returned"   // Возвращено
)

// CourierStatus статус курьера
type CourierStatus string

// Константы для статусов курьера
const (
	CourierStatusAvailable   CourierStatus = "available"   // Доступен
	CourierStatusBusy        CourierStatus = "busy"        // Занят
	CourierStatusReserved    CourierStatus = "reserved"    // Зарезервирован
	CourierStatusUnavailable CourierStatus = "unavailable" // Недоступен
	CourierStatusOffline     CourierStatus = "offline"     // Не в сети
)

// Delivery представляет информацию о доставке
type Delivery struct {
	ID                 uint           `json:"id" gorm:"primaryKey"`
	OrderID            uint           `json:"order_id" gorm:"not null;index"`
	UserID             uint           `json:"user_id" gorm:"not null;index"`
	CourierID          *uint          `json:"courier_id" gorm:"index"`
	Status             DeliveryStatus `json:"status" gorm:"not null;default:'pending'"`
	ScheduledStartTime *time.Time     `json:"scheduled_start_time"`
	ScheduledEndTime   *time.Time     `json:"scheduled_end_time"`
	ActualStartTime    *time.Time     `json:"actual_start_time"`
	ActualEndTime      *time.Time     `json:"actual_end_time"`
	DeliveryAddress    string         `json:"delivery_address" gorm:"not null"`
	RecipientName      string         `json:"recipient_name" gorm:"not null"`
	RecipientPhone     string         `json:"recipient_phone" gorm:"not null"`
	Notes              string         `json:"notes"`
	TrackingCode       string         `json:"tracking_code"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// TableName указывает имя таблицы для Delivery
func (Delivery) TableName() string {
	return "delivery"
}

// Courier представляет информацию о курьере
type Courier struct {
	ID            uint          `json:"id" gorm:"primaryKey"`
	Name          string        `json:"name" gorm:"not null"`
	Phone         string        `json:"phone" gorm:"not null"`
	Email         string        `json:"email" gorm:"not null"`
	Status        CourierStatus `json:"status" gorm:"not null;default:'available'"`
	CurrentZoneID *uint         `json:"current_zone_id"`
	VehicleType   string        `json:"vehicle_type"`
	VehicleNumber string        `json:"vehicle_number"`
	Capacity      int           `json:"capacity"`
	Rating        float64       `json:"rating"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// CourierSchedule представляет расписание курьера
type CourierSchedule struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	CourierID   uint      `json:"courier_id" gorm:"not null;index"`
	SlotID      uint      `json:"slot_id" gorm:"not null;index"`
	OrderID     *uint     `json:"order_id" gorm:"index"`
	DeliveryID  *uint     `json:"delivery_id" gorm:"index"`
	StartTime   time.Time `json:"start_time" gorm:"not null"`
	EndTime     time.Time `json:"end_time" gorm:"not null"`
	IsReserved  bool      `json:"is_reserved" gorm:"not null;default:false"`
	IsCompleted bool      `json:"is_completed" gorm:"not null;default:false"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DeliveryTimeSlot представляет доступный временной слот для доставки
type DeliveryTimeSlot struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	StartTime  time.Time `json:"start_time" gorm:"not null"`
	EndTime    time.Time `json:"end_time" gorm:"not null"`
	ZoneID     uint      `json:"zone_id" gorm:"not null"`
	Capacity   int       `json:"capacity" gorm:"not null"`
	Available  int       `json:"available" gorm:"not null"`
	IsDisabled bool      `json:"is_disabled" gorm:"not null;default:false"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName указывает имя таблицы для DeliveryTimeSlot
func (DeliveryTimeSlot) TableName() string {
	return "delivery_slots"
}

// DeliveryZone представляет зону доставки
type DeliveryZone struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"not null"`
	Code      string    `json:"code" gorm:"not null;unique"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ReserveCourierRequest запрос на резервацию курьера
type ReserveCourierRequest struct {
	OrderID      uint      `json:"order_id" binding:"required"`
	UserID       uint      `json:"user_id" binding:"required"`
	DeliveryDate time.Time `json:"delivery_date" binding:"required"`
	TimeSlotID   uint      `json:"time_slot_id" binding:"required"`
	Address      string    `json:"address" binding:"required"`
	ZoneID       uint      `json:"zone_id" binding:"required"`
}

// ReleaseCourierRequest запрос на освобождение резервации курьера
type ReleaseCourierRequest struct {
	OrderID uint `json:"order_id" binding:"required"`
	UserID  uint `json:"user_id" binding:"required"`
}

// ConfirmCourierRequest запрос на подтверждение доставки
type ConfirmCourierRequest struct {
	OrderID uint `json:"order_id" binding:"required"`
	UserID  uint `json:"user_id" binding:"required"`
}

// DeliveryResponse ответ на операции с доставкой
type DeliveryResponse struct {
	Success         bool      `json:"success"`
	Message         string    `json:"message,omitempty"`
	OrderID         uint      `json:"order_id,omitempty"`
	DeliveryID      *uint     `json:"delivery_id,omitempty"`
	CourierID       *uint     `json:"courier_id,omitempty"`
	ScheduledStart  time.Time `json:"scheduled_start,omitempty"`
	ScheduledEnd    time.Time `json:"scheduled_end,omitempty"`
	Status          string    `json:"status,omitempty"`
	CourierSchedule *uint     `json:"courier_schedule,omitempty"`
}

// GetDeliveryResponse ответ на запрос информации о доставке
type GetDeliveryResponse struct {
	ID                 uint           `json:"id"`
	OrderID            uint           `json:"order_id"`
	UserID             uint           `json:"user_id"`
	CourierID          *uint          `json:"courier_id,omitempty"`
	Status             DeliveryStatus `json:"status"`
	ScheduledStartTime *time.Time     `json:"scheduled_start_time,omitempty"`
	ScheduledEndTime   *time.Time     `json:"scheduled_end_time,omitempty"`
	ActualStartTime    *time.Time     `json:"actual_start_time,omitempty"`
	ActualEndTime      *time.Time     `json:"actual_end_time,omitempty"`
	DeliveryAddress    string         `json:"delivery_address"`
	RecipientName      string         `json:"recipient_name"`
	RecipientPhone     string         `json:"recipient_phone"`
	TrackingCode       string         `json:"tracking_code,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// ListDeliveryResponse ответ на запрос списка доставок
type ListDeliveryResponse struct {
	Deliveries []GetDeliveryResponse `json:"deliveries"`
	Total      int64                 `json:"total"`
}

// GetTimeSlotResponse ответ на запрос временных слотов
type GetTimeSlotResponse struct {
	ID        uint      `json:"id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	ZoneID    uint      `json:"zone_id"`
	Available int       `json:"available"`
}

// ListTimeSlotResponse ответ на запрос списка временных слотов
type ListTimeSlotResponse struct {
	TimeSlots []GetTimeSlotResponse `json:"time_slots"`
	Total     int64                 `json:"total"`
}

// CheckAvailabilityRequest запрос на проверку доступности временных слотов
type CheckAvailabilityRequest struct {
	DeliveryDate time.Time `json:"delivery_date" binding:"required"`
	ZoneID       uint      `json:"zone_id" binding:"required"`
}

// CheckAvailabilityResponse ответ на проверку доступности временных слотов
type CheckAvailabilityResponse struct {
	Available bool                  `json:"available"`
	TimeSlots []GetTimeSlotResponse `json:"time_slots,omitempty"`
}

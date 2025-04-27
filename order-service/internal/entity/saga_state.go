package entity

import (
	"time"

	"gorm.io/datatypes"
)

// SagaStatus представляет возможные статусы саги
type SagaStatus string

const (
	SagaStatusRunning      SagaStatus = "running"
	SagaStatusCompensating SagaStatus = "compensating"
	SagaStatusCompleted    SagaStatus = "completed"
	SagaStatusFailed       SagaStatus = "failed"
	SagaStatusCompensated  SagaStatus = "compensated" // Завершена компенсация (по сути, failed)
)

// SagaState представляет состояние саги, хранящееся в БД
type SagaState struct {
	SagaID            string            `gorm:"primaryKey;type:varchar(255)"`
	OrderID           uint              `gorm:"not null;index"`
	Status            SagaStatus        `gorm:"not null;type:varchar(50);default:running;index"`
	CompensatedSteps  datatypes.JSONMap `gorm:"not null;default:'{}'"` // Используем datatypes.JSONMap для JSONB
	TotalToCompensate int               `gorm:"not null;default:0"`
	LastStep          string            `gorm:"type:varchar(100)"`
	ErrorMessage      string            `gorm:"type:text"`
	CreatedAt         time.Time         `gorm:"not null;default:now()"`
	UpdatedAt         time.Time         `gorm:"not null;default:now()"`

	// Связь с заказом (GORM автоматически не создает поле Order, если не нужно)
	// Order             Order             `gorm:"foreignKey:OrderID"` // Опционально, если нужна прямая загрузка заказа
}

// TableName задает имя таблицы для GORM
func (SagaState) TableName() string {
	return "saga_states"
}

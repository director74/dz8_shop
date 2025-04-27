package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/director74/dz8_shop/order-service/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// sagaStateRepository реализация SagaStateRepository с использованием GORM
type sagaStateRepository struct {
	db *gorm.DB
}

// NewSagaStateRepository создает новый экземпляр репозитория состояний саг
func NewSagaStateRepository(db *gorm.DB) *sagaStateRepository {
	return &sagaStateRepository{db: db}
}

// Create создает новую запись о состоянии саги
func (r *sagaStateRepository) Create(ctx context.Context, state *entity.SagaState) error {
	// Устанавливаем время создания и обновления
	now := time.Now()
	state.CreatedAt = now
	state.UpdatedAt = now
	// Убедимся, что CompensatedSteps не nil перед сохранением
	if state.CompensatedSteps == nil {
		state.CompensatedSteps = make(map[string]interface{})
	}

	result := r.db.WithContext(ctx).Create(state)
	if result.Error != nil {
		return fmt.Errorf("ошибка создания состояния саги %s: %w", state.SagaID, result.Error)
	}
	return nil
}

// GetByID получает состояние саги по ее ID
func (r *sagaStateRepository) GetByID(ctx context.Context, sagaID string) (*entity.SagaState, error) {
	var state entity.SagaState
	result := r.db.WithContext(ctx).First(&state, "saga_id = ?", sagaID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, result.Error // Возвращаем ошибку "не найдено"
		}
		return nil, fmt.Errorf("ошибка получения состояния саги %s: %w", sagaID, result.Error)
	}
	return &state, nil
}

// Update обновляет существующее состояние саги
func (r *sagaStateRepository) Update(ctx context.Context, state *entity.SagaState) error {
	// Обновляем время обновления
	state.UpdatedAt = time.Now()
	// Убедимся, что CompensatedSteps не nil перед сохранением
	if state.CompensatedSteps == nil {
		state.CompensatedSteps = make(map[string]interface{})
	}

	// Используем Clauses(clause.Returning{}) чтобы GORM вернул обновленную запись (если нужно)
	// Используем Omit(clause.Associations) чтобы не пытаться обновить связанные сущности (Order)
	result := r.db.WithContext(ctx).Omit(clause.Associations).Save(state)
	if result.Error != nil {
		return fmt.Errorf("ошибка обновления состояния саги %s: %w", state.SagaID, result.Error)
	}
	// Проверяем, была ли запись действительно обновлена (GORM может не вернуть ошибку, если запись не найдена при Save)
	if result.RowsAffected == 0 {
		// Можно вернуть gorm.ErrRecordNotFound или кастомную ошибку
		return gorm.ErrRecordNotFound // Указываем, что запись для обновления не найдена
	}
	return nil
}

// Delete удаляет состояние саги по ее ID
func (r *sagaStateRepository) Delete(ctx context.Context, sagaID string) error {
	// Создаем пустой экземпляр, чтобы указать GORM таблицу и ключ
	result := r.db.WithContext(ctx).Delete(&entity.SagaState{SagaID: sagaID})
	if result.Error != nil {
		return fmt.Errorf("ошибка удаления состояния саги %s: %w", sagaID, result.Error)
	}
	// GORM может не вернуть ошибку, если запись не найдена. Проверяем RowsAffected.
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound // Указываем, что запись для удаления не найдена
	}
	return nil
}

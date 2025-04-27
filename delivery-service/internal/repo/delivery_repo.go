package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/director74/dz8_shop/delivery-service/internal/entity"
	"gorm.io/gorm"
)

// DeliveryRepo репозиторий для работы с доставкой
type DeliveryRepo struct {
	db *gorm.DB
}

// NewDeliveryRepo создает новый репозиторий доставки
func NewDeliveryRepo(db *gorm.DB) *DeliveryRepo {
	return &DeliveryRepo{
		db: db,
	}
}

// GetDeliveryByID получает информацию о доставке по ID
func (r *DeliveryRepo) GetDeliveryByID(id uint) (*entity.Delivery, error) {
	var delivery entity.Delivery
	result := r.db.First(&delivery, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &delivery, nil
}

// GetDeliveryByOrderID получает информацию о доставке по ID заказа
func (r *DeliveryRepo) GetDeliveryByOrderID(orderID uint) (*entity.Delivery, error) {
	var delivery entity.Delivery
	result := r.db.Where("order_id = ?", orderID).First(&delivery)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &delivery, nil
}

// GetAllDeliveries получает список всех доставок с пагинацией
func (r *DeliveryRepo) GetAllDeliveries(limit, offset int) ([]entity.Delivery, int64, error) {
	var deliveries []entity.Delivery
	var total int64

	r.db.Model(&entity.Delivery{}).Count(&total)
	result := r.db.Limit(limit).Offset(offset).Find(&deliveries)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	return deliveries, total, nil
}

// CreateDelivery создает новую доставку
func (r *DeliveryRepo) CreateDelivery(delivery *entity.Delivery) error {
	result := r.db.Create(delivery)
	return result.Error
}

// UpdateDelivery обновляет информацию о доставке
func (r *DeliveryRepo) UpdateDelivery(delivery *entity.Delivery) error {
	result := r.db.Save(delivery)
	return result.Error
}

// DeleteDelivery удаляет доставку
func (r *DeliveryRepo) DeleteDelivery(id uint) error {
	result := r.db.Delete(&entity.Delivery{}, id)
	return result.Error
}

// GetCourierByID получает информацию о курьере по ID
func (r *DeliveryRepo) GetCourierByID(id uint) (*entity.Courier, error) {
	var courier entity.Courier
	result := r.db.First(&courier, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &courier, nil
}

// GetAvailableCouriers получает список доступных курьеров для зоны и времени
func (r *DeliveryRepo) GetAvailableCouriers(zoneID uint, startTime, endTime time.Time) ([]entity.Courier, error) {
	var couriers []entity.Courier

	// Ищем курьеров, которые привязаны к указанной зоне и имеют статус "доступен"
	result := r.db.Where("current_zone_id = ? AND status = ?", zoneID, entity.CourierStatusAvailable).
		Find(&couriers)
	if result.Error != nil {
		return nil, result.Error
	}

	// Фильтруем курьеров, проверяя их расписание
	var availableCouriers []entity.Courier
	for _, courier := range couriers {
		// Проверяем, нет ли пересечений в расписании курьера на указанное время
		var count int64
		r.db.Model(&entity.CourierSchedule{}).
			Where("courier_id = ? AND ((start_time <= ? AND end_time >= ?) OR (start_time <= ? AND end_time >= ?) OR (start_time >= ? AND end_time <= ?))",
				courier.ID, startTime, startTime, endTime, endTime, startTime, endTime).
			Where("is_reserved = ?", true).
			Count(&count)

		if count == 0 {
			availableCouriers = append(availableCouriers, courier)
		}
	}

	return availableCouriers, nil
}

// GetCourierSchedule получает расписание курьера
func (r *DeliveryRepo) GetCourierSchedule(courierID uint, startDate, endDate time.Time) ([]entity.CourierSchedule, error) {
	var schedules []entity.CourierSchedule
	result := r.db.Where("courier_id = ? AND start_time >= ? AND end_time <= ?", courierID, startDate, endDate).
		Find(&schedules)
	return schedules, result.Error
}

// GetTimeSlotByID получает информацию о временном слоте по ID
func (r *DeliveryRepo) GetTimeSlotByID(id uint) (*entity.DeliveryTimeSlot, error) {
	var slot entity.DeliveryTimeSlot
	result := r.db.First(&slot, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, result.Error
	}
	return &slot, nil
}

// GetAvailableTimeSlots получает доступные временные слоты для зоны на указанную дату
func (r *DeliveryRepo) GetAvailableTimeSlots(zoneID uint, date time.Time) ([]entity.DeliveryTimeSlot, error) {
	var slots []entity.DeliveryTimeSlot

	// Начало и конец указанного дня
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	// Ищем все слоты для указанной зоны на указанную дату, которые еще имеют доступные места
	result := r.db.Where("zone_id = ? AND start_time >= ? AND end_time <= ? AND available > 0 AND is_disabled = ?",
		zoneID, startOfDay, endOfDay, false).
		Find(&slots)

	return slots, result.Error
}

// ReserveCourier резервирует курьера для доставки
func (r *DeliveryRepo) ReserveCourier(ctx context.Context, orderID, userID, timeSlotID uint, address string, zoneID uint) (*entity.DeliveryResponse, error) {
	// Начинаем транзакцию
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем информацию о временном слоте
	var timeSlot entity.DeliveryTimeSlot
	if err := tx.First(&timeSlot, timeSlotID).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("временной слот не найден: %w", err)
	}

	// Проверяем доступность слота
	if timeSlot.Available <= 0 || timeSlot.IsDisabled {
		tx.Rollback()
		return nil, fmt.Errorf("временной слот недоступен")
	}

	// Получаем доступного курьера
	var availableCouriers []entity.Courier
	if err := tx.Where("current_zone_id = ? AND status = ?", zoneID, entity.CourierStatusAvailable).
		Find(&availableCouriers).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("ошибка при поиске доступных курьеров: %w", err)
	}

	if len(availableCouriers) == 0 {
		tx.Rollback()
		return nil, fmt.Errorf("нет доступных курьеров в указанной зоне")
	}

	// Проверяем, есть ли у курьеров свободное расписание на указанное время
	var selectedCourier *entity.Courier
	var courierSchedule entity.CourierSchedule

	for _, courier := range availableCouriers {
		// Проверяем, нет ли пересечений в расписании курьера на указанное время
		var count int64
		tx.Model(&entity.CourierSchedule{}).
			Where("courier_id = ? AND ((start_time <= ? AND end_time >= ?) OR (start_time <= ? AND end_time >= ?) OR (start_time >= ? AND end_time <= ?))",
				courier.ID, timeSlot.StartTime, timeSlot.StartTime, timeSlot.EndTime, timeSlot.EndTime, timeSlot.StartTime, timeSlot.EndTime).
			Where("is_reserved = ?", true).
			Count(&count)

		if count == 0 {
			selectedCourier = &courier
			break
		}
	}

	if selectedCourier == nil {
		tx.Rollback()
		return nil, fmt.Errorf("нет доступных курьеров в указанное время")
	}

	// Создаем запись в расписании курьера
	courierSchedule = entity.CourierSchedule{
		CourierID:   selectedCourier.ID,
		SlotID:      timeSlotID,
		OrderID:     &orderID,
		StartTime:   timeSlot.StartTime,
		EndTime:     timeSlot.EndTime,
		IsReserved:  true,
		IsCompleted: false,
		Notes:       "Зарезервировано для заказа #" + fmt.Sprintf("%d", orderID),
	}

	if err := tx.Create(&courierSchedule).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("ошибка при создании расписания курьера: %w", err)
	}

	// Создаем запись о доставке
	delivery := entity.Delivery{
		OrderID:            orderID,
		UserID:             userID,
		CourierID:          &selectedCourier.ID,
		Status:             entity.DeliveryStatusScheduled,
		ScheduledStartTime: &timeSlot.StartTime,
		ScheduledEndTime:   &timeSlot.EndTime,
		DeliveryAddress:    address,
		RecipientName:      "", // Будет заполнено позже
		RecipientPhone:     "", // Будет заполнено позже
	}

	if err := tx.Create(&delivery).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("ошибка при создании записи о доставке: %w", err)
	}

	// Обновляем запись курьера
	selectedCourier.Status = entity.CourierStatusReserved
	if err := tx.Save(selectedCourier).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("ошибка при обновлении статуса курьера: %w", err)
	}

	// Обновляем доступность временного слота
	timeSlot.Available--
	if err := tx.Save(&timeSlot).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("ошибка при обновлении доступности временного слота: %w", err)
	}

	// Обновляем запись в расписании курьера с ID доставки
	courierSchedule.DeliveryID = &delivery.ID
	if err := tx.Save(&courierSchedule).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("ошибка при обновлении расписания курьера: %w", err)
	}

	// Подтверждаем транзакцию
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Формируем ответ
	response := &entity.DeliveryResponse{
		Success:         true,
		Message:         "Курьер успешно зарезервирован",
		OrderID:         orderID,
		DeliveryID:      &delivery.ID,
		CourierID:       &selectedCourier.ID,
		ScheduledStart:  timeSlot.StartTime,
		ScheduledEnd:    timeSlot.EndTime,
		Status:          string(delivery.Status),
		CourierSchedule: &courierSchedule.ID,
	}

	return response, nil
}

// ReleaseCourier освобождает резервацию курьера
func (r *DeliveryRepo) ReleaseCourier(ctx context.Context, orderID uint) error {
	// Начинаем транзакцию
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Получаем информацию о доставке
	var delivery entity.Delivery
	if err := tx.Where("order_id = ?", orderID).First(&delivery).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Если доставка не найдена, считаем что операция успешна
			tx.Rollback()
			return nil
		}
		tx.Rollback()
		return fmt.Errorf("ошибка при поиске доставки: %w", err)
	}

	// Проверяем статус доставки
	if delivery.Status != entity.DeliveryStatusScheduled && delivery.Status != entity.DeliveryStatusPending {
		tx.Rollback()
		return fmt.Errorf("невозможно отменить доставку в текущем статусе: %s", delivery.Status)
	}

	// Получаем информацию о расписании курьера
	var schedule entity.CourierSchedule
	if err := tx.Where("delivery_id = ? AND is_reserved = ?", delivery.ID, true).
		First(&schedule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Если расписание не найдено, обновляем только доставку
			delivery.Status = entity.DeliveryStatusCancelled
			if err := tx.Save(&delivery).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("ошибка при обновлении статуса доставки: %w", err)
			}
			return tx.Commit().Error
		}
		tx.Rollback()
		return fmt.Errorf("ошибка при поиске расписания курьера: %w", err)
	}

	// Обновляем статус курьера
	if delivery.CourierID != nil {
		var courier entity.Courier
		if err := tx.First(&courier, *delivery.CourierID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("ошибка при поиске курьера: %w", err)
		}

		courier.Status = entity.CourierStatusAvailable
		if err := tx.Save(&courier).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("ошибка при обновлении статуса курьера: %w", err)
		}
	}

	// Обновляем доступность временного слота
	if delivery.ScheduledStartTime != nil && delivery.ScheduledEndTime != nil {
		var timeSlot entity.DeliveryTimeSlot
		if err := tx.Where("start_time = ? AND end_time = ?", delivery.ScheduledStartTime, delivery.ScheduledEndTime).
			First(&timeSlot).Error; err == nil {
			timeSlot.Available++
			if err := tx.Save(&timeSlot).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("ошибка при обновлении доступности временного слота: %w", err)
			}
		}
	}

	// Удаляем запись из расписания курьера
	if err := tx.Delete(&schedule).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка при удалении расписания курьера: %w", err)
	}

	// Обновляем статус доставки
	delivery.Status = entity.DeliveryStatusCancelled
	if err := tx.Save(&delivery).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("ошибка при обновлении статуса доставки: %w", err)
	}

	// Подтверждаем транзакцию
	return tx.Commit().Error
}

// ConfirmDelivery подтверждает доставку
func (r *DeliveryRepo) ConfirmDelivery(ctx context.Context, orderID uint) error {
	// Получаем информацию о доставке
	var delivery entity.Delivery
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).First(&delivery).Error; err != nil {
		return fmt.Errorf("ошибка при поиске доставки: %w", err)
	}

	// Обновляем статус доставки
	delivery.Status = entity.DeliveryStatusCompleted
	if err := r.db.WithContext(ctx).Save(&delivery).Error; err != nil {
		return fmt.Errorf("ошибка при обновлении статуса доставки: %w", err)
	}

	return nil
}

// CheckAvailability проверяет доступность временных слотов
func (r *DeliveryRepo) CheckAvailability(date time.Time, zoneID uint) (*entity.CheckAvailabilityResponse, error) {
	slots, err := r.GetAvailableTimeSlots(zoneID, date)
	if err != nil {
		return nil, err
	}

	response := &entity.CheckAvailabilityResponse{
		Available: len(slots) > 0,
	}

	if len(slots) > 0 {
		response.TimeSlots = make([]entity.GetTimeSlotResponse, 0, len(slots))
		for _, s := range slots {
			slotResponse := entity.GetTimeSlotResponse{
				ID:        s.ID,
				StartTime: s.StartTime,
				EndTime:   s.EndTime,
				ZoneID:    s.ZoneID,
				Available: s.Available,
			}
			response.TimeSlots = append(response.TimeSlots, slotResponse)
		}
	}

	return response, nil
}

package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/director74/dz8_shop/warehouse-service/internal/entity"
	"github.com/director74/dz8_shop/warehouse-service/internal/repo"
)

// WarehouseUseCase бизнес-логика для работы со складом
type WarehouseUseCase struct {
	repo *repo.WarehouseRepo
}

// NewWarehouseUseCase создает новый use case для склада
func NewWarehouseUseCase(repo *repo.WarehouseRepo) *WarehouseUseCase {
	return &WarehouseUseCase{
		repo: repo,
	}
}

// GetWarehouseItemByID получает информацию о товаре по ID
func (u *WarehouseUseCase) GetWarehouseItemByID(id uint) (*entity.GetWarehouseResponse, error) {
	item, err := u.repo.GetWarehouseItemByID(id)
	if err != nil {
		return nil, err
	}

	if item == nil {
		return nil, nil
	}

	return &entity.GetWarehouseResponse{
		ID:          item.ID,
		ProductID:   item.ProductID,
		SKU:         item.SKU,
		Quantity:    item.Quantity,
		Available:   item.Available,
		Status:      item.Status,
		Location:    item.Location,
		LastOrderID: item.LastOrderID,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

// GetWarehouseItemByProductID получает информацию о товаре по ID продукта
func (u *WarehouseUseCase) GetWarehouseItemByProductID(productID uint) (*entity.GetWarehouseResponse, error) {
	item, err := u.repo.GetWarehouseItemByProductID(productID)
	if err != nil {
		return nil, err
	}

	if item == nil {
		return nil, nil
	}

	return &entity.GetWarehouseResponse{
		ID:          item.ID,
		ProductID:   item.ProductID,
		SKU:         item.SKU,
		Quantity:    item.Quantity,
		Available:   item.Available,
		Status:      item.Status,
		Location:    item.Location,
		LastOrderID: item.LastOrderID,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

// GetAllWarehouseItems получает список всех товаров
func (u *WarehouseUseCase) GetAllWarehouseItems(limit, offset int) (*entity.ListWarehouseResponse, error) {
	if limit <= 0 {
		limit = 10
	}

	items, total, err := u.repo.GetAllWarehouseItems(limit, offset)
	if err != nil {
		return nil, err
	}

	var response entity.ListWarehouseResponse
	response.Total = total

	for _, item := range items {
		response.Items = append(response.Items, entity.GetWarehouseResponse{
			ID:          item.ID,
			ProductID:   item.ProductID,
			SKU:         item.SKU,
			Quantity:    item.Quantity,
			Available:   item.Available,
			Status:      item.Status,
			Location:    item.Location,
			LastOrderID: item.LastOrderID,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		})
	}

	return &response, nil
}

// CheckWarehouseAvailability проверяет наличие товаров
func (u *WarehouseUseCase) CheckWarehouseAvailability(req *entity.CheckWarehouseRequest) (*entity.CheckWarehouseResponse, error) {
	available, unavailableItems, err := u.repo.CheckWarehouseAvailability(req.Items)
	if err != nil {
		return nil, err
	}

	return &entity.CheckWarehouseResponse{
		Available:        available,
		UnavailableItems: unavailableItems,
	}, nil
}

// ReserveWarehouseItems резервирует товары для заказа
func (u *WarehouseUseCase) ReserveWarehouseItems(ctx context.Context, req *entity.ReserveWarehouseRequest) (*entity.WarehouseResponse, error) {
	response := &entity.WarehouseResponse{
		OrderID: req.OrderID,
	}

	// Проверяем доступность товаров перед резервацией
	checkReq := &entity.CheckWarehouseRequest{
		Items: req.Items,
	}
	availability, err := u.CheckWarehouseAvailability(checkReq)
	if err != nil {
		return nil, err
	}

	if !availability.Available {
		response.Success = false
		response.Message = "Некоторые товары недоступны для резервации"
		return response, errors.New("недостаточно товаров для резервации")
	}

	// Резервируем каждый товар по отдельности
	var reservedItems []entity.ReservedItemInfo
	for _, item := range req.Items {
		reservation, err := u.repo.ReserveWarehouseItem(ctx, req.OrderID, item.ProductID, item.Quantity, req.ExpiresIn)
		if err != nil {
			// Если произошла ошибка, освобождаем уже зарезервированные товары
			_ = u.ReleaseWarehouseItems(ctx, &entity.ReleaseWarehouseRequest{
				OrderID: req.OrderID,
				UserID:  req.UserID,
			})
			return nil, err
		}

		reservedItems = append(reservedItems, entity.ReservedItemInfo{
			ProductID:  item.ProductID,
			Quantity:   item.Quantity,
			ReservedID: reservation.ID,
		})
	}

	response.Success = true
	response.Message = "Товары успешно зарезервированы"
	response.ReservedItems = reservedItems

	return response, nil
}

// ReleaseWarehouseItems освобождает резервацию товаров
func (u *WarehouseUseCase) ReleaseWarehouseItems(ctx context.Context, req *entity.ReleaseWarehouseRequest) error {
	return u.repo.ReleaseWarehouseItems(ctx, req.OrderID)
}

// ConfirmWarehouseItems подтверждает резервацию товаров (продажа)
func (u *WarehouseUseCase) ConfirmWarehouseItems(ctx context.Context, req *entity.ConfirmWarehouseRequest) error {
	return u.repo.ConfirmWarehouseItems(ctx, req.OrderID)
}

// GetReservationsByOrderID получает все резервации для заказа
func (u *WarehouseUseCase) GetReservationsByOrderID(orderID uint) ([]entity.WarehouseReservation, error) {
	return u.repo.GetReservationsByOrderID(orderID)
}

// Методы для интеграции с системой саг

// ReserveForSaga резервирует товары для заказа в контексте саги
func (u *WarehouseUseCase) ReserveForSaga(ctx context.Context, data interface{}) error {
	reqData, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("неверный формат данных для резервации")
	}

	// Извлекаем данные из контекста саги
	orderID, ok := reqData["order_id"].(uint)
	if !ok {
		return fmt.Errorf("неверный формат ID заказа")
	}

	userID, ok := reqData["user_id"].(uint)
	if !ok {
		return fmt.Errorf("неверный формат ID пользователя")
	}

	itemsData, ok := reqData["items"].([]interface{})
	if !ok {
		return fmt.Errorf("неверный формат списка товаров")
	}

	// Преобразуем данные товаров в структуру ReserveItem
	var items []entity.ReserveItem
	for _, itemData := range itemsData {
		itemMap, ok := itemData.(map[string]interface{})
		if !ok {
			return fmt.Errorf("неверный формат данных товара")
		}

		productID, ok := itemMap["product_id"].(uint)
		if !ok {
			return fmt.Errorf("неверный формат ID продукта")
		}

		quantity, ok := itemMap["quantity"].(int)
		if !ok {
			return fmt.Errorf("неверный формат количества товара")
		}

		items = append(items, entity.ReserveItem{
			ProductID: productID,
			Quantity:  quantity,
		})
	}

	// Создаем запрос на резервацию
	var expiry time.Duration = 30 * time.Minute // Резервация на 30 минут
	req := &entity.ReserveWarehouseRequest{
		OrderID:   orderID,
		UserID:    userID,
		Items:     items,
		ExpiresIn: &expiry,
	}

	// Выполняем резервацию
	response, err := u.ReserveWarehouseItems(ctx, req)
	if err != nil {
		return err
	}

	if !response.Success {
		return fmt.Errorf("не удалось зарезервировать товары: %s", response.Message)
	}

	// Добавляем информацию о резервации в данные саги
	reqData["reservation_info"] = response
	return nil
}

// ReleaseForSaga освобождает резервацию товаров в контексте саги (компенсирующая операция)
func (u *WarehouseUseCase) ReleaseForSaga(ctx context.Context, data interface{}) error {
	reqData, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("неверный формат данных для освобождения резервации")
	}

	// Извлекаем данные из контекста саги
	orderID, ok := reqData["order_id"].(uint)
	if !ok {
		return fmt.Errorf("неверный формат ID заказа")
	}

	userID, ok := reqData["user_id"].(uint)
	if !ok {
		return fmt.Errorf("неверный формат ID пользователя")
	}

	// Создаем запрос на освобождение резервации
	req := &entity.ReleaseWarehouseRequest{
		OrderID: orderID,
		UserID:  userID,
	}

	// Выполняем освобождение резервации
	return u.ReleaseWarehouseItems(ctx, req)
}

// ConfirmForSaga подтверждает резервацию товаров в контексте саги
func (u *WarehouseUseCase) ConfirmForSaga(ctx context.Context, data interface{}) error {
	reqData, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("неверный формат данных для подтверждения резервации")
	}

	// Извлекаем данные из контекста саги
	orderID, ok := reqData["order_id"].(uint)
	if !ok {
		return fmt.Errorf("неверный формат ID заказа")
	}

	userID, ok := reqData["user_id"].(uint)
	if !ok {
		return fmt.Errorf("неверный формат ID пользователя")
	}

	// Создаем запрос на подтверждение резервации
	req := &entity.ConfirmWarehouseRequest{
		OrderID: orderID,
		UserID:  userID,
	}

	// Выполняем подтверждение резервации
	return u.ConfirmWarehouseItems(ctx, req)
}

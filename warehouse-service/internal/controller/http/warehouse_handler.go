package http

import (
	"net/http"
	"strconv"

	pkgMiddleware "github.com/director74/dz8_shop/pkg/middleware"
	"github.com/director74/dz8_shop/warehouse-service/config"
	"github.com/director74/dz8_shop/warehouse-service/internal/entity"
	"github.com/director74/dz8_shop/warehouse-service/internal/usecase"
	"github.com/gin-gonic/gin"
)

// WarehouseHandler обработчик HTTP запросов для склада
type WarehouseHandler struct {
	warehouseUseCase *usecase.WarehouseUseCase
	config           *config.Config
}

// NewWarehouseHandler создает новый обработчик склада
func NewWarehouseHandler(warehouseUseCase *usecase.WarehouseUseCase, cfg *config.Config) *WarehouseHandler {
	return &WarehouseHandler{
		warehouseUseCase: warehouseUseCase,
		config:           cfg,
	}
}

// HealthCheck обрабатывает запрос на проверку работоспособности сервиса
func (h *WarehouseHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// getUserID получает ID пользователя из контекста
func getUserID(c *gin.Context) uint {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}

	id, ok := userID.(uint)
	if !ok {
		return 0
	}

	return id
}

// GetWarehouseItem возвращает информацию о товаре по ID
func (h *WarehouseHandler) GetWarehouseItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID товара"})
		return
	}

	warehouse, err := h.warehouseUseCase.GetWarehouseItemByID(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if warehouse == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "товар не найден"})
		return
	}

	c.JSON(http.StatusOK, warehouse)
}

// GetWarehouseItemByProduct возвращает информацию о товаре по ID продукта
func (h *WarehouseHandler) GetWarehouseItemByProduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID продукта"})
		return
	}

	warehouse, err := h.warehouseUseCase.GetWarehouseItemByProductID(uint(productID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if warehouse == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "товар не найден"})
		return
	}

	c.JSON(http.StatusOK, warehouse)
}

// GetAllWarehouseItems возвращает список всех товаров
func (h *WarehouseHandler) GetAllWarehouseItems(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	warehouse, err := h.warehouseUseCase.GetAllWarehouseItems(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, warehouse)
}

// CheckWarehouseAvailability проверяет наличие товаров
func (h *WarehouseHandler) CheckWarehouseAvailability(c *gin.Context) {
	var req entity.CheckWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.warehouseUseCase.CheckWarehouseAvailability(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ReserveWarehouseItems резервирует товары для заказа
func (h *WarehouseHandler) ReserveWarehouseItems(c *gin.Context) {
	var req entity.ReserveWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем права доступа
	userID := getUserID(c)
	if userID == 0 || userID != req.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещен"})
		return
	}

	response, err := h.warehouseUseCase.ReserveWarehouseItems(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !response.Success {
		c.JSON(http.StatusBadRequest, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ReleaseWarehouseItems освобождает резервацию товаров
func (h *WarehouseHandler) ReleaseWarehouseItems(c *gin.Context) {
	var req entity.ReleaseWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем права доступа
	userID := getUserID(c)
	if userID == 0 || userID != req.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещен"})
		return
	}

	err := h.warehouseUseCase.ReleaseWarehouseItems(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "резервация успешно отменена", "order_id": req.OrderID})
}

// ConfirmWarehouseItems подтверждает резервацию товаров (продажа)
func (h *WarehouseHandler) ConfirmWarehouseItems(c *gin.Context) {
	var req entity.ConfirmWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем права доступа
	userID := getUserID(c)
	if userID == 0 || userID != req.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещен"})
		return
	}

	err := h.warehouseUseCase.ConfirmWarehouseItems(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "резервация успешно подтверждена", "order_id": req.OrderID})
}

// GetOrderReservations возвращает все резервации для заказа
func (h *WarehouseHandler) GetOrderReservations(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("order_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID заказа"})
		return
	}

	reservations, err := h.warehouseUseCase.GetReservationsByOrderID(uint(orderID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reservations": reservations})
}

// InternalReserveWarehouseItems резервирует товары для заказа (для внутренних вызовов)
func (h *WarehouseHandler) InternalReserveWarehouseItems(c *gin.Context) {
	var req entity.ReserveWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.warehouseUseCase.ReserveWarehouseItems(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !response.Success {
		c.JSON(http.StatusBadRequest, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// InternalReleaseWarehouseItems освобождает резервацию товаров (для внутренних вызовов)
func (h *WarehouseHandler) InternalReleaseWarehouseItems(c *gin.Context) {
	var req entity.ReleaseWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.warehouseUseCase.ReleaseWarehouseItems(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "резервация успешно отменена", "order_id": req.OrderID})
}

// InternalConfirmWarehouseItems подтверждает резервацию товаров (для внутренних вызовов)
func (h *WarehouseHandler) InternalConfirmWarehouseItems(c *gin.Context) {
	var req entity.ConfirmWarehouseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.warehouseUseCase.ConfirmWarehouseItems(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "резервация успешно подтверждена", "order_id": req.OrderID})
}

// RegisterRoutes регистрирует маршруты для склада
func (h *WarehouseHandler) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	// Эндпоинт для проверки работоспособности сервиса
	router.GET("/health", h.HealthCheck)

	// Публичные API маршруты (с авторизацией)
	warehouse := router.Group("/api/v1/warehouse")
	{
		warehouse.GET("/:id", h.GetWarehouseItem)
		warehouse.GET("/product/:product_id", h.GetWarehouseItemByProduct)
		warehouse.GET("", h.GetAllWarehouseItems)
		warehouse.POST("/check", h.CheckWarehouseAvailability)
		warehouse.POST("/reserve", authMiddleware, h.ReserveWarehouseItems)
		warehouse.POST("/release", authMiddleware, h.ReleaseWarehouseItems)
		warehouse.POST("/confirm", authMiddleware, h.ConfirmWarehouseItems)
		warehouse.GET("/order/:order_id", authMiddleware, h.GetOrderReservations)
	}

	// Внутренние API маршруты (с проверкой доступа для внутренних сервисов)
	internalAPIConfig := &pkgMiddleware.InternalAPIConfig{
		TrustedNetworks: h.config.Internal.TrustedNetworks,
		APIKeyEnvName:   h.config.Internal.APIKeyEnvName,
		DefaultAPIKey:   h.config.Internal.DefaultAPIKey,
		HeaderName:      h.config.Internal.HeaderName,
	}

	internalAuthMiddleware := pkgMiddleware.NewInternalAuthMiddleware(internalAPIConfig)
	internal := router.Group("/internal", internalAuthMiddleware.Required())
	{
		internalWarehouse := internal.Group("/warehouse")
		{
			internalWarehouse.GET("/:id", h.GetWarehouseItem)
			internalWarehouse.GET("/product/:product_id", h.GetWarehouseItemByProduct)
			internalWarehouse.POST("/check", h.CheckWarehouseAvailability)
			internalWarehouse.POST("/reserve", h.InternalReserveWarehouseItems)
			internalWarehouse.POST("/release", h.InternalReleaseWarehouseItems)
			internalWarehouse.POST("/confirm", h.InternalConfirmWarehouseItems)
			internalWarehouse.GET("/order/:order_id", h.GetOrderReservations)
		}
	}
}

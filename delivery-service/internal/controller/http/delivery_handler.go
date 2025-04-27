package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/director74/dz8_shop/delivery-service/internal/entity"
	"github.com/director74/dz8_shop/delivery-service/internal/usecase"
	"github.com/gin-gonic/gin"
)

// DeliveryHandler обработчик HTTP запросов для доставки
type DeliveryHandler struct {
	deliveryUseCase *usecase.DeliveryUseCase
}

// NewDeliveryHandler создает новый обработчик HTTP запросов для доставки
func NewDeliveryHandler(deliveryUseCase *usecase.DeliveryUseCase) *DeliveryHandler {
	return &DeliveryHandler{
		deliveryUseCase: deliveryUseCase,
	}
}

// RegisterRoutes регистрирует маршруты для доставки
func (h *DeliveryHandler) RegisterRoutes(router *gin.Engine) {
	// Добавляем эндпоинт для проверки работоспособности сервиса
	router.GET("/health", h.HealthCheck)

	deliveryGroup := router.Group("/api/v1/delivery")
	{
		deliveryGroup.GET("/:id", h.GetDeliveryByID)
		deliveryGroup.GET("/order/:order_id", h.GetDeliveryByOrderID)
		deliveryGroup.GET("/list", h.GetAllDeliveries)
		deliveryGroup.POST("/check-availability", h.CheckAvailability)
		deliveryGroup.POST("/reserve", h.ReserveCourier)
		deliveryGroup.POST("/release", h.ReleaseCourier)
		deliveryGroup.POST("/confirm", h.ConfirmDelivery)
	}
}

// HealthCheck обрабатывает запрос на проверку работоспособности сервиса
func (h *DeliveryHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetDeliveryByID обрабатывает запрос на получение информации о доставке по ID
func (h *DeliveryHandler) GetDeliveryByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат ID"})
		return
	}

	delivery, err := h.deliveryUseCase.GetDeliveryByID(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if delivery == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "доставка не найдена"})
		return
	}

	c.JSON(http.StatusOK, delivery)
}

// GetDeliveryByOrderID обрабатывает запрос на получение информации о доставке по ID заказа
func (h *DeliveryHandler) GetDeliveryByOrderID(c *gin.Context) {
	orderIDStr := c.Param("order_id")
	orderID, err := strconv.ParseUint(orderIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат ID заказа"})
		return
	}

	delivery, err := h.deliveryUseCase.GetDeliveryByOrderID(uint(orderID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if delivery == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "доставка не найдена"})
		return
	}

	c.JSON(http.StatusOK, delivery)
}

// GetAllDeliveries обрабатывает запрос на получение списка всех доставок с пагинацией
func (h *DeliveryHandler) GetAllDeliveries(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат limit"})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный формат offset"})
		return
	}

	deliveries, err := h.deliveryUseCase.GetAllDeliveries(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, deliveries)
}

// CheckAvailability обрабатывает запрос на проверку доступности временных слотов
func (h *DeliveryHandler) CheckAvailability(c *gin.Context) {
	var req entity.CheckAvailabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.deliveryUseCase.CheckAvailability(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ReserveCourier обрабатывает запрос на резервацию курьера
func (h *DeliveryHandler) ReserveCourier(c *gin.Context) {
	var req entity.ReserveCourierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.deliveryUseCase.ReserveCourier(context.Background(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ReleaseCourier обрабатывает запрос на освобождение резервации курьера
func (h *DeliveryHandler) ReleaseCourier(c *gin.Context) {
	var req entity.ReleaseCourierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.deliveryUseCase.ReleaseCourier(context.Background(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Резервация успешно отменена"})
}

// ConfirmDelivery обрабатывает запрос на подтверждение доставки
func (h *DeliveryHandler) ConfirmDelivery(c *gin.Context) {
	var req entity.ConfirmCourierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.deliveryUseCase.ConfirmDelivery(context.Background(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Доставка успешно подтверждена"})
}
